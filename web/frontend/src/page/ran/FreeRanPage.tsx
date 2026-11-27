import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import AppLayout from '../../components/layout/AppLayout'
import { useNotifications } from '../../hooks/useNotifications'
import { useConfirm } from '../../components/feedback/Feedback'
import { extractErrorMessage } from '../../apiClient'
import type { GnbProfile } from '../../api'
import { ueGnbId } from '../../lab/canvas/graph'
import { useEvents } from '../../lab/console/EventsContext'
import { STAGE_LABEL } from '../../lab/console/events'
import { useLab } from '../../lab/LabContext'
import { Button, Dot, Hint, KV, Section, ui } from '../../lab/LabUi'
import GnbProfilesPanel from '../../lab/profiles/GnbProfilesPanel'
import { BulkCreate } from '../../lab/profiles/parts'
import type { LabGnb } from '../../lab/useLab'
import { useStoredState } from '../../lab/useProfiles'
import { CoreMenu } from '../../lab/GnbPicker'
import ListToolbar from '../../lab/workbench/ListToolbar'
import { compareValues } from '../../lab/sort'
import Workbench from '../../lab/workbench/Workbench'
import { logsHref } from '../../lab/workbench/logSources'
import DetailHeader from '../../lab/workbench/DetailHeader'
import ItemList from '../../lab/workbench/ItemList'
import Split from '../../lab/workbench/Split'

type Tab = 'status' | 'config'
// what the detail pane shows: a saved profile, or a new one (blank, or a
// copy of a saved one)
type View = { kind: 'profile'; id: string } | { kind: 'new'; from?: string }

const addrOf = (p: GnbProfile) => (p.core === 'external' ? p.hostIp : p.n2Ip) || '—'

