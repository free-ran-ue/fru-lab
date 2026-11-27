import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import AppLayout from '../../components/layout/AppLayout'
import LabCanvas from '../../lab/canvas/LabCanvas'
import { buildGraph } from '../../lab/canvas/graph'
import { NodeIcon, type IconName } from '../../lab/canvas/icons'
import { useEvents } from '../../lab/console/EventsContext'
import { useLab } from '../../lab/LabContext'
import { Actions, Button, Dot, KV, Section, ui } from '../../lab/LabUi'
import { templateLabel } from '../../lab/useLab'
import Workbench, { logSourceFor } from '../../lab/workbench/Workbench'
import NodeInspector from '../../lab/workbench/NodeInspector'
import { useConsole } from '../../lab/workbench/Console'
import empty from './empty-lab.module.css'

const STEPS: { n: number; icon: IconName; title: string; text: string; to: string }[] = [
  { n: 1, icon: 'core', title: 'Core Network', text: 'Deploy free5GC from a template: Basic, ULCL or Two Slice. Skip it if your gNB will use a core outside the lab.', to: '/core-network' },
  { n: 2, icon: 'gnb', title: 'free-ran', text: 'Bring up a gNB from a profile, on the lab free5GC or pointed at any AMF and UPF you can reach.', to: '/free-ran' },
  { n: 3, icon: 'ue', title: 'free-ue', text: 'Launch UEs from the free5GC subscribers or your own UE profiles, then test their data path.', to: '/free-ue' },
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
              <span className={empty.icon}><svg width="18" height="18" viewBox="0 0 24 24" aria-hidden="true"><NodeIcon name={s.icon} x={0} y={0} size={24} /></svg></span>
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
// Each node links to the view that manages it; the console follows every
// component's logs and events.
export default function DashboardPage() {
  const navigate = useNavigate()
  const lab = useLab()
  const { ueLive, events } = useEvents()
  const con = useConsole()
  const [selected, setSelected] = useState<string | null>(null)
  const { nodes, ueCounts } = lab

  if (!lab.loaded || !(lab.coreUp || lab.gnbUp || lab.ueUp)) {
    return (
      <AppLayout crumbs={[{ label: 'Overview' }]}>
        {lab.loaded ? <EmptyLab /> : <p className={ui.empty}>Checking the lab…</p>}
      </AppLayout>
    )
  }

  const graph = buildGraph(lab, 'all', 'basic', ueLive)
  const errors = events.filter((e) => e.level === 'error').slice(-5).reverse()
  const coreText = lab.gnbExternal && !lab.coreUp ? 'external' : lab.runningTemplate ? templateLabel(lab.runningTemplate) : 'off'
  const withPdu = lab.ue.rows.filter((r) => r.status === 'running' && ueLive[r.ueId]?.ip).length

  const summary = (
    <>
      <Section title="Lab">
        <KV rows={[
          { k: 'Core', v: <><Dot status={lab.gnbExternal && !lab.coreUp ? 'running' : nodes.core.status} /> {coreText}</> },
          { k: 'NFs healthy', v: lab.free5gc.networkFunctions.length ? `${lab.healthyNfs}/${lab.free5gc.networkFunctions.length}` : '—' },
          { k: 'gNB', v: <><Dot status={nodes.gnb.status} /> {nodes.gnb.status === 'stopped' ? 'off' : lab.isTwoSlice ? '2 slices' : lab.gnb.gnbDeployment?.profileName}</> },
          { k: 'UEs running', v: `${ueCounts.running} / ${ueCounts.total}` },
          { k: 'UEs with PDU session', v: withPdu },
        ]} />
        <Actions>
          <Button small onClick={() => navigate('/core-network')}>Core Network</Button>
          <Button small onClick={() => navigate('/free-ran')}>free-ran</Button>
          <Button small onClick={() => navigate('/free-ue')}>free-ue</Button>
        </Actions>
      </Section>
      <Section title="Recent errors">
        {errors.length === 0
          ? <p className={ui.empty} style={{ padding: 0 }}>None.</p>
          : <ul className={ui.list}>{errors.map((e) => <li key={e.id}><Dot status="unhealthy" />{e.ue?.id ?? e.node}: {e.text}</li>)}</ul>}
      </Section>
      <Section><p className={ui.empty} style={{ padding: 0 }}>Select a node on the topology for its details.</p></Section>
    </>
  )

  return (
    <AppLayout crumbs={[{ label: 'Overview' }]} fill>
      <Workbench
        title="Overview"
        status={<><span>core <b>{coreText}</b></span><span>gNB <b>{nodes.gnb.status}</b></span><span>UEs <b>{ueCounts.running}/{ueCounts.total}</b></span><span>PDU sessions <b>{withPdu}</b></span></>}
        console={con}
        selectedId={selected}
        onSelect={setSelected}
        canvas={(
          <LabCanvas
            label="The lab: UE fleet over Uu to the gNB, N2 and N3 into the core, N6 out to the data network."
            {...graph}
            selectedId={selected}
            onSelect={(id) => { setSelected(id); const s = logSourceFor(id); if (s) con.setSource(s) }}
          />
        )}
        inspector={selected ? <NodeInspector id={selected} onLogs={() => { const s = logSourceFor(selected); if (s) { con.setSource(s); con.show('logs') } }} /> : summary}
      />
    </AppLayout>
  )
}
