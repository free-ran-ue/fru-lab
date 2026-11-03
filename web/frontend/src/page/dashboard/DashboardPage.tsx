import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import AppLayout from '../../components/layout/AppLayout'
import Button from '../../components/button/button'
import NotificationContainer from '../../components/notifications/NotificationContainer'
import { useNotifications } from '../../hooks/useNotifications'
import { extractErrorMessage } from '../../apiClient'
import TopologyCanvas, { type TopologySelection } from './TopologyCanvas'
import DetailedTopology from './DetailedTopology'
import DetailPanel, { type DetailRow } from './DetailPanel'
import UePanel from './UePanel'
import UeTerminalModal from './UeTerminalModal'
import StatusDot from './StatusDot'
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

interface Kpi {
  label: string
  value: string
  sub: string
  status: NodeStatus
}

export default function DashboardPage() {
  const navigate = useNavigate()
  const [selection, setSelection] = useState<TopologySelection>({ node: 'core', nf: null, ue: null })
  const selected: NodeId = selection.node
  const [ueTerminalInstance, setUeTerminalInstance] = useState<string | null>(null)
  const [isTearingDown, setIsTearingDown] = useState(false)
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
      // anything not fully stopped can be stopped, including a deploy that
      // is stuck (e.g. a container that could not start stays "deploying")
      if (activeTarget.node.status !== 'stopped') {
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

  async function handleDeployCore() {
    try {
      await free5gc.deploy(selectedTemplate)
      addSuccess(`${free5gc.node.label} deploy started`)
    } catch (error) {
      addError(extractErrorMessage(error, `${free5gc.node.label} action failed`))
    }
  }

  // Tear down the whole lab in dependency order: UEs, then gNB (both
  // slices under ulcl-2slice), then the core - the same order the backend
  // enforces for single stops.
  async function handleTearDown() {
    if (!window.confirm('Stop every UE instance, the gNB and the core network?')) return
    setIsTearingDown(true)
    try {
      if (ue.rows.some((row) => row.status !== 'stopped')) await ue.stopAll()
      const gnbStops: Promise<void>[] = []
      if (gnb.node.status !== 'stopped') gnbStops.push(gnb.stop())
      if (gnbSlice1.node.status !== 'stopped') gnbStops.push(gnbSlice1.stop())
      if (gnbSlice2.node.status !== 'stopped') gnbStops.push(gnbSlice2.stop())
      await Promise.all(gnbStops)
      if (free5gc.node.status !== 'stopped') await free5gc.stop()
      addSuccess('Lab torn down')
    } catch (error) {
      addError(extractErrorMessage(error, 'Tear down failed'))
    } finally {
      setIsTearingDown(false)
    }
  }

  function handleViewLogs() {
    if (selected === 'ue') {
      const row = ue.rows.find((r) => r.ueId === selection.ue)
      navigate(row && row.hasBeenDeployed ? `/logs?target=ue&instance=${encodeURIComponent(row.ueId)}` : '/logs?target=ue')
      return
    }
    navigate(`/logs?target=${selected === 'gnb' && isTwoSliceCore ? 'gnb-slice1' : selected}`)
  }

  const gnbDisplayNode = isTwoSliceCore ? computeGnbAggregateNode(gnbSlice1.node, gnbSlice2.node) : gnb.node

  const nodes: Record<NodeId, DeploymentNode> = {
    core: free5gc.node,
    gnb: gnbDisplayNode,
    ue: computeUeNode(ue.rows),
  }

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
  // the primary action is Stop for anything not stopped (a stuck deploy
  // included) and Deploy otherwise, so the rules follow the same split
  const actionBlockedReason = selected === 'core' && nodes.core.status !== 'stopped' && anyGnbRunning
    ? 'Stop gNB before stopping the core network'
    : selected === 'gnb' && nodes.gnb.status !== 'stopped' && anyUeRunning
      ? 'Stop all UE instances before stopping gNB'
      : selected === 'gnb' && nodes.gnb.status === 'stopped' && nodes.core.status !== 'running'
        ? 'Deploy the core network before deploying gNB'
        : undefined

  const healthyCount = free5gc.networkFunctions.filter((nf) => nf.status === 'running').length
  const totalNfs = free5gc.networkFunctions.length
  const unhealthyCount = totalNfs - healthyCount

  const coreStopped = nodes.core.status === 'stopped'
  const anythingUp = !coreStopped || anyGnbRunning || anyUeRunning
    || gnb.node.status !== 'stopped' || gnbSlice1.node.status !== 'stopped' || gnbSlice2.node.status !== 'stopped'

  const ueRunning = ue.rows.filter((row) => row.status === 'running').length
  const ueStarting = ue.rows.filter((row) => row.status === 'deploying').length
  const ueUnhealthy = ue.rows.filter((row) => row.status === 'unhealthy').length

  const kpis: Kpi[] = [
    {
      label: 'Core network',
      value: getStatusMeta(nodes.core.status).text,
      sub: !coreStopped && nodes.core.lastDeployed !== '—'
        ? `deployed ${nodes.core.lastDeployed}`
        : `free5GC · ${coreTemplateLabel} template`,
      status: nodes.core.status,
    },
    {
      label: 'gNB',
      value: getStatusMeta(nodes.gnb.status).text,
      sub: anyGnbRunning && nodes.gnb.lastDeployed !== '—'
        ? `${isTwoSliceCore ? 'slice1 + slice2 · ' : ''}deployed ${nodes.gnb.lastDeployed}`
        : isTwoSliceCore ? 'free-ran-ue · slice1 + slice2' : 'free-ran-ue · basic template',
      status: nodes.gnb.status,
    },
    {
      label: 'UE',
      value: `${ueRunning} / ${ue.rows.length}`,
      sub: ue.rows.length === 0
        ? 'no subscribers yet'
        : ueUnhealthy > 0
          ? `${ueUnhealthy} instance${ueUnhealthy === 1 ? '' : 's'} unhealthy`
          : ueStarting > 0
            ? `${ueStarting} instance${ueStarting === 1 ? '' : 's'} starting`
            : ueRunning === ue.rows.length
              ? 'all instances up'
              : 'instances running / subscribers',
      status: nodes.ue.status,
    },
    {
      label: 'Healthy NFs',
      value: totalNfs > 0 ? `${healthyCount} / ${totalNfs}` : '—',
      sub: totalNfs === 0
        ? 'core not deployed'
        : unhealthyCount > 0
          ? `${unhealthyCount} NF${unhealthyCount === 1 ? '' : 's'} not running`
          : 'all containers up',
      status: totalNfs === 0 ? 'stopped' : unhealthyCount > 0 ? 'unhealthy' : 'running',
    },
  ]

  // ----- the "Selected node" aside -----
  const isActionPending = selected === 'gnb' && isTwoSliceCore
    ? (gnbSlice1.isActionPending || gnbSlice2.isActionPending)
    : (activeTarget?.isActionPending ?? false)
  const selectedNode = nodes[selected]
  const isSelectedUp = selectedNode.status !== 'stopped'
  const targetWord = selected === 'core' ? 'core' : 'gNB'
  const primaryLabel = isActionPending
    ? (isSelectedUp ? 'Stopping…' : 'Deploying…')
    : `${isSelectedUp ? 'Stop' : 'Deploy'} ${targetWord}`

  const selectedNf = selected === 'core' && selection.nf
    ? free5gc.networkFunctions.find((nf) => nf.name === selection.nf) ?? null
    : null

  let asideName = selectedNode.label
  let asideStatus: NodeStatus = selectedNode.status
  let asideRows: DetailRow[] = []
  if (selected === 'core' && selectedNf) {
    asideName = selectedNf.name
    asideStatus = selectedNf.status
    asideRows = [
      { k: 'Service', v: selectedNf.name.toLowerCase() },
      { k: 'Part of', v: `free5GC · ${coreTemplateLabel}` },
      { k: 'Core status', v: getStatusMeta(nodes.core.status).text },
      { k: 'Last deployed', v: nodes.core.lastDeployed },
    ]
  } else if (selected === 'core') {
    asideName = '5G core'
    asideRows = [
      { k: 'Stack', v: 'free5GC' },
      { k: 'Template', v: coreTemplateLabel },
      { k: 'Running NFs', v: totalNfs > 0 ? `${healthyCount} / ${totalNfs}` : '—' },
      { k: 'Last deployed', v: nodes.core.lastDeployed },
    ]
  } else if (selected === 'gnb') {
    asideRows = [
      { k: 'Simulator', v: 'free-ran-ue' },
      { k: 'Template', v: isTwoSliceCore ? 'ulcl-2slice (two slices)' : 'basic' },
      ...gnbNetworkFunctions.map((nf) => ({ k: nf.name, v: getStatusMeta(nf.status).text })),
      { k: 'Last deployed', v: nodes.gnb.lastDeployed },
    ]
  }

  return (
    <AppLayout
      crumbs={[{ label: 'Dashboard' }]}
      overlay={(
        <NotificationContainer
          errors={errors}
          successes={successes}
          onClose={removeNotification}
        />
      )}
    >
      <section className={styles.pageHead}>
        <div className={styles.titleBlock}>
          <div className={styles.titleRow}>
            <h1 className={styles.title}>Lab dashboard</h1>
            <span className={styles.templateTag}>TEMPLATE · {coreTemplateLabel.toUpperCase()}</span>
          </div>
          <p className={styles.subtitle}>free5GC, gNB and UEs deployed on this host's Docker.</p>
        </div>
        <div className={styles.headActions}>
          {coreStopped && (
            <label className={styles.templatePicker}>
              <span>Template</span>
              <select
                className={styles.templateSelect}
                value={selectedTemplate}
                onChange={(event) => setSelectedTemplate(event.target.value as Free5gcTemplate)}
                disabled={free5gc.isActionPending}
              >
                {FREE5GC_TEMPLATE_OPTIONS.map((option) => (
                  <option key={option.value} value={option.value}>{option.label}</option>
                ))}
              </select>
            </label>
          )}
          <Button variant="secondary" onClick={handleTearDown} disabled={!anythingUp || isTearingDown}>
            {isTearingDown ? 'Tearing down…' : 'Tear down'}
          </Button>
          <Button variant="primary" onClick={handleDeployCore} disabled={!coreStopped || free5gc.isActionPending}>
            {free5gc.isActionPending && coreStopped ? 'Deploying…' : 'Deploy core'}
          </Button>
        </div>
      </section>

      <section aria-label="Lab health" className={styles.kpis}>
        {kpis.map((k) => (
          <div key={k.label} className={styles.kpi}>
            <span className={styles.kpiLabel}>{k.label}</span>
            <span className={styles.kpiValue}><StatusDot status={k.status} size={10} /><span className="num">{k.value}</span></span>
            <span className={styles.kpiSub}>{k.sub}</span>
          </div>
        ))}
      </section>

      <section className={styles.topologyRow}>
        <div className={styles.panel}>
          <div className={styles.panelHead}>
            <h2 className={styles.panelTitle}>Network topology</h2>
            <span className={styles.panelNote}>Select a node for details</span>
          </div>
          <TopologyCanvas
            ueRows={ue.rows}
            gnb={nodes.gnb}
            gnbDetail={isTwoSliceCore ? 'slice1 + slice2' : 'free-ran-ue'}
            coreStatus={nodes.core.status}
            coreNfs={free5gc.networkFunctions}
            selection={selection}
            onSelect={setSelection}
          />
          <div className={styles.legend}>
            {(['running', 'deploying', 'unhealthy', 'stopped'] as NodeStatus[]).map((status) => (
              <span key={status} className={styles.legendItem}>
                <StatusDot status={status} />{getStatusMeta(status).text}
              </span>
            ))}
          </div>
        </div>

        {selected === 'ue' ? (
          <UePanel
            node={nodes.ue}
            rows={ue.rows}
            selectedUe={selection.ue}
            onSelectUe={(ueId) => setSelection({ node: 'ue', nf: null, ue: ueId })}
            isLoading={ue.isLoadingSubscribers}
            loadError={ue.subscribersError}
            pendingInstances={ue.pendingInstances}
            onDeploy={handleDeployUe}
            onStop={handleStopUe}
            onStopAll={handleStopAllUe}
            onOpenTerminal={setUeTerminalInstance}
            onViewLogs={handleViewLogs}
            deployBlockedReason={!anyGnbRunning ? 'Deploy gNB before deploying a UE instance' : undefined}
          />
        ) : (
          <DetailPanel
            name={asideName}
            status={asideStatus}
            rows={asideRows}
            hint={actionBlockedReason}
            footer={(
              <>
                <Button variant="secondary" onClick={handleViewLogs}>View logs</Button>
                <Button
                  variant={isSelectedUp ? 'secondary' : 'primary'}
                  onClick={handlePrimaryAction}
                  disabled={!activeTarget || isActionPending || Boolean(actionBlockedReason)}
                >
                  {primaryLabel}
                </Button>
              </>
            )}
          />
        )}
      </section>

      <section className={styles.panel}>
        <div className={styles.panelHead}>
          <h2 className={styles.panelTitle}>Detailed network topology</h2>
          <span className={styles.panelNote}>SBI service mesh, N2 / N3 to the gNB and Uu to each attached UE</span>
        </div>
        <div className={styles.detailedWrap}>
          <DetailedTopology
            free5gcNfs={free5gc.networkFunctions}
            gnbStatus={nodes.gnb.status}
            gnbSlice1Status={gnbSlice1.node.status}
            gnbSlice2Status={gnbSlice2.node.status}
            ueRows={ue.rows}
          />
        </div>
      </section>

      <UeTerminalModal
        instance={ueTerminalInstance}
        onClose={() => setUeTerminalInstance(null)}
      />
    </AppLayout>
  )
}