// FreeRanPage is everything about gNBs on one page. Every gNB profile is
// one gNB and one row. Actions sit in two fixed places only:
//   list toolbar   the shared ListToolbar: New, Bulk create, find,
//                  All / Running; Deploy / Delete selected; Start all
//                  stopped (onto a picked core), Stop all
//   detail header  Deploy / Stop, Logs, Duplicate, Delete (one gNB)
// and each row ends with its Deploy / Stop. The detail pane has a Status
// tab (the running gNB) and a Config tab (the profile, Save at its top).
export default function FreeRanPage() {
  const navigate = useNavigate()
  const lab = useLab()
  const { ueLive, events } = useEvents()
  const { gnbInstances, gnbProfiles } = lab
  const { addError, addSuccess } = useNotifications()
  const confirm = useConfirm()
  const [storedId, setStoredId] = useStoredState('frulab-gnb-profile', '')
  const [view, setView] = useState<View | null>(null)
  const [tab, setTab] = useState<Tab>('status')
  const [find, setFind] = useState('')
  const [show, setShow] = useStoredState<'all' | 'running'>('frulab-gnb-show', 'all')
  // the rows ticked for Delete selected
  const [ticked, setTicked] = useState<Set<string>>(new Set())
  const [deleting, setDeleting] = useState(false)

  const profiles = gnbProfiles.profiles
  // the core a profile deploys onto by default
  const savedCore = (p: GnbProfile) => (p.coreNetwork ? lab.coreNetworks.networks.find((n) => n.id === p.coreNetwork) : undefined)
  const coreOf = (p: GnbProfile) => (p.core !== 'external' ? 'free5GC' : p.coreNetwork ? (savedCore(p)?.name ?? 'a deleted core') : `external ${p.amfIp ?? ''}`)
  const runningOf = (id?: string) => lab.gnbs.find((g) => g.profileId === id)
  // a UE that failed to register is up but not connected
  const connected = (id: string) => lab.upUes.filter((r) => ueGnbId(lab, r) === id && ueLive[r.ueId]?.stage !== 'failed')

  // the selection: what was picked, else the remembered profile, else the first
  let current: View | null = null
  if (view?.kind === 'new') current = view
  else {
    const p = profiles.find((x) => x.id === (view?.id ?? storedId)) ?? profiles[0]
    if (p?.id) current = { kind: 'profile', id: p.id }
  }
  const profile = current?.kind === 'profile' ? profiles.find((p) => p.id === current.id) : undefined
  const running = runningOf(profile?.id)
  const shownTab: Tab = current?.kind === 'new' ? 'config' : tab

  function pick(id: string) {
    setView({ kind: 'profile', id })
    setStoredId(id)
  }

  // why deploying `p` onto its own core cannot happen now, if it cannot
  const deployBlocked = (p: GnbProfile) => {
    if (p.core !== 'external') return lab.blocked.gnbDeploy
    if (!p.coreNetwork) return undefined
    const n = savedCore(p)
    return !n ? 'Its core network was deleted' : !n.enabled ? `Core network ${n.name} is disabled` : undefined
  }

  // core: '' keeps the profile's, 'lab' is free5GC, else a core network id
  const coreName = (core: string) => (core === 'lab' ? 'free5GC' : lab.coreNetworks.networks.find((n) => n.id === core)?.name)
  async function deploy(p: GnbProfile, core = '') {
    if (!p.id) return
    try {
      await gnbInstances.deploy(p.id, core)
      addSuccess(`gNB ${p.name} is starting${core ? ` on ${coreName(core)}` : ''}`)
      pick(p.id)
      setTab('status')
    } catch (error) {
      addError(extractErrorMessage(error, 'gNB deploy failed'))
    }
  }

  async function stop(g: LabGnb) {
    try {
      await g.stop()
      addSuccess(`${g.name} stopped`)
    } catch (error) {
      addError(extractErrorMessage(error, `Failed to stop ${g.name}`))
    }
  }

  async function remove(p: GnbProfile) {
    if (!p.id || !await confirm({ title: `Delete gNB profile ${p.name}?`, message: 'The profile is removed for good. Its gNB is not running.', confirmLabel: 'Delete', danger: true })) return
    try {
      await gnbProfiles.remove(p.id)
      setView(null)
      addSuccess(`Deleted ${p.name}`)
    } catch (error) {
      addError(extractErrorMessage(error, 'Failed to delete the profile'))
    }
  }

  // Deploy selected / Start all stopped: those profiles, one after
  // another, onto `core`
  const [deployingMany, setDeployingMany] = useState<string | null>(null)
  const [stoppingAll, setStoppingAll] = useState(false)
  async function deployTicked(core: string) {
    await deployMany(profiles.filter((p) => p.id && ticked.has(p.id) && !runningOf(p.id)), core)
  }
  async function deployMany(targets: GnbProfile[], core: string) {
    let failed = 0
    for (const [i, p] of targets.entries()) {
      setDeployingMany(`Deploying ${i + 1}/${targets.length}…`)
      try { await gnbInstances.deploy(p.id!, core) } catch (error) { failed += 1; addError(extractErrorMessage(error, `gNB ${p.name} failed to deploy`)) }
    }
    setDeployingMany(null)
    setTicked(new Set())
    if (targets.length > failed) addSuccess(`Started ${targets.length - failed} of ${targets.length} gNB${targets.length === 1 ? '' : 's'}${core ? ` on ${coreName(core)}` : ''}`)
  }

  // Stop all: every running gNB (each UE on them must be stopped first)
  async function stopAllGnbs() {
    setStoppingAll(true)
    let failed = 0
    for (const g of lab.gnbs) {
      try { await g.stop() } catch (error) { failed += 1; addError(extractErrorMessage(error, `Failed to stop ${g.name}`)) }
    }
    setStoppingAll(false)
    if (lab.gnbs.length > failed) addSuccess(`Stopped ${lab.gnbs.length - failed} gNB${lab.gnbs.length - failed === 1 ? '' : 's'}`)
  }

  // Delete selected: the ticked profiles (running ones can't be ticked)
  async function removeTicked() {
    const targets = profiles.filter((p) => p.id && ticked.has(p.id) && !runningOf(p.id))
    if (targets.length === 0 || !await confirm({ title: `Delete ${targets.length} gNB profile${targets.length === 1 ? '' : 's'}?`, message: <ul>{targets.slice(0, 10).map((p) => <li key={p.id}>{p.name}</li>)}{targets.length > 10 && <li>… {targets.length - 10} more</li>}</ul>, confirmLabel: 'Delete', danger: true })) return
    setDeleting(true)
    let failed = 0
    for (const p of targets) {
      try { await gnbProfiles.remove(p.id!) } catch { failed += 1 }
    }
    setDeleting(false)
    setTicked(new Set())
    setView(null)
    if (failed) addError(`${failed} of ${targets.length} profiles could not be deleted`)
    else addSuccess(`Deleted ${targets.length} gNB profile${targets.length === 1 ? '' : 's'}`)
  }

  async function bulk(count: number) {
    if (!profile?.id) return
    try {
      const created = await gnbProfiles.bulk(profile.id, count)
      addSuccess(`Created ${created.length} gNB profile${created.length === 1 ? '' : 's'}`)
    } catch (error) {
      addError(extractErrorMessage(error, 'Bulk create failed'))
    }
  }

  // Deploy or Stop for one profile: the row's end and the detail header
  function toggle(p: GnbProfile) {
    const g = runningOf(p.id)
    if (g) {
      const blocked = lab.gnbStopBlocked(g.id)
      return <Button kind="danger" small busy={g.pending} disabled={Boolean(blocked)} title={blocked} onClick={() => stop(g)}>Stop</Button>
    }
    // Deploy opens the cores to deploy onto, the profile's own first
    return <CoreMenu busy={Boolean(p.id && gnbInstances.pending[p.id])} own={{ text: coreOf(p), blocked: deployBlocked(p) }} onPick={(core) => deploy(p, core)} />
  }

  // top to bottom by profile name, A to Z, until a header is clicked
  const shown = [...profiles].sort((a, b) => compareValues(a.name, b.name, 'asc')).filter((p) => (show === 'all' || runningOf(p.id)) && (!find || `${p.name} ${p.n2Ip ?? ''} ${p.hostIp ?? ''} ${p.gnbId}`.toLowerCase().includes(find.toLowerCase())))

  const list = (
    <ItemList
      label="gNBs"
      columns={['Profile', 'Core', 'N2 / host IP', 'UEs']}
      template="minmax(0, 1.2fr) minmax(0, 1fr) minmax(0, 1fr) 2.5rem"
      rows={shown.map((p) => {
        const g = runningOf(p.id)
        return { id: p.id ?? '', status: g?.status ?? 'stopped', cells: [p.name, coreOf(p), addrOf(p), g ? connected(g.id).length : '—'], action: toggle(p), checkable: g ? 'Stop its gNB first' : true }
      })}
      selectedId={current?.kind === 'profile' ? current.id : null}
      onSelect={pick}
      checked={ticked}
      onChecked={setTicked}
      defaultSort={{ key: '0', dir: 'asc' }}
      head={(
        <ListToolbar
          onNew={() => setView({ kind: 'new' })}
          bulk={<BulkCreate disabled={!profile} title={profile ? `Copy ${profile.name}; gNB ID and N2 IP count up to the next free value` : 'Select a profile to copy'} onCreate={bulk} />}
          find={find}
          onFind={setFind}
          findLabel="Find a gNB"
          show={show}
          onShow={setShow}
          counts={{ all: profiles.length, running: lab.gnbs.length }}
          ticked={ticked.size}
          deploySelected={<Button kind="primary" small busy={Boolean(deployingMany)} title="Each onto its profile's core" onClick={() => deployTicked('')}>{deployingMany ?? `Deploy selected (${ticked.size})`}</Button>}
          onDeleteSelected={removeTicked}
          deleting={deleting}
          startAll={<Button kind="primary" small busy={Boolean(deployingMany)} disabled={profiles.every((p) => !p.id || runningOf(p.id))} title="Each onto its profile's core" onClick={() => deployMany(profiles.filter((p) => p.id && !runningOf(p.id)), '')}>Start all stopped</Button>}
          onStopAll={stopAllGnbs}
          stopAll={{ disabled: lab.gnbs.length === 0 || lab.ueUp, busy: stoppingAll, title: lab.ueUp ? 'Stop the UEs first (free-ue)' : undefined }}
        />
      )}
      empty={profiles.length ? 'Nothing matches.' : 'No gNB profiles yet. Press + New: every profile is one gNB.'}
    />
  )

  // the Status tab: what the gNB will deploy with, or how it runs
  function statusOf(p: GnbProfile, g?: LabGnb) {
    if (!g) {
      const blocked = deployBlocked(p)
      return (
        <Section title="Will deploy with">
          <KV rows={[
            { k: 'Core', v: coreOf(p) },
            { k: 'gNB ID', v: p.gnbId },
            { k: p.core === 'external' ? 'Host IP (N2/N3)' : 'N2 IP', v: addrOf(p) },
            ...(p.core === 'external' && p.hostInterface ? [{ k: 'Host interface', v: p.hostInterface }] : []),
            { k: 'RAN IP', v: 'picked on deploy' },
            { k: 'PLMN', v: `${p.mcc}${p.mnc}` },
            { k: 'TAC', v: p.tac },
            { k: 'SD', v: p.sd || '—' },
          ]} />
          {blocked && <Hint>{blocked}</Hint>}
        </Section>
      )
    }
    const d = g.deployment
    const ues = connected(g.id)
    const ngSetup = [...events].reverse().find((e) => e.node === g.id && /NG Setup complete|Cannot reach the AMF/.test(e.text))
    const stopBlocked = lab.gnbStopBlocked(g.id)
    return (
      <>
        <Section title="Running">
          <KV rows={[
            { k: 'gNB ID', v: d?.gnbId || '—' },
            { k: g.core === 'external' ? 'Host IP (N2/N3)' : 'N2 IP', v: d?.n2Ip || d?.hostIp || '—' },
            ...(g.core === 'external' && d?.hostInterface ? [{ k: 'Host interface', v: d.hostInterface }] : []),
            { k: 'RAN IP (auto)', v: d?.ranIp || '—' },
            { k: 'SD', v: g.sd || '—' },
            { k: 'NG Setup', v: ngSetup ? <><Dot status={ngSetup.level === 'error' ? 'unhealthy' : 'running'} /> {ngSetup.level === 'error' ? 'failed' : 'done'}</> : 'waiting' },
            { k: 'Last deployed', v: g.lastDeployed },
          ]} />
          {stopBlocked && <Hint>{stopBlocked}</Hint>}
        </Section>
        <Section title="Connected UEs" aside={<span>{ues.length}</span>}>
          {ues.length === 0
            ? <p className={ui.empty} style={{ padding: 0 }}>No UE is connected.</p>
            : (
              <ul className={ui.list}>
                {ues.map((r) => {
                  const l = ueLive[r.ueId]
                  return (
                    <li key={r.ueId}>
                      <button type="button" onClick={() => navigate(`/free-ue?ue=${encodeURIComponent(r.ueId)}`)}>
                        <Dot status={l?.stage === 'failed' ? 'unhealthy' : r.status} />{r.ueId.replace(/^imsi-/, '')}
                        <span style={{ marginLeft: 'auto' }}>{l?.ip ?? (l?.stage ? STAGE_LABEL[l.stage] : r.status)}</span>
                      </button>
                    </li>
                  )
                })}
              </ul>
            )}
        </Section>
        {g.networkFunctions.length > 0 && (
          <Section title="Containers">
            <ul className={ui.list}>{g.networkFunctions.map((s) => <li key={s.name}><Dot status={s.status} />{s.name}<span>{s.status}</span></li>)}</ul>
          </Section>
        )}
      </>
    )
  }

  // ----- detail -----
  const notify = (ok: boolean, msg: string) => (ok ? addSuccess(msg) : addError(msg))
  let detail = <p className={ui.empty}>Press + New to create the first gNB profile.</p>
  if (current?.kind === 'new') {
    const from = current.from ? profiles.find((p) => p.id === current.from) : undefined
    detail = (
      <>
        <DetailHeader icon="gnb" name="New gNB profile" sub={from ? `copy of ${from.name}` : 'from the lab defaults'} actions={<Button small onClick={() => setView(current.from ? { kind: 'profile', id: current.from } : null)}>Cancel</Button>} />
        <GnbProfilesPanel
          key={`new-${current.from ?? ''}`}
          profiles={gnbProfiles}
          selectedId=""
          copyOf={from}
          onSelect={(id) => { if (id) { pick(id); setTab('config') } }}
          onNotify={notify}
        />
      </>
    )
  } else if (profile) {
    detail = (
      <>
        <DetailHeader
          icon="gnb"
          name={profile.name}
          sub={`${running ? running.name : 'not deployed'} · ${coreOf(profile)}`}
          status={running?.status ?? 'stopped'}
          actions={(
            <>
              {toggle(profile)}
              <Button small disabled={!running} title={running ? undefined : 'Not running'} onClick={() => { if (running) navigate(logsHref(running.id)) }}>Logs</Button>
              <Button small onClick={() => setView({ kind: 'new', from: profile.id })}>Duplicate</Button>
              <Button kind="danger" small disabled={Boolean(running)} title={running ? 'Stop its gNB first' : undefined} onClick={() => remove(profile)}>Delete</Button>
            </>
          )}
          tab={shownTab}
          tabs={[{ value: 'status', label: 'Status' }, { value: 'config', label: 'Config' }]}
          onTab={setTab}
        />
        {shownTab === 'config'
          ? <GnbProfilesPanel key={profile.id} profiles={gnbProfiles} selectedId={profile.id ?? ''} running={running} onSelect={(id) => { if (id) pick(id) }} onNotify={notify} />
          : statusOf(profile, running)}
      </>
    )
  }

  const externalAmfs = [...new Set(lab.externalGnbs.map((g) => g.deployment?.amfIp).filter(Boolean))]

  return (
    <AppLayout crumbs={[{ label: 'gNBs' }]} fill>
      <Workbench
        title="gNBs"
        status={<><span>profiles <b>{profiles.length}</b></span><span>running <b>{lab.gnbs.length}</b></span><span>lab core <b>{lab.nodes.core.status}</b></span>{externalAmfs.length > 0 && <span>external <b>{externalAmfs.join(', ')}</b></span>}<span>UEs connected <b>{lab.gnbs.reduce((n, g) => n + connected(g.id).length, 0)}</b></span></>}
        body={<Split storageKey="frulab-split-gnb-6040" defaultRatio={0.6} left={list} right={detail} leftLabel="gNB list" rightLabel="gNB detail" />}
      />
    </AppLayout>
  )
}
