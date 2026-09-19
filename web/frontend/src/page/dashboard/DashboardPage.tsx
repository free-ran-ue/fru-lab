import { useState } from 'react'
import Sidebar from '../../components/sidebar/Sidebar'
import StatsCard from '../../components/stats/stats-card'
import NotificationContainer from '../../components/notifications/NotificationContainer'
import { useNotifications } from '../../hooks/useNotifications'
import { extractErrorMessage } from '../../apiClient'
import TopologyCanvas from './TopologyCanvas'
import DetailPanel from './DetailPanel'
import UePanel from './UePanel'
import LogsModal from './LogsModal'
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
  const [selected, setSelected] = useState<NodeId>('core')
  const [showLogs, setShowLogs] = useState(false)
  const [ueLogsInstance, setUeLogsInstance] = useState<string | null>(null)
  const [ueLogLines, setUeLogLines] = useState<string[]>([])
  const [isLoadingUeLogs, setIsLoadingUeLogs] = useState(false)
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

  async function handleViewUeLogs(ueId: string) {
    setUeLogsInstance(ueId)
    setIsLoadingUeLogs(true)
    try {
      const lines = await ue.fetchLogs(ueId)
      setUeLogLines(lines)
    } catch (error) {
      addError(extractErrorMessage(error, 'Failed to load logs'))
    } finally {
      setIsLoadingUeLogs(false)
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

  async function handleViewLogs() {
    setShowLogs(true)
    if (!activeTarget) return

    try {
      await activeTarget.fetchLogs()
    } catch (error) {
      addError(extractErrorMessage(error, 'Failed to load logs'))
    }
  }

  const nodes: Record<NodeId, DeploymentNode> = {
    core: free5gc.node,
    gnb: gnb.node,
    ue: computeUeNode(ue.rows),
  }
  const selectedNode = nodes[selected]

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
            (built-in templates for now, custom parameters coming later)
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
                onViewLogs={handleViewUeLogs}
                onOpenTerminal={setUeTerminalInstance}
              />
            ) : (
              <DetailPanel
                node={selectedNode}
                networkFunctions={activeTarget ? activeTarget.networkFunctions : []}
                onViewLogs={handleViewLogs}
                onPrimaryAction={activeTarget ? handlePrimaryAction : undefined}
                isActionPending={activeTarget?.isActionPending ?? false}
              />
            )}
          </div>
        </section>
      </main>

      <LogsModal
        isOpen={showLogs}
        node={selectedNode}
        logLines={activeTarget ? activeTarget.logLines : []}
        isLoading={activeTarget?.isLoadingLogs ?? false}
        onClose={() => setShowLogs(false)}
      />

      <LogsModal
        isOpen={ueLogsInstance !== null}
        node={{
          id: 'ue',
          label: `UE — ${ueLogsInstance}`,
          sublabel: '',
          status: 'running',
          template: '',
          lastDeployed: '',
        }}
        logLines={ueLogLines}
        isLoading={isLoadingUeLogs}
        onClose={() => setUeLogsInstance(null)}
      />

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
