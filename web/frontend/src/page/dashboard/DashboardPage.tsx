import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import Sidebar from '../../components/sidebar/Sidebar'
import StatsCard from '../../components/stats/stats-card'
import NotificationContainer from '../../components/notifications/NotificationContainer'
import { useNotifications } from '../../hooks/useNotifications'
import { extractErrorMessage } from '../../apiClient'
import TopologyCanvas from './TopologyCanvas'
import DetailPanel from './DetailPanel'
import UePanel from './UePanel'
import UeTerminalModal from './UeTerminalModal'
import { useFree5gcStatus } from './useFree5gcStatus'
import { useGnbStatus } from './useGnbStatus'
import { useUeInstances, type UeRow } from './useUeInstances'
import { getStatusMeta } from './statusMeta'
import type { DeploymentNode, NodeId, NodeStatus } from './types'
import styles from './dashboard-page.module.css'

// UE's count is driven by subscribers, not by who happens to be deployed
// right now - a subscriber that's never been deployed still counts toward
// the total, just as "stopped".
function computeUeNode(rows: UeRow[]): DeploymentNode {
  const total = rows.length
  const runningCount = rows.filter((row) => row.status === 'running').length
  const hasUnhealthy = rows.some((row) => row.status === 'unhealthy')
  const hasDeploying = rows.some((row) => row.status === 'deploying')

  const status: NodeStatus = hasUnhealthy
    ? 'unhealthy'
    : hasDeploying
      ? 'deploying'
      : runningCount > 0
        ? 'running'
        : 'stopped'

  const mostRecentDeploy = rows
    .map((row) => row.lastDeployed)
    .find((lastDeployed) => lastDeployed !== '—')

  return {
    id: 'ue',
    label: 'UE',
    sublabel: total === 0 ? 'No subscribers yet' : `${runningCount} / ${total} UE instance${total === 1 ? '' : 's'} running`,
    status,
    statusLabel: `${runningCount} / ${total}`,
    template: 'basic (built-in template)',
    lastDeployed: mostRecentDeploy ?? '—',
  }
}

