import { useEffect, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import AppLayout from '../../components/layout/AppLayout'
import LabCanvas from '../../lab/canvas/LabCanvas'
import { buildOverviewGraph } from '../../lab/canvas/graph'
import { NodeIcon, type IconName } from '../../lab/canvas/icons'
import { useEvents } from '../../lab/console/EventsContext'
import { useLab } from '../../lab/LabContext'
import { clockOf, eventNode, eventSource, levelStatus } from '../../lab/console/eventSource'
import { Actions, Button, Dot, KV, Section, Segmented, ui } from '../../lab/LabUi'
import { isFree5gcCore, templateLabel } from '../../lab/useLab'
import Workbench from '../../lab/workbench/Workbench'
import { logsHref } from '../../lab/workbench/logSources'
import TerminalDock from '../../lab/workbench/TerminalDock'
import TwinView from '../../lab/twin/TwinView'
import { useNotifications } from '../../hooks/useNotifications'
import { useConfirm } from '../../components/feedback/Feedback'
import { extractErrorMessage } from '../../apiClient'
import { useStoredState } from '../../lab/useProfiles'
import NodeInspector from '../../lab/workbench/NodeInspector'
import empty from './empty-lab.module.css'

const STEPS: { n: number; icon: IconName; title: string; text: string; to: string }[] = [
  { n: 1, icon: 'core', title: 'Core Network', text: 'Deploy the lab free5GC from a template (Basic or ULCL), add more free5GC cores on this host, or add a core outside the lab under Core Network.', to: '/core-network' },
  { n: 2, icon: 'gnb', title: 'free-ran', text: 'Save gNB profiles and bring up as many gNBs as you need, on the lab free5GC or pointed at any AMF and UPF you can reach.', to: '/free-ran' },
  { n: 3, icon: 'ue', title: 'free-ue', text: 'Launch a UE from each UE profile onto the gNB you pick, then test its data path.', to: '/free-ue' },
]

// EmptyLab is the overview before anything runs: the three steps.
function EmptyLab() {
  const navigate = useNavigate()
  return (
    <section className={empty.wrap} aria-label="Get started">
      <div className={empty.head}>
        <h1>Nothing is running</h1>
        <p>Bring the lab up in three steps. The topology, inspector and console appear here as soon as the first piece is deployed.</p>
      </div>
      <ol className={empty.steps}>
        {STEPS.map((s) => (
          <li key={s.n}>
            <button type="button" className={empty.step} onClick={() => navigate(s.to)}>
              <span className={empty.num}>{s.n}</span>
              <span className={empty.icon}><svg width="27" height="27" viewBox="0 0 24 24" aria-hidden="true"><NodeIcon name={s.icon} x={0} y={0} size={24} /></svg></span>
              <span className={empty.text}><b>{s.title}</b>{s.text}</span>
              <span className={empty.go} aria-hidden="true">→</span>
            </button>
          </li>
        ))}
      </ol>
    </section>
  )
}

// DashboardPage is the overview: the whole lab on one topology, read-only.
// Each node links to the view that manages it and to its logs; a running
// UE or gNB opens a shell in the terminal dock under the topology
// (double-click its node; ?terminal=<ue> opens one from another page). Events and Logs are the Overview's
// sub-pages.
export default function DashboardPage() {
  const navigate = useNavigate()
  const lab = useLab()
  const { ueLive, events } = useEvents()
  const [selected, setSelected] = useState<string | null>(null)
  const { nodes, ueCounts } = lab
  const [params, setParams] = useSearchParams()
  // the open shells, by node id (ue:<imsi>, gnb-<profileId>); they survive
  // leaving the page and reconnect on return
  const [sessions, setSessions] = useStoredState<string[]>('frulab-terminal-nodes', [])
  const [activeTerm, setActiveTerm] = useState<string | null>(null)
  // the logical topology or the 2D digital twin
  const [view, setView] = useStoredState<'topology' | 'twin'>('frulab-overview-view', 'topology')
  const { addError, addSuccess } = useNotifications()
  const confirm = useConfirm()
  // what Tear down is stopping right now; null when idle
  const [tearing, setTearing] = useState<string | null>(null)

  // Tear down stops the whole lab in the order the backend requires: every
  // UE, then every gNB, then the lab core and every free5GC core network
  async function tearDown() {
    const ues = lab.upUes.length
    const gnbs = lab.gnbs
    const cores = lab.upCores
    const what = [
      ues && `${ues} UE${ues === 1 ? '' : 's'}`,
      gnbs.length && `${gnbs.length} gNB${gnbs.length === 1 ? '' : 's'}`,
      lab.coreUp && 'the free5GC core',
      cores.length && `the free5GC core network${cores.length === 1 ? '' : 's'} ${cores.map((c) => c.name).join(', ')}`,
    ].filter(Boolean).join(', ')
    if (!await confirm({ title: 'Tear down the whole lab?', message: <>This stops {what}, in that order. Their profiles stay; deploy them again any time.</>, confirmLabel: 'Tear down', danger: true })) return
    try {
      if (ues) { setTearing('Stopping UEs…'); await lab.ue.stopAll() }
      for (const g of gnbs) { setTearing(`Stopping ${g.profileName}…`); await g.stop() }
      if (lab.coreUp) { setTearing('Stopping the core…'); await lab.free5gc.stop() }
      for (const c of cores) { setTearing(`Stopping ${c.name}…`); await c.stop() }
      setSessions([])
      addSuccess('The lab is torn down')
    } catch (error) {
      addError(extractErrorMessage(error, 'Tear down stopped part way'))
    } finally {
      setTearing(null)
    }
  }

  // a node a shell can open in: a running UE or a running profile gNB
  function canShell(id: string): boolean {
    if (id.startsWith('ue:')) return lab.ue.rows.some((r) => r.ueId === id.slice(3) && r.status === 'running')
    return lab.gnbs.some((g) => g.id === id && g.status === 'running')
  }
  function openTerminal(id: string) {
    if (!sessions.includes(id)) setSessions([...sessions, id])
    setActiveTerm(id)
  }
  const terminalLabel = (id: string) => (id.startsWith('ue:') ? id.slice(3).replace(/^imsi-/, '') : `gNB ${lab.gnbs.find((g) => g.id === id)?.profileName ?? id}`)
  // another page asked for a terminal
  const asked = params.get('terminal')
  useEffect(() => {
    if (!asked) return
    const node = asked.startsWith('gnb-') || asked.startsWith('ue:') ? asked : `ue:${asked}`
    openTerminal(node)
    setSelected(node)
    setParams({}, { replace: true })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [asked])

  const coresUp = lab.upCores.length > 0
  if (!lab.loaded || !(lab.coreUp || coresUp || lab.gnbUp || lab.ueUp || lab.coreNetworks.networks.some((n) => n.enabled && !isFree5gcCore(n)))) {
    return (
      <AppLayout crumbs={[{ label: 'Overview' }]}>
        {lab.loaded ? <EmptyLab /> : <p className={ui.empty}>Checking the lab…</p>}
      </AppLayout>
    )
  }

  // a running UE or gNB gets a terminal button in the inspector (or a
  // double-click on its node)
  function terminalButton(id: string) {
    if (!canShell(id)) return undefined
    return <Section><Button kind="primary" small onClick={() => openTerminal(id)}>{sessions.includes(id) ? 'Show terminal' : 'Open terminal'}</Button><p className={ui.empty} style={{ padding: '0.375rem 0 0' }}>Tip: double-click the node.</p></Section>
  }

  const graph = buildOverviewGraph(lab, ueLive)
  // every event, newest first (the Events page has them all, searchable)
  const recent = events.slice(-200).reverse()
  const coreText = lab.gnbExternal && !lab.coreUp ? 'external' : lab.runningTemplate ? templateLabel(lab.runningTemplate) : 'off'
  const withPdu = lab.ue.rows.filter((r) => r.status === 'running' && ueLive[r.ueId]?.ip).length

  const summary = (
    <>
      <Section title="Lab">
        <KV rows={[
          { k: 'Core', v: <><Dot status={lab.gnbExternal && !lab.coreUp ? 'running' : nodes.core.status} /> {coreText}</> },
          { k: 'NFs healthy', v: lab.free5gc.networkFunctions.length ? `${lab.healthyNfs}/${lab.free5gc.networkFunctions.length}` : '—' },
          ...lab.upCores.map((c) => ({ k: c.name, v: <><Dot status={c.status} /> {c.template ? templateLabel(c.template) : c.status}{c.networkFunctions.length ? ` · ${c.healthyNfs}/${c.networkFunctions.length} NF` : ''}</> })),
          { k: 'gNBs running', v: lab.gnbs.length },
          { k: 'UEs connected', v: `${ueCounts.running} / ${ueCounts.total}` },
          { k: 'UEs with PDU session', v: withPdu },
        ]} />
        <Actions>
          <Button small onClick={() => navigate('/core-network/lab')}>free5GC</Button>
          <Button small onClick={() => navigate('/free-ran')}>free-ran</Button>
          <Button small onClick={() => navigate('/free-ue')}>free-ue</Button>
        </Actions>
      </Section>
      <Section><p className={ui.empty} style={{ padding: 0 }}>Select a node for its details and its own events.</p></Section>
      <Section title="Events" aside={<button type="button" className={ui.linkBtn} onClick={() => navigate('/events')}>all {events.length} →</button>}>
        {recent.length === 0
          ? <p className={ui.empty} style={{ padding: 0 }}>No events yet.</p>
          : (
            <ul className={`${ui.list} ${ui.eventList}`}>
              {recent.map((e) => (
                <li key={e.id}>
                  <button type="button" onClick={() => setSelected(eventNode(e))} title={`${clockOf(e.t)} · ${e.text}`}>
                    <Dot status={levelStatus(e.level)} />
                    <span className={ui.eventText}><b>{eventSource(lab, e)}</b> {e.text}</span>
                    <span>{new Date(e.t).toISOString().slice(11, 19)}</span>
                  </button>
                </li>
              ))}
            </ul>
          )}
      </Section>
    </>
  )

  return (
    <AppLayout crumbs={[{ label: 'Overview' }]} fill>
      <Workbench
        title="Overview"
        actions={(lab.coreUp || coresUp || lab.gnbUp || lab.ueUp || tearing) ? <Button kind="danger" busy={Boolean(tearing)} title="Stop every UE, every gNB and every core fru-lab deployed" onClick={tearDown}>{tearing ?? 'Tear down'}</Button> : undefined}
        status={<><Segmented value={view} options={[{ value: 'topology', label: 'Topology' }, { value: 'twin', label: 'Twin' }]} onChange={setView} /><span>core <b>{coreText}</b></span>{coresUp && <span>free5GC cores <b>{lab.upCores.length}</b></span>}<span>gNBs <b>{lab.gnbs.length}</b></span><span>UEs <b>{ueCounts.running}/{ueCounts.total}</b></span><span>PDU sessions <b>{withPdu}</b></span></>}
        canvas={(
          <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
            <div style={{ position: 'relative', flex: '1 1 auto', minHeight: 0 }}>
              {view === 'twin'
                ? <TwinView selectedId={selected} onSelect={setSelected} onOpen={(id) => { setSelected(id); if (canShell(id)) openTerminal(id) }} />
                : (
                <LabCanvas
                  label="The lab: each connected UE on the radio to its gNB, every running gNB over N2 and N3 to its core, N6 out to the data network."
                  {...graph}
                  selectedId={selected}
                  onSelect={setSelected}
                  onOpen={(id) => { setSelected(id); if (canShell(id)) openTerminal(id) }}
                />
                )}
            </div>
            <TerminalDock
              sessions={sessions}
              active={activeTerm}
              onActive={setActiveTerm}
              onClose={(id) => setSessions(sessions.filter((x) => x !== id))}
              onCloseAll={() => setSessions([])}
              labelOf={terminalLabel}
            />
          </div>
        )}
        inspector={selected ? <NodeInspector id={selected} onLogs={() => navigate(logsHref(selected))} extra={terminalButton(selected)} /> : summary}
      />
    </AppLayout>
  )
}
