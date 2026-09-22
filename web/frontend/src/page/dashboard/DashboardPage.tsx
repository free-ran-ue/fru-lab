import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import Sidebar from '../../components/sidebar/Sidebar'
import StatsCard from '../../components/stats/stats-card'
import NotificationContainer from '../../components/notifications/NotificationContainer'
import { useNotifications } from '../../hooks/useNotifications'
import { extractErrorMessage } from '../../apiClient'
import TopologyCanvas from './TopologyCanvas'
import DetailedTopology from './DetailedTopology'
import DetailPanel from './DetailPanel'
import UePanel from './UePanel'
import UeTerminalModal from './UeTerminalModal'
import { useFree5gcStatus } from './useFree5gcStatus'
import { useGnbStatus } from './useGnbStatus'
import { useGnbSliceStatus } from './useGnbSliceStatus'
import { useUeInstances, type UeRow } from './useUeInstances'
import { getStatusMeta } from './statusMeta'
import { FREE5GC_TEMPLATE_OPTIONS, detectFree5gcTemplate, type Free5gcTemplate } from './free5gcTemplate'
import type { DeploymentNode, NetworkFunction, NodeId, NodeStatus } from './types'
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

// gNB stays one card/node in the topology regardless of core's template -
// under ulcl-2slice, "deploy"/"stop" on that one card just fans out to both
// slice targets at once (see handlePrimaryAction), and this aggregates
// their two statuses into the single status the card shows.
function computeGnbAggregateNode(slice1: DeploymentNode, slice2: DeploymentNode): DeploymentNode {
  const statuses = [slice1.status, slice2.status]
  const status: NodeStatus = statuses.includes('unhealthy')
    ? 'unhealthy'
    : statuses.includes('deploying')
      ? 'deploying'
      : statuses.some((s) => s === 'running')
        ? 'running'
        : 'stopped'

  return {
    id: 'gnb',
    label: 'gNB',
    sublabel: 'free-ran-ue · slice1 + slice2',
    status,
    template: 'ulcl-2slice (two slices)',
    lastDeployed: slice1.lastDeployed !== '—' ? slice1.lastDeployed : slice2.lastDeployed,
  }
}