export default function DashboardPage() {
  const navigate = useNavigate()
  const [selected, setSelected] = useState<NodeId>('core')
  const [ueTerminalInstance, setUeTerminalInstance] = useState<string | null>(null)

  const { errors, successes, addError, addSuccess, removeNotification } = useNotifications()
  const free5gc = useFree5gcStatus()
  const gnb = useGnbStatus()
  const ue = useUeInstances()

  // core and gnb are single-instance targets with a uniform deploy/stop/logs
  // shape; ue is multi-instance (one per subscriber) and gets its own panel.
  const activeTarget = selected === 'core' ? free5gc : selected === 'gnb' ? gnb : null

  async function handleDeployUe(ueId: string) {
    try {
      await ue.deploy(ueId)
      addSuccess(`Deploying UE for ${ueId}`)
    } catch (error) {
      addError(extractErrorMessage(error, `Failed to deploy UE for ${ueId}`))
    }
  }

  async function handleStopUe(ueId: string) {
    try {
      await ue.stop(ueId)
      addSuccess(`UE ${ueId} stopped`)
    } catch (error) {
      addError(extractErrorMessage(error, `Failed to stop UE ${ueId}`))
    }
  }

  async function handlePrimaryAction() {
    if (!activeTarget) return

    try {
      if (activeTarget.node.status === 'running') {
        await activeTarget.stop()
        addSuccess(`${activeTarget.node.label} stopped`)
      } else {
        await activeTarget.deploy()
        addSuccess(`${activeTarget.node.label} deploy started`)
      }
    } catch (error) {
      addError(extractErrorMessage(error, `${activeTarget.node.label} action failed`))
    }
  }

  function handleViewLogs() {
    navigate(`/logs?target=${selected}`)
  }

  const nodes: Record<NodeId, DeploymentNode> = {
    core: free5gc.node,
    gnb: gnb.node,
    ue: computeUeNode(ue.rows),
  }
  const selectedNode = nodes[selected]

  // mirrors the backend's own dependency-order enforcement (Processor.
  // checkStopDependents / checkDeployPrerequisite), just so the button is
  // disabled with a reason instead of only erroring after a click.
  const anyUeRunning = ue.rows.some((row) => row.status !== 'stopped')
  const actionBlockedReason = selected === 'core' && nodes.core.status === 'running' && nodes.gnb.status !== 'stopped'
    ? 'Stop gNB before stopping the core network'
    : selected === 'gnb' && nodes.gnb.status === 'running' && anyUeRunning
      ? 'Stop all UE instances before stopping gNB'
      : selected === 'gnb' && nodes.gnb.status !== 'running' && nodes.core.status !== 'running'
        ? 'Deploy the core network before deploying gNB'
        : undefined

  const healthyCount = free5gc.networkFunctions.filter((nf) => nf.status === 'running').length
  const totalNfs = free5gc.networkFunctions.length
  const unhealthyCount = totalNfs - healthyCount

  return (
    <div className={styles.layout}>
      <NotificationContainer
        errors={errors}
        successes={successes}
        onClose={removeNotification}
      />

      <Sidebar />

      <main className={styles.content}>
        <header>
          <h2 className={styles.title}>Lab Dashboard</h2>
          <p className={styles.subtitle}>
            Deploy and monitor the free5GC core network and the free-ran-ue gNB / UE simulator
          </p>
        </header>

        <section className={styles.statsGrid}>
          <StatsCard
            title="Core Network"
            value={getStatusLabel(nodes.core.status)}
            description="free5GC · basic template"
            valueColor={getStatusMeta(nodes.core.status).color}
          />
          <StatsCard
            title="gNB"
            value={getStatusLabel(nodes.gnb.status)}
            description="free-ran-ue · basic template"
            valueColor={getStatusMeta(nodes.gnb.status).color}
          />
          <StatsCard
            title="UE"
            value={nodes.ue.statusLabel ?? getStatusLabel(nodes.ue.status)}
            description={ue.rows.length > 0 ? 'instances running / subscribers' : 'free-ran-ue · basic template'}
            valueColor={getStatusMeta(nodes.ue.status).color}
          />
          <StatsCard
            title="Healthy NFs"
            value={totalNfs > 0 ? `${healthyCount} / ${totalNfs}` : '—'}
            description={unhealthyCount > 0
              ? `${unhealthyCount} NF${unhealthyCount === 1 ? '' : 's'} failing health check`
              : 'All network functions healthy'}
          />
        </section>

        <section className={styles.topologySection}>
          <div className={styles.topologyHeader}>
            <div>
              <h3>Network Topology</h3>
              <p>Click a node to see its status and actions. Drag nodes to rearrange the layout.</p>
            </div>
            <div className={styles.legend}>
              <span className={styles.legendItem}>
                <span className={`${styles.legendDot} ${styles.legendDotRunning}`} />Running
              </span>
              <span className={styles.legendItem}>
                <span className={`${styles.legendDot} ${styles.legendDotStopped}`} />Stopped
              </span>
              <span className={styles.legendItem}>
                <span className={`${styles.legendDot} ${styles.legendDotUnhealthy}`} />Unhealthy
              </span>
            </div>
          </div>

          <div className={styles.topologyBody}>
            <div className={styles.canvasWrap}>
              <TopologyCanvas nodes={nodes} selected={selected} onSelect={setSelected} />
            </div>

            {selected === 'ue' ? (
              <UePanel
                rows={ue.rows}
                isLoading={ue.isLoadingSubscribers}
                loadError={ue.subscribersError}
                pendingInstances={ue.pendingInstances}
                onDeploy={handleDeployUe}
                onStop={handleStopUe}
                onOpenTerminal={setUeTerminalInstance}
                deployBlockedReason={nodes.gnb.status !== 'running' ? 'Deploy gNB before deploying a UE instance' : undefined}
              />
            ) : (
              <DetailPanel
                node={selectedNode}
                networkFunctions={activeTarget ? activeTarget.networkFunctions : []}
                onViewLogs={handleViewLogs}
                onPrimaryAction={activeTarget ? handlePrimaryAction : undefined}
                isActionPending={activeTarget?.isActionPending ?? false}
                actionBlockedReason={actionBlockedReason}
              />
            )}
          </div>
        </section>
      </main>

      <UeTerminalModal
        instance={ueTerminalInstance}
        onClose={() => setUeTerminalInstance(null)}
      />
    </div>
  )
}

function getStatusLabel(status: string): string {
  return status.charAt(0).toUpperCase() + status.slice(1)
}
