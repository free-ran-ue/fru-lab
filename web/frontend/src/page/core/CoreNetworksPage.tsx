import { useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import AppLayout from '../../components/layout/AppLayout'
import { useConfirm } from '../../components/feedback/Feedback'
import { useNotifications } from '../../hooks/useNotifications'
import { extractErrorMessage } from '../../apiClient'
import type { CoreNetwork } from '../../api'
import CoreNetworkForm, { NEW_CORE } from '../../lab/core/CoreNetworkForm'
import { useLab } from '../../lab/LabContext'
import { Button, KV, Section, ui } from '../../lab/LabUi'
import { isFree5gcCore, templateLabel, type LabCore } from '../../lab/useLab'
import { PickMenu } from '../../lab/GnbPicker'
import { FREE5GC_TEMPLATE_OPTIONS, type Free5gcTemplate } from '../../lab/free5gcTemplate'
import DetailHeader from '../../lab/workbench/DetailHeader'
import ItemList from '../../lab/workbench/ItemList'
import Split from '../../lab/workbench/Split'
import Workbench from '../../lab/workbench/Workbench'

const LAB_ID = 'lab'

// CoreNetworksPage lists every core gNBs can attach to: the lab free5GC,
// the free5GC core networks fru-lab deploys on this host and the cores
// outside the lab saved here. "+ Add network" (and picking
// a saved one) opens its form on the right; each core also has its own
// page under Core Network in the sidebar.
export default function CoreNetworksPage() {
  const navigate = useNavigate()
  const lab = useLab()
  const { networks, save, remove } = lab.coreNetworks
  const { addError, addSuccess } = useNotifications()
  const confirm = useConfirm()
  const [params, setParams] = useSearchParams()
  const [adding, setAdding] = useState(false)

  const picked = params.get('core') ?? LAB_ID
  const selected = adding ? undefined : networks.find((n) => n.id === picked)
  const selId = adding ? null : selected?.id ?? LAB_ID
  const pick = (id: string) => { setAdding(false); setParams({ core: id }, { replace: true }) }
  // the gNBs attached to a core: the lab one's without one
  const gnbsOn = lab.gnbsOnCore
  const free5gcCore = (n?: CoreNetwork): LabCore | undefined => (n && isFree5gcCore(n) ? lab.free5gcCores.find((c) => c.id === n.id) : undefined)

  async function onSave(n: CoreNetwork) {
    try {
      const saved = await save(n)
      addSuccess(`${n.id ? 'Saved' : 'Added'} core network ${saved.name}`)
      if (saved.id) pick(saved.id)
    } catch (error) {
      addError(extractErrorMessage(error, 'Failed to save the core network'))
      throw error
    }
  }

  async function onDelete(n: CoreNetwork) {
    if (!n.id || !await confirm({ title: `Delete core network ${n.name}?`, message: 'Its page goes too. gNB profiles pointing at its addresses keep them.', confirmLabel: 'Delete', danger: true })) return
    try {
      await remove(n.id)
      setParams({}, { replace: true })
      addSuccess(`Deleted core network ${n.name}`)
    } catch (error) {
      addError(extractErrorMessage(error, 'Failed to delete the core network'))
    }
  }

  // free5GC: Deploy (pick a template) or Stop
  async function deployLab(t: string) {
    try {
      await lab.free5gc.deploy(t as Free5gcTemplate)
      addSuccess(`free5GC ${templateLabel(t as Free5gcTemplate)} is starting`)
    } catch (error) {
      addError(extractErrorMessage(error, 'free5GC deploy failed'))
    }
  }
  async function stopLab() {
    try {
      await lab.free5gc.stop()
      addSuccess('free5GC stopped')
    } catch (error) {
      addError(extractErrorMessage(error, 'Failed to stop free5GC'))
    }
  }
  // a free5GC core network: Deploy (pick a template) or Stop, like free5GC
  async function deployCore(c: LabCore, t: string) {
    try {
      await c.deploy(t as Free5gcTemplate)
      addSuccess(`${c.name} ${templateLabel(t as Free5gcTemplate)} is starting`)
    } catch (error) {
      addError(extractErrorMessage(error, `${c.name} deploy failed`))
    }
  }
  async function stopCore(c: LabCore) {
    try {
      await c.stop()
      addSuccess(`${c.name} stopped`)
    } catch (error) {
      addError(extractErrorMessage(error, `Failed to stop ${c.name}`))
    }
  }
  const templateItems = FREE5GC_TEMPLATE_OPTIONS.map((o) => ({ value: o.value, label: o.label }))
  const coreAction = (c: LabCore) => (c.status === 'stopped'
    ? <PickMenu label="Deploy" title={`${c.name} template`} items={templateItems} busy={c.pending} onPick={(t) => deployCore(c, t)} />
    : <Button kind="danger" small busy={c.pending} disabled={Boolean(c.stopBlocked)} title={c.stopBlocked} onClick={() => stopCore(c)}>Stop</Button>)

  // the cores outside the lab: Enable puts the core in the lab (on the Overview), Disable
  // takes it out
  const [toggling, setToggling] = useState<string | null>(null)
  async function setEnabled(n: CoreNetwork, enabled: boolean) {
    setToggling(n.id ?? null)
    try {
      await save({ ...n, enabled })
      addSuccess(`${enabled ? 'Enabled' : 'Disabled'} core network ${n.name}`)
    } catch (error) {
      addError(extractErrorMessage(error, `Failed to ${enabled ? 'enable' : 'disable'} ${n.name}`))
    } finally {
      setToggling(null)
    }
  }
  const labAction = lab.nodes.core.status === 'stopped'
    ? <PickMenu label="Deploy" title="free5GC template" items={templateItems} busy={lab.free5gc.isActionPending} onPick={deployLab} />
    : <Button kind="danger" small busy={lab.free5gc.isActionPending} disabled={Boolean(lab.blocked.coreStop)} title={lab.blocked.coreStop} onClick={stopLab}>Stop</Button>
  const extAction = (n: CoreNetwork) => (n.enabled
    ? <Button kind="danger" small busy={toggling === n.id} onClick={() => setEnabled(n, false)}>Disable</Button>
    : <Button kind="primary" small busy={toggling === n.id} onClick={() => setEnabled(n, true)}>Enable</Button>)

  const coreStatus = lab.nodes.core.status
  const list = (
    <ItemList
      label="Core networks"
      columns={['Name', 'N2 (AMF)', 'N3 (UPF)', 'DNs', 'gNBs']}
      template="minmax(0, 1.2fr) minmax(0, 1fr) minmax(0, 1fr) minmax(0, 1fr) 3rem"
      rows={[
        { id: LAB_ID, status: coreStatus, cells: [lab.labCore.name, lab.labCore.n2Ip, lab.labCore.n3Ip, lab.labCore.dns.map((d) => d.name).join(', '), gnbsOn().length], action: labAction },
        ...networks.map((n) => {
          const c = free5gcCore(n)
          return {
            id: n.id ?? '',
            status: c ? c.status : n.enabled ? 'running' as const : 'stopped' as const,
            cells: [n.name, n.n2Ip || '—', n.n3Ip || '—', n.dns.map((d) => d.name).join(', '), gnbsOn(n).length],
            action: c ? coreAction(c) : extAction(n),
          }
        }),
      ]}
      selectedId={selId}
      onSelect={pick}
      defaultSort={{ key: '0', dir: 'asc' }}
      head={<Button kind="primary" small onClick={() => setAdding(true)}>+ Add network</Button>}
      empty="No core networks."
    />
  )

  let detail
  if (adding) {
    detail = (
      <>
        <DetailHeader icon="core" name="New core network" sub="a free5GC on this host, or a core outside the lab" actions={<Button small onClick={() => setAdding(false)}>Cancel</Button>} />
        <CoreNetworkForm value={NEW_CORE} takenNames={networks.map((n) => n.name)} onSave={onSave} />
      </>
    )
  } else if (selected && free5gcCore(selected)) {
    const c = free5gcCore(selected)!
    const up = c.status !== 'stopped'
    detail = (
      <>
        <DetailHeader
          icon="core"
          name={selected.name}
          sub={`free5GC on this host${selected.slot ? ` · slot ${selected.slot}` : ''} · ${c.template ? templateLabel(c.template) : 'not running'} · ${c.gnbs.length} gNB attached`}
          status={c.status}
          actions={(
            <>
              {coreAction(c)}
              <Button small onClick={() => navigate(`/core-network/n/${selected.id}`)}>Open page</Button>
              <Button kind="danger" small disabled={up} title={up ? 'Stop it first' : undefined} onClick={() => onDelete(selected)}>Delete</Button>
            </>
          )}
        />
        <CoreNetworkForm key={selected.id} value={selected} takenNames={networks.filter((n) => n.id !== selected.id).map((n) => n.name)} onSave={onSave} />
      </>
    )
  } else if (selected) {
    detail = (
      <>
        <DetailHeader
          icon="core"
          name={selected.name}
          sub={`outside the lab · ${selected.enabled ? 'enabled' : 'disabled'} · ${gnbsOn(selected).length} gNB attached`}
          status={selected.enabled ? 'running' : 'stopped'}
          statusText={selected.enabled ? 'ENABLED' : 'DISABLED'}
          actions={(
            <>
              {extAction(selected)}
              <Button small onClick={() => navigate(`/core-network/n/${selected.id}`)}>Open page</Button>
              <Button kind="danger" small disabled={Boolean(selected.enabled)} title={selected.enabled ? 'Disable it first' : undefined} onClick={() => onDelete(selected)}>Delete</Button>
            </>
          )}
        />
        <CoreNetworkForm key={selected.id} value={selected} takenNames={networks.filter((n) => n.id !== selected.id).map((n) => n.name)} onSave={onSave} />
      </>
    )
  } else {
    detail = (
      <>
        <DetailHeader
          icon="core"
          name={lab.labCore.name}
          sub={lab.runningTemplate ? `${templateLabel(lab.runningTemplate)} · deployed by fru-lab` : 'deployed by fru-lab · not running'}
          status={coreStatus}
          actions={<>{labAction}<Button small onClick={() => navigate('/core-network/lab')}>Open page</Button></>}
        />
        <Section title="Addresses">
          <KV rows={[
            { k: 'N2 (AMF)', v: `${lab.labCore.n2Ip} · ${lab.labCore.n2Interface}` },
            { k: 'N3 (UPF)', v: `${lab.labCore.n3Ip} · ${lab.labCore.n3Interface}` },
            { k: 'DN', v: lab.labCore.dns.map((d) => d.name).join(', ') },
          ]} />
          <p className={ui.empty} style={{ padding: '0.5rem 0 0' }}>The lab core's addresses are fixed. Deploy or stop it, and pick its template, on its page.</p>
        </Section>
      </>
    )
  }

  return (
    <AppLayout crumbs={[{ label: 'Networks' }]} fill>
      <Workbench
        title="Core networks"
        status={<><span>cores <b>{networks.length + 1}</b></span><span>free5GC <b>{coreStatus}</b></span>{lab.free5gcCores.length > 0 && <span>free5GC cores up <b>{lab.upCores.length}/{lab.free5gcCores.length}</b></span>}<span>external enabled <b>{networks.filter((n) => !isFree5gcCore(n) && n.enabled).length}</b></span></>}
        body={<Split storageKey="frulab-split-cores" defaultRatio={0.6} left={list} right={detail} leftLabel="Core network list" rightLabel="Core network detail" />}
      />
    </AppLayout>
  )
}
