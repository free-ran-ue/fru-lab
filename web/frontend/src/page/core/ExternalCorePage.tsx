import { useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import AppLayout from '../../components/layout/AppLayout'
import LabCanvas from '../../lab/canvas/LabCanvas'
import { buildCoreGraph } from '../../lab/canvas/graph'
import { useLab } from '../../lab/LabContext'
import { Actions, Button, Dot, InspectorHead, KV, Section, ui } from '../../lab/LabUi'
import Workbench from '../../lab/workbench/Workbench'

// ExternalCorePage is one saved core outside the lab: the core as one node
// with its N2 / N3 endpoints, the gNBs attached to them (running external
// gNBs dialing its AMF) and its data networks.
export default function ExternalCorePage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const lab = useLab()
  const [selected, setSelected] = useState<string | null>(null)
  const core = lab.coreNetworks.networks.find((n) => n.id === id)

  if (!core) {
    return (
      <AppLayout crumbs={[{ label: 'Not found' }]}>
        <p className={ui.empty}>{lab.coreNetworks.loaded ? 'No such core network. It may have been deleted.' : 'Loading…'}</p>
      </AppLayout>
    )
  }

  const gnbs = lab.gnbs.filter((g) => g.core === 'external' && g.deployment?.amfIp === core.n2Ip)
  const graph = buildCoreGraph({
    id: `core:${core.id}`,
    name: core.name,
    sub: 'outside the lab',
    n2Ip: core.n2Ip,
    n2Interface: core.n2Interface,
    n3Ip: core.n3Ip,
    n3Interface: core.n3Interface,
    dns: core.dns,
    gnbs: gnbs.map((g) => ({ id: g.id, name: g.profileName, status: g.status })),
  })

  const inspector = (
    <>
      <InspectorHead icon="core" name={core.name} sub="core network outside the lab" />
      <Section title="Addresses">
        <KV rows={[
          { k: 'N2 (AMF)', v: core.n2Interface ? `${core.n2Ip} · ${core.n2Interface}` : core.n2Ip },
          { k: 'N3 (UPF)', v: core.n3Interface ? `${core.n3Ip} · ${core.n3Interface}` : core.n3Ip },
        ]} />
        <Actions><Button small onClick={() => navigate(`/core-network?core=${core.id}`)}>Edit</Button></Actions>
      </Section>
      <Section title="Data networks" aside={<span>{core.dns.length}</span>}>
        <ul className={ui.list}>{core.dns.map((d) => <li key={d.name}><Dot status="stopped" />{d.name}</li>)}</ul>
      </Section>
      <Section title="gNBs attached" aside={<span>{gnbs.length}</span>}>
        {gnbs.length === 0
          ? <p className={ui.empty} style={{ padding: 0 }}>None. A gNB profile on an external core with AMF {core.n2Ip} attaches here once deployed.</p>
          : <ul className={ui.list}>{gnbs.map((g) => <li key={g.id}><button type="button" onClick={() => navigate('/free-ran')}><Dot status={g.status} />{g.profileName}<span style={{ marginLeft: 'auto' }}>{g.deployment?.hostIp ?? ''}</span></button></li>)}</ul>}
      </Section>
    </>
  )

  return (
    <AppLayout crumbs={[{ label: core.name }]} fill>
      <Workbench
        title={core.name}
        status={<><span>N2 <b>{core.n2Ip}</b></span><span>N3 <b>{core.n3Ip}</b></span><span>DNs <b>{core.dns.length}</b></span><span>gNBs <b>{gnbs.length}</b></span></>}
        canvas={<LabCanvas label={`The core ${core.name}: N2 and N3 from its gNBs, N6 to its data networks.`} {...graph} selectedId={selected} onSelect={setSelected} />}
        inspector={inspector}
      />
    </AppLayout>
  )
}