export default function DashboardPage() {
  const navigate = useNavigate()
  const [selected, setSelected] = useState<NodeId>('core')
  const [ueTerminalInstance, setUeTerminalInstance] = useState<string | null>(null)
  // which free5gc template to deploy next time - only relevant while core
  // is stopped; once running, the actual template is read back from its
  // live services instead (see detectFree5gcTemplate).
  const [selectedTemplate, setSelectedTemplate] = useState<Free5gcTemplate>('basic')

  const { errors, successes, addError, addSuccess, removeNotification } = useNotifications()
  const free5gc = useFree5gcStatus()
  const gnb = useGnbStatus()
  const gnbSlice1 = useGnbSliceStatus('slice1')
  const gnbSlice2 = useGnbSliceStatus('slice2')
  const ue = useUeInstances()

  const coreTemplate = detectFree5gcTemplate(free5gc.networkFunctions) ?? selectedTemplate
  const coreTemplateLabel = FREE5GC_TEMPLATE_OPTIONS.find((option) => option.value === coreTemplate)?.label ?? 'Basic'
  const isTwoSliceCore = coreTemplate === 'ulcl-2slice'

  // core and gnb (as one card) are single-instance targets with a uniform
  // deploy/stop/logs shape; ue is multi-instance (one per subscriber) and
  // gets its own panel. Under two-slice core, gnb's own deploy/stop aren't
  // actually used (handlePrimaryAction fans out to both slices instead) -
  // activeTarget just needs to be non-null so the button renders/enables.
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

  async function handleStopAllUe() {
    try {
      await ue.stopAll()
      addSuccess('All UE instances stopped')
    } catch (error) {
      addError(extractErrorMessage(error, 'Failed to stop all UE instances'))
    }
  }

  async function handlePrimaryAction() {
    // gNB stays a single card, but under two-slice core, deploying/stopping
    // it fans out to both independent slice targets at once.
    if (selected === 'gnb' && isTwoSliceCore) {
      const running = gnbSlice1.node.status !== 'stopped' || gnbSlice2.node.status !== 'stopped'
      try {
        if (running) {
          await Promise.allSettled([gnbSlice1.stop(), gnbSlice2.stop()])
          addSuccess('gNB stopped')
        } else {
          await Promise.allSettled([gnbSlice1.deploy(), gnbSlice2.deploy()])
          addSuccess('gNB deploy started')
        }
      } catch (error) {
        addError(extractErrorMessage(error, 'gNB action failed'))
      }
      return
    }

    if (!activeTarget) return

    try {
      if (activeTarget.node.status === 'running') {
        await activeTarget.stop()
        addSuccess(`${activeTarget.node.label} stopped`)
      } else {
        await activeTarget.deploy(selected === 'core' ? selectedTemplate : undefined)
        addSuccess(`${activeTarget.node.label} deploy started`)
      }
    } catch (error) {
      addError(extractErrorMessage(error, `${activeTarget.node.label} action failed`))
    }
  }

  function handleViewLogs() {
    navigate(`/logs?target=${selected}`)
  }

  const gnbDisplayNode = isTwoSliceCore ? computeGnbAggregateNode(gnbSlice1.node, gnbSlice2.node) : gnb.node

  const nodes: Record<NodeId, DeploymentNode> = {
    core: free5gc.node,
    gnb: gnbDisplayNode,
    ue: computeUeNode(ue.rows),
  }
  const selectedNode = nodes[selected]

  // when the gNB card represents both slices, show both slices' own
  // services in the panel instead of a single ambiguous "GNB" entry.
  const gnbNetworkFunctions: NetworkFunction[] = isTwoSliceCore
    ? [
      ...gnbSlice1.networkFunctions.map((nf) => ({ ...nf, name: `SLICE1-${nf.name}` })),
      ...gnbSlice2.networkFunctions.map((nf) => ({ ...nf, name: `SLICE2-${nf.name}` })),
    ]
    : gnb.networkFunctions

  const anyGnbRunning = nodes.gnb.status !== 'stopped'

  // mirrors the backend's own dependency-order enforcement (Processor.
  // checkStopDependents / checkDeployPrerequisite), just so the button is
  // disabled with a reason instead of only erroring after a click.
  const anyUeRunning = ue.rows.some((row) => row.status !== 'stopped')
  const actionBlockedReason = selected === 'core' && nodes.core.status === 'running' && anyGnbRunning
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
            description={`free5GC · ${coreTemplateLabel} template`}
            valueColor={getStatusMeta(nodes.core.status).color}
          />
          <StatsCard
            title="gNB"
            value={getStatusLabel(nodes.gnb.status)}
            description={isTwoSliceCore ? 'free-ran-ue · slice1 + slice2' : 'free-ran-ue · basic template'}
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
                onStopAll={handleStopAllUe}
                onOpenTerminal={setUeTerminalInstance}
                deployBlockedReason={!anyGnbRunning ? 'Deploy gNB before deploying a UE instance' : undefined}
              />
            ) : (
              <DetailPanel
                node={selectedNode}
                networkFunctions={selected === 'gnb' ? gnbNetworkFunctions : (activeTarget ? activeTarget.networkFunctions : [])}
                onViewLogs={handleViewLogs}
                onPrimaryAction={activeTarget ? handlePrimaryAction : undefined}
                isActionPending={
                  selected === 'gnb' && isTwoSliceCore
                    ? (gnbSlice1.isActionPending || gnbSlice2.isActionPending)
                    : (activeTarget?.isActionPending ?? false)
                }
                actionBlockedReason={actionBlockedReason}
                templateOptions={selected === 'core' ? FREE5GC_TEMPLATE_OPTIONS : undefined}
                selectedTemplate={selected === 'core' ? selectedTemplate : undefined}
                onTemplateChange={selected === 'core' ? (value) => setSelectedTemplate(value as Free5gcTemplate) : undefined}
              />
            )}
          </div>
        </section>

        <section className={styles.topologySection}>
          <div className={styles.topologyHeader}>
            <div>
              <h3>Detailed Network Topology</h3>
              <p>The core's SBI service mesh, N2/N3 to the gNB, and Uu to each UE currently attached.</p>
            </div>
          </div>

          <DetailedTopology
            free5gcNfs={free5gc.networkFunctions}
            gnbStatus={nodes.gnb.status}
            gnbSlice1Status={gnbSlice1.node.status}
            gnbSlice2Status={gnbSlice2.node.status}
            ueRows={ue.rows}
          />
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
