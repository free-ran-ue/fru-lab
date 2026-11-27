import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import type { CoreNetwork } from '../../api'
import LabCoreTabs from '../../lab/core/LabCoreTabs'
import AppLayout from '../../components/layout/AppLayout'
import { useNotifications } from '../../hooks/useNotifications'
import { extractErrorMessage } from '../../apiClient'
import LabCanvas from '../../lab/canvas/LabCanvas'
import { buildCoreGraph } from '../../lab/canvas/graph'
import { useLab } from '../../lab/LabContext'
import { Actions, Button, Dot, Hint, InspectorHead, KV, Section, Segmented, ui } from '../../lab/LabUi'
import { FREE5GC_TEMPLATE_OPTIONS, type Free5gcTemplate } from '../../lab/free5gcTemplate'
import { templateLabel } from '../../lab/useLab'
import { StatusTag } from '../../lab/StatusTag'
import Workbench from '../../lab/workbench/Workbench'
import { logsHref } from '../../lab/workbench/logSources'
import NodeInspector from '../../lab/workbench/NodeInspector'

const SHORT: Record<Free5gcTemplate, string> = { basic: 'Basic', ulcl: 'ULCL' }

// Free5gcCorePage is one free5GC core network (kind free5gc) - another
// free5GC fru-lab deploys on this host - as the lab free5GC's page is: the
// template pick and Deploy / Stop with its NFs in the inspector, the core
// with its N2 / N3 endpoints, its gNBs and its DN on the canvas, and its
// Subscribers as the second tab. Its node is core:<id>, its NFs
// corenf:<id>:<NF> (their logs stream as core-<id>).
export default function Free5gcCorePage({ core }: { core: CoreNetwork }) {
  const navigate = useNavigate()
  const lab = useLab()
  const { addError, addSuccess } = useNotifications()
  const [picked, setPicked] = useState<Free5gcTemplate>('basic')
  const [selected, setSelected] = useState<string | null>(null)
  const id = core.id ?? ''
  const base = `/core-network/n/${id}`
  const nodeId = `core:${id}`
  const c = lab.free5gcCores.find((x) => x.id === id)
  const status = c?.status ?? 'stopped'
  const stopped = status === 'stopped'
  const blocked = stopped ? undefined : c?.stopBlocked
  const template = c?.template ?? picked
  const nfs = c?.networkFunctions ?? []
  const healthy = c?.healthyNfs ?? 0
  const gnbs = c?.gnbs ?? []

  const graph = buildCoreGraph({
    id: nodeId,
    name: core.name,
    sub: `${templateLabel(template)}${stopped ? ' · stopped' : ''}`,
    status,
    n2Ip: core.n2Ip ?? '',
    n2Interface: core.n2Interface,
    n3Ip: core.n3Ip ?? '',
    n3Interface: core.n3Interface,
    dns: core.dns,
    gnbs: gnbs.map((g) => ({ id: g.id, name: g.profileName, status: g.status })),
  })

  async function toggleCore() {
    if (!c) return
    try {
      if (stopped) {
        await c.deploy(picked)
        addSuccess(`${core.name} ${SHORT[picked]} is starting`)
      } else {
        await c.stop()
        addSuccess(`${core.name} stopped`)
      }
    } catch (error) {
      addError(extractErrorMessage(error, 'Core network action failed'))
    }
  }

  const primary = (
    <Button kind={stopped ? 'primary' : 'danger'} busy={c?.pending} disabled={!c || Boolean(blocked)} title={blocked} onClick={toggleCore}>
      {stopped ? `Deploy ${SHORT[picked]}` : 'Stop core'}
    </Button>
  )

  const coreInspector = (
    <>
      <InspectorHead icon="core" name={core.name} sub={`free5GC on this host${core.slot ? ` · slot ${core.slot}` : ''}`} status={status} />
      <Section title="Deployment">
        {stopped && (
          <div style={{ marginBottom: '0.625rem' }}>
            <Segmented value={picked} options={FREE5GC_TEMPLATE_OPTIONS.map((o) => ({ value: o.value, label: SHORT[o.value] }))} onChange={setPicked} disabled={c?.pending} />
          </div>
        )}
        <KV rows={[
          { k: 'Template', v: `${templateLabel(template)}${stopped ? ' (preview)' : ''}` },
          { k: 'NFs healthy', v: nfs.length ? `${healthy} / ${nfs.length}` : '—' },
          { k: 'Last deployed', v: c?.lastDeployed ?? '—' },
          { k: 'Subscribers', v: <a href={`${base}/subscribers`} onClick={(e) => { e.preventDefault(); navigate(`${base}/subscribers`) }}>open</a> },
        ]} />
        {blocked && <Hint>{blocked}</Hint>}
        <Actions>{primary}{!stopped && <Button small onClick={() => navigate(logsHref(nodeId))}>Logs (all NFs)</Button>}</Actions>
      </Section>
      <Section title="Addresses">
        <KV rows={[
          { k: 'N2 (AMF)', v: [core.n2Ip, core.n2Interface].filter(Boolean).join(' · ') || '—' },
          { k: 'N3 (UPF)', v: [core.n3Ip, core.n3Interface].filter(Boolean).join(' · ') || '—' },
          { k: 'Webconsole port', v: core.webconsolePort ?? '—' },
          { k: 'DN', v: core.dns.map((d) => d.name).join(', ') },
        ]} />
        <Actions><Button small onClick={() => navigate(`/core-network?core=${id}`)}>Edit</Button></Actions>
      </Section>
      {nfs.length > 0 && (
        <Section title="Network functions" aside={<span>{healthy}/{nfs.length}</span>}>
          <ul className={ui.list}>
            {nfs.map((nf) => (
              <li key={nf.name}>
                <button type="button" onClick={() => setSelected(`corenf:${id}:${nf.name}`)}><Dot status={nf.status} />{nf.name}<span style={{ marginLeft: 'auto' }}><StatusTag status={nf.status} /></span></button>
              </li>
            ))}
          </ul>
        </Section>
      )}
      <Section title="gNBs attached" aside={<span>{gnbs.length}</span>}>
        {gnbs.length === 0
          ? <p className={ui.empty} style={{ padding: 0 }}>None. Deploy a gNB onto {core.name} on free-ran.</p>
          : <ul className={ui.list}>{gnbs.map((g) => <li key={g.id}><button type="button" onClick={() => setSelected(g.id)}><Dot status={g.status} />{g.profileName}<span style={{ marginLeft: 'auto' }}>{g.deployment?.n2Ip ?? ''}</span></button></li>)}</ul>}
      </Section>
    </>
  )

  // the core, its N2 / N3 endpoints and its DN all show the core's panel
  const inspector = !selected || selected === nodeId || selected.startsWith(`${nodeId}:`)
    ? coreInspector
    : <NodeInspector id={selected} onLogs={() => navigate(logsHref(selected))} extra={<Section><Button small onClick={() => setSelected(null)}>Back to {core.name}</Button></Section>} />

  return (
    <AppLayout crumbs={[{ label: 'Networks', to: '/core-network' }, { label: core.name }]} fill>
      <Workbench
        title={core.name}
        status={<><LabCoreTabs base={base} label={core.name} /><span>{core.name} <b>{status}</b></span>{nfs.length > 0 && <span>NFs <b>{healthy}/{nfs.length}</b></span>}<span>gNBs <b>{gnbs.length}</b></span></>}
        canvas={(
          <LabCanvas
            label={`The free5GC core network ${core.name} as one node: N2 and N3 from its gNBs, N6 to its data network.`}
            {...graph}
            selectedId={selected}
            onSelect={setSelected}
          />
        )}
        inspector={inspector}
      />
    </AppLayout>
  )
}
