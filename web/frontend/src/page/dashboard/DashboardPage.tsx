import { useState, type ReactNode } from 'react'
import { useNavigate } from 'react-router-dom'
import AppLayout from '../../components/layout/AppLayout'
import NotificationContainer from '../../components/notifications/NotificationContainer'
import { useNotifications } from '../../hooks/useNotifications'
import { extractErrorMessage } from '../../apiClient'
import TopologySchematic, { type GnbSlice, type TopologySelection } from './TopologySchematic'
import SelectedNodePanel, { type DetailRow } from './SelectedNodePanel'
import UeInstancesPanel from './UeInstancesPanel'
import UeTerminalModal from './UeTerminalModal'
import { StatusSwatch, StatusTag } from './StatusTag'
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

// mirrors backend's constant.SLICE1_SD / SLICE2_SD - which gNB slice a
// UE attaches to under ulcl-2slice, from its subscriber's first session.
const SLICE1_SD = '010203'
const SLICE2_SD = '112233'

const LEGEND: NodeStatus[] = ['running', 'deploying', 'stopped', 'unhealthy']

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

  const coreNote = !coreStopped && nodes.core.lastDeployed !== '—'
    ? `${coreTemplateLabel} template · deployed ${nodes.core.lastDeployed}`
    : `free5GC · ${coreTemplateLabel} template`
  const gnbNote = anyGnbRunning && nodes.gnb.lastDeployed !== '—'
    ? `${isTwoSliceCore ? 'slice1 + slice2 · ' : ''}deployed ${nodes.gnb.lastDeployed}`
    : isTwoSliceCore ? 'free-ran-ue · slice1 + slice2' : 'free-ran-ue · basic template'
  const ueNote = ue.rows.length === 0
    ? 'no subscribers yet'
    : ueUnhealthy > 0
      ? `${ueUnhealthy} instance${ueUnhealthy === 1 ? '' : 's'} unhealthy`
      : ueStarting > 0
        ? `${ueStarting} instance${ueStarting === 1 ? '' : 's'} starting`
        : ueRunning === ue.rows.length
          ? 'all instances up'
          : 'running / subscribers'
  const nfNote = totalNfs === 0
    ? 'core not deployed'
    : unhealthyCount > 0
      ? `${unhealthyCount} NF${unhealthyCount === 1 ? '' : 's'} not running`
      : 'all network functions healthy'

  // under ulcl-2slice the one gNB node stands for both slice gNBs; each
  // up UE is counted on the slice its subscriber's sd belongs to
  const upUes = ue.rows.filter((row) => row.status !== 'stopped')
  const gnbSlices: GnbSlice[] | null = isTwoSliceCore
    ? [
      { name: 'slice1', status: gnbSlice1.node.status, ueCount: upUes.filter((row) => row.sd === SLICE1_SD).length },
      { name: 'slice2', status: gnbSlice2.node.status, ueCount: upUes.filter((row) => row.sd === SLICE2_SD).length },
    ]
    : null

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
  const selectedUeRow = selected === 'ue' ? ue.rows.find((row) => row.ueId === selection.ue) ?? null : null
  const ueDeployBlockedReason = !anyGnbRunning ? 'Deploy gNB before deploying a UE instance' : undefined

  const selectedLabel = selected === 'core'
    ? (selectedNf?.name ?? 'free5GC core')
    : selected === 'gnb' ? 'gNB' : (selectedUeRow?.ueId ?? 'UE')

  function selectUe(ueId: string) {
    setSelection({ node: 'ue', nf: null, ue: ueId })
  }

  let aside: ReactNode
  if (selected === 'ue') {
    const rows: DetailRow[] = [
      { k: 'Simulator', v: 'free-ran-ue' },
      { k: 'Template', v: 'basic' },
      { k: 'Instances', v: `${ueRunning} / ${ue.rows.length} running`, mono: true },
      ...(selectedUeRow
        ? [
          { k: 'PLMN', v: selectedUeRow.plmnId, mono: true },
          ...(selectedUeRow.gpsi ? [{ k: 'GPSI', v: selectedUeRow.gpsi, mono: true }] : []),
          { k: 'Last deployed', v: selectedUeRow.lastDeployed, mono: true },
        ]
        : [{ k: 'Last deployed', v: nodes.ue.lastDeployed, mono: true }]),
    ]
    const rowPending = selectedUeRow ? (ue.pendingInstances[selectedUeRow.ueId] ?? false) : false
    const anyUePending = upUes.some((row) => ue.pendingInstances[row.ueId])
    aside = (
      <SelectedNodePanel
        name={selectedUeRow?.ueId ?? 'UE'}
        status={selectedUeRow?.status ?? nodes.ue.status}
        statusText={selectedUeRow ? undefined : nodes.ue.statusLabel}
        rows={rows}
        hint={!selectedUeRow || selectedUeRow.status === 'stopped' ? ueDeployBlockedReason : undefined}
        actions={selectedUeRow ? (
          <>
            {selectedUeRow.status !== 'stopped' ? (
              <button type="button" className={`${styles.btn} ${styles.btnDanger}`} onClick={() => handleStopUe(selectedUeRow.ueId)} disabled={rowPending}>
                {rowPending ? 'Stopping…' : 'Stop UE'}
              </button>
            ) : (
              <button type="button" className={`${styles.btn} ${styles.btnPrimary}`} onClick={() => handleDeployUe(selectedUeRow.ueId)} disabled={rowPending || Boolean(ueDeployBlockedReason)}>
                {rowPending ? 'Deploying…' : 'Deploy UE'}
              </button>
            )}
            {selectedUeRow.status === 'running' && (
              <button type="button" className={styles.btn} onClick={() => setUeTerminalInstance(selectedUeRow.ueId)}>Terminal</button>
            )}
            <button type="button" className={styles.btn} onClick={handleViewLogs}>View logs</button>
          </>
        ) : (
          <>
            {upUes.length > 0 && (
              <button type="button" className={`${styles.btn} ${styles.btnDanger}`} onClick={handleStopAllUe} disabled={anyUePending}>
                {anyUePending ? 'Stopping…' : 'Stop all'}
              </button>
            )}
            <button type="button" className={styles.btn} onClick={handleViewLogs}>View logs</button>
          </>
        )}
      />
    )
  } else {
    let name = selectedNode.label
    let status: NodeStatus = selectedNode.status
    let rows: DetailRow[]
    if (selected === 'core' && selectedNf) {
      name = selectedNf.name
      status = selectedNf.status
      rows = [
        { k: 'Service', v: selectedNf.name.toLowerCase(), mono: true },
        { k: 'Part of', v: `free5GC · ${coreTemplateLabel}` },
        { k: 'Core status', v: getStatusMeta(nodes.core.status).text },
        { k: 'Last deployed', v: nodes.core.lastDeployed, mono: true },
      ]
    } else if (selected === 'core') {
      name = 'free5GC core'
      rows = [
        { k: 'Stack', v: 'free5GC' },
        { k: 'Template', v: coreTemplateLabel },
        { k: 'Last deployed', v: nodes.core.lastDeployed, mono: true },
      ]
    } else {
      rows = [
        { k: 'Simulator', v: 'free-ran-ue' },
        { k: 'Template', v: isTwoSliceCore ? 'ulcl-2slice (two slices)' : 'basic' },
        { k: 'Last deployed', v: nodes.gnb.lastDeployed, mono: true },
      ]
    }
    aside = (
      <SelectedNodePanel
        name={name}
        status={status}
        rows={rows}
        services={selected === 'core' ? free5gc.networkFunctions : gnbNetworkFunctions}
        selectedService={selected === 'core' ? selection.nf : null}
        onSelectService={selected === 'core' ? (nf) => setSelection({ node: 'core', nf, ue: null }) : undefined}
        hint={actionBlockedReason}
        actions={(
          <>
            <button
              type="button"
              className={`${styles.btn} ${isSelectedUp ? styles.btnDanger : styles.btnPrimary}`}
              onClick={handlePrimaryAction}
              disabled={!activeTarget || isActionPending || Boolean(actionBlockedReason)}
            >
              {primaryLabel}
            </button>
            <button type="button" className={styles.btn} onClick={handleViewLogs}>View logs</button>
          </>
        )}
      />
    )
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
        <h1 className={styles.title}>Lab dashboard</h1>
        <span className={styles.subtitle}>free5GC core network and the free-ran-ue gNB / UE simulator</span>
        <div className={styles.headActions}>
          {coreStopped && (
            <label className={styles.templatePicker}>
              <span>Template</span>
              <select
                className={styles.select}
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
          <button type="button" className={styles.btn} onClick={handleTearDown} disabled={!anythingUp || isTearingDown}>
            {isTearingDown ? 'Tearing down…' : 'Tear down'}
          </button>
          <button type="button" className={`${styles.btn} ${styles.btnPrimary}`} onClick={handleDeployCore} disabled={!coreStopped || free5gc.isActionPending}>
            {free5gc.isActionPending && coreStopped ? 'Deploying…' : 'Deploy core'}
          </button>
        </div>
      </section>

      <section className={styles.meta} aria-label="Deployment status">
        <div>
          <span>Core network</span>
          <b><StatusTag status={nodes.core.status} /><em>{coreNote}</em></b>
        </div>
        <div>
          <span>gNB</span>
          <b><StatusTag status={nodes.gnb.status} /><em>{gnbNote}</em></b>
        </div>
        <div>
          <span>UE instances</span>
          <b><StatusTag status={nodes.ue.status} /><span className="num">{ueRunning} / {ue.rows.length}</span><em>{ueNote}</em></b>
        </div>
        <div>
          <span>Healthy NFs</span>
          <b><span className="num">{totalNfs > 0 ? `${healthyCount} / ${totalNfs}` : '—'}</span><em>{nfNote}</em></b>
        </div>
      </section>

      <section className={styles.panel} aria-label="Network topology">
        <div className={styles.ph}>
          <h2>Network topology</h2>
          {LEGEND.map((status) => (
            <span key={status} className={styles.legend}><StatusSwatch status={status} />{getStatusMeta(status).text}</span>
          ))}
          <span className={styles.aux}>selected: {selectedLabel}</span>
        </div>
        <TopologySchematic
          template={coreTemplate}
          templateLabel={coreTemplateLabel}
          coreStatus={nodes.core.status}
          coreNfs={free5gc.networkFunctions}
          gnbStatus={nodes.gnb.status}
          gnbTemplate={isTwoSliceCore ? 'ulcl-2slice template' : 'basic template'}
          gnbSlices={gnbSlices}
          ueStatus={nodes.ue.status}
          ueRows={ue.rows}
          selection={selection}
          onSelect={setSelection}
        />
      </section>

      <div className={styles.dash}>
        <div className={styles.dashMain}>
          <UeInstancesPanel
            rows={ue.rows}
            isLoading={ue.isLoadingSubscribers}
            loadError={ue.subscribersError}
            pendingInstances={ue.pendingInstances}
            selectedUe={selected === 'ue' ? selection.ue : null}
            onSelectUe={selectUe}
            onDeploy={handleDeployUe}
            onStop={handleStopUe}
            onStopAll={handleStopAllUe}
            onOpenTerminal={setUeTerminalInstance}
            deployBlockedReason={ueDeployBlockedReason}
          />
        </div>

        {aside}
      </div>

      <UeTerminalModal
        instance={ueTerminalInstance}
        onClose={() => setUeTerminalInstance(null)}
      />
    </AppLayout>
  )
}
