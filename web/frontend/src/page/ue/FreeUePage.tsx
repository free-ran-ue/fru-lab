import { useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import AppLayout from '../../components/layout/AppLayout'
import { useNotifications } from '../../hooks/useNotifications'
import { useConfirm } from '../../components/feedback/Feedback'
import { extractErrorMessage } from '../../apiClient'
import { ID, ueGnbId } from '../../lab/canvas/graph'
import { useEvents } from '../../lab/console/EventsContext'
import { STAGE_LABEL } from '../../lab/console/events'
import { DeployMenu } from '../../lab/GnbPicker'
import ListToolbar from '../../lab/workbench/ListToolbar'
import { useLab } from '../../lab/LabContext'
import { Button, Hint, KV, Section, ui } from '../../lab/LabUi'
import UeProfilesPanel from '../../lab/profiles/UeProfilesPanel'
import { BulkCreate } from '../../lab/profiles/parts'
import type { UeRow } from '../../lab/useUeInstances'
import { useStoredState } from '../../lab/useProfiles'
import { compareValues } from '../../lab/sort'
import Workbench from '../../lab/workbench/Workbench'
import { logsHref } from '../../lab/workbench/logSources'
import DetailHeader from '../../lab/workbench/DetailHeader'
import ItemList from '../../lab/workbench/ItemList'
import Split from '../../lab/workbench/Split'

type Tab = 'status' | 'config'

// FreeUePage is everything about UEs on one page. Every UE profile is one
// UE and one row (plus any UE still deployed without a profile). Actions
// sit in two fixed places only:
//   list toolbar   the shared ListToolbar: New, Bulk create, find,
//                  All / Running; Deploy / Delete selected; Start all
//                  stopped (onto a picked gNB), Stop all
//   detail header  Deploy / Stop, Terminal, Logs, Duplicate, Delete
// and each row ends with its Deploy / Stop. Every deploy goes onto the gNB
// picked in the list toolbar. The detail pane has a Status tab (the
// session) and a Config tab (the profile, Save at its top).
export default function FreeUePage() {
  const navigate = useNavigate()
  const lab = useLab()
  const { ueLive } = useEvents()
  const { ue, ueCounts, ueProfiles } = lab
  const { addError, addSuccess } = useNotifications()
  const confirm = useConfirm()
  const [params, setParams] = useSearchParams()
  const [show, setShow] = useStoredState<'running' | 'all'>('frulab-ue-show', 'all')
  const [find, setFind] = useState('')
  // the rows ticked for Delete selected
  const [ticked, setTicked] = useState<Set<string>>(new Set())
  const [deleting, setDeleting] = useState(false)
  const [tab, setTab] = useState<Tab>('status')
  // a new profile being written: blank, or a copy of this profile id
  const [creating, setCreating] = useState<{ from?: string } | null>(null)

  const blocked = lab.blocked.ueDeploy
  const ready = ue.rows.filter((r) => r.status === 'running' && ueLive[r.ueId]?.ip).length
  const stoppedIds = ue.rows.filter((r) => r.status === 'stopped' && r.source !== 'other').map((r) => r.ueId)
  const anyPending = lab.upUes.some((r) => ue.pendingInstances[r.ueId])

  // the gNB a UE is on: the one recorded when it was deployed
  const gnbName = (r: UeRow) => {
    const id = ueGnbId(lab, r)
    return lab.gnbs.find((x) => x.id === id)?.profileName ?? (id || '—')
  }
  const profileOf = (r?: UeRow) => (r?.profileId ? ueProfiles.profiles.find((p) => p.id === r.profileId) : undefined)

  const pool = show === 'running' ? lab.upUes : ue.rows
  // top to bottom by profile name, A to Z (UEs with no profile last),
  // until a header is clicked
  const rows = pool
    .filter((r) => !find || `${r.ueId} ${r.profileName ?? ''}`.toLowerCase().includes(find.toLowerCase()))
    .sort((a, b) => compareValues(a.profileName, b.profileName, 'asc') || compareValues(a.ueId, b.ueId, 'asc'))
  const selectedId = params.get('ue')
  const row = creating ? undefined : (ue.rows.find((r) => r.ueId === selectedId) ?? rows[0])
  const profile = profileOf(row)
  const live = row && row.status !== 'stopped' ? ueLive[row.ueId] : undefined
  const shownTab: Tab = row?.source === 'other' ? 'status' : tab

  function pick(ueId: string) {
    setCreating(null)
    setParams({ ue: ueId }, { replace: true })
  }

  // gnb '' lets the backend pick a running gNB by SD
  async function deploy(ueId: string, gnb: string) {
    try {
      await ue.deploy(ueId, gnb || undefined)
      addSuccess(`${ueId} is starting`)
    } catch (error) {
      addError(extractErrorMessage(error, `Failed to deploy ${ueId}`))
    }
  }

  async function stop(ueId: string) {
    try {
      await ue.stop(ueId)
      addSuccess(`${ueId} stopped`)
    } catch (error) {
      addError(extractErrorMessage(error, `Failed to stop ${ueId}`))
    }
  }

  async function stopAll() {
    try {
      await ue.stopAll()
      addSuccess('All UEs stopped')
    } catch (error) {
      addError(extractErrorMessage(error, 'Failed to stop all UEs'))
    }
  }

  async function launchStopped(gnb: string) {
    const failedIds = await ue.deployMany(stoppedIds, gnb || undefined)
    if (failedIds.length) addError(`${failedIds.length} of ${stoppedIds.length} UEs failed to start`)
    else addSuccess(`Started ${stoppedIds.length} UE${stoppedIds.length === 1 ? '' : 's'}`)
  }

  async function remove(r: UeRow) {
    const p = profileOf(r)
    if (!p?.id || !await confirm({ title: `Delete UE profile ${p.name}?`, message: 'The profile is removed for good. Its UE is not running.', confirmLabel: 'Delete', danger: true })) return
    try {
      await ueProfiles.remove(p.id)
      setParams({}, { replace: true })
      addSuccess(`Deleted ${p.name}`)
    } catch (error) {
      addError(extractErrorMessage(error, 'Failed to delete the profile'))
    }
  }

  // Deploy selected: the ticked UEs, one after another, onto `gnb`
  const [deployingMany, setDeployingMany] = useState<string | null>(null)
  async function deployTicked(gnb: string) {
    const ids = ue.rows.filter((r) => ticked.has(r.ueId) && r.status === 'stopped' && r.source !== 'other').map((r) => r.ueId)
    setDeployingMany(`Deploying 0/${ids.length}…`)
    const failedIds = await ue.deployMany(ids, gnb || undefined, (done) => setDeployingMany(`Deploying ${done}/${ids.length}…`))
    setDeployingMany(null)
    setTicked(new Set())
    if (failedIds.length) addError(`${failedIds.length} of ${ids.length} UEs failed to start`)
    else addSuccess(`Started ${ids.length} UE${ids.length === 1 ? '' : 's'}`)
  }

  // Delete selected: the ticked UEs' profiles (running ones can't be ticked)
  async function removeTicked() {
    const targets = ue.rows.filter((r) => ticked.has(r.ueId) && r.status === 'stopped').map(profileOf).filter((p) => p?.id)
    if (targets.length === 0 || !await confirm({ title: `Delete ${targets.length} UE profile${targets.length === 1 ? '' : 's'}?`, message: <ul>{targets.slice(0, 10).map((p) => <li key={p!.id}>{p!.name}</li>)}{targets.length > 10 && <li>… {targets.length - 10} more</li>}</ul>, confirmLabel: 'Delete', danger: true })) return
    setDeleting(true)
    let failed = 0
    for (const p of targets) {
      try { await ueProfiles.remove(p!.id!) } catch { failed += 1 }
    }
    setDeleting(false)
    setTicked(new Set())
    if (failed) addError(`${failed} of ${targets.length} profiles could not be deleted`)
    else addSuccess(`Deleted ${targets.length} UE profile${targets.length === 1 ? '' : 's'}`)
  }

  async function bulk(count: number) {
    if (!profile?.id) return
    try {
      const created = await ueProfiles.bulk(profile.id, count)
      addSuccess(`Created ${created.length} UE profile${created.length === 1 ? '' : 's'}`)
    } catch (error) {
      addError(extractErrorMessage(error, 'Bulk create failed'))
    }
  }

  // Deploy (opens the gNB menu) or Stop: the row's end and the detail header
  function toggle(r: UeRow) {
    if (r.status !== 'stopped') return <Button kind="danger" small busy={ue.pendingInstances[r.ueId]} onClick={() => stop(r.ueId)}>Stop</Button>
    const why = r.source === 'other' ? 'No saved UE profile matches this UE' : blocked
    const p = profileOf(r)
    const preset = p?.gnbProfileId ? lab.gnbProfiles.profiles.find((g) => g.id === p.gnbProfileId) : undefined
    const own = p?.gnbProfileId ? { name: preset?.name ?? 'a deleted gNB profile', blocked: lab.gnbs.some((g) => g.profileId === p.gnbProfileId && g.status === 'running') ? undefined : 'not running' } : undefined
    return <DeployMenu busy={ue.pendingInstances[r.ueId]} blocked={why} own={own} onPick={(gnb) => deploy(r.ueId, gnb)} />
  }

  const list = (
    <ItemList
      label="UEs"
      columns={['Profile', 'IMSI', 'gNB', 'State', 'UE IP']}
      template="minmax(0, 1.1fr) minmax(0, 1.3fr) minmax(0, 0.8fr) minmax(0, 0.8fr) minmax(0, 0.9fr)"
      rows={rows.slice(0, 300).map((r) => {
        const l = r.status === 'stopped' ? undefined : ueLive[r.ueId]
        return {
          id: r.ueId,
          status: l?.stage === 'failed' && r.status === 'running' ? 'unhealthy' : r.status,
          cells: [r.profileName ?? '—', r.ueId.replace(/^imsi-/, ''), r.status === 'stopped' ? '—' : gnbName(r), r.status === 'stopped' ? 'stopped' : l?.stage ? STAGE_LABEL[l.stage] : r.status, l?.ip ?? '—'],
          action: toggle(r),
          checkable: r.source === 'other' ? 'No saved profile to delete' : r.status !== 'stopped' ? 'Stop it first' : true,
        }
      })}
      selectedId={row?.ueId ?? null}
      onSelect={pick}
      checked={ticked}
      onChecked={setTicked}
      defaultSort={{ key: '0', dir: 'asc' }}
      head={(
        <ListToolbar
          onNew={() => setCreating({})}
          bulk={<BulkCreate disabled={!profile} title={profile ? `Copy ${profile.name}; the MSIN counts up to the next free value` : 'Select a UE with a profile to copy'} onCreate={bulk} />}
          find={find}
          onFind={setFind}
          findLabel="Find a UE"
          show={show}
          onShow={setShow}
          counts={{ all: ue.rows.length, running: lab.upUes.length }}
          ticked={ticked.size}
          deploySelected={<Button kind="primary" small busy={Boolean(deployingMany)} disabled={Boolean(blocked)} title={blocked ?? "Each onto its profile's gNB"} onClick={() => deployTicked('')}>{deployingMany ?? `Deploy selected (${ticked.size})`}</Button>}
          onDeleteSelected={removeTicked}
          deleting={deleting}
          startAll={<Button kind="primary" small busy={stoppedIds.some((id) => ue.pendingInstances[id])} disabled={Boolean(blocked) || stoppedIds.length === 0} title={blocked ?? "Each onto its profile's gNB"} onClick={() => launchStopped('')}>Start all stopped</Button>}
          onStopAll={stopAll}
          stopAll={{ disabled: !lab.ueUp, busy: anyPending }}
        />
      )}
      empty={ue.rows.length ? 'Nothing matches.' : 'No UEs yet. Press + New: every UE profile is one UE.'}
    />
  )

  const notify = (ok: boolean, msg: string) => (ok ? addSuccess(msg) : addError(msg))
  const onSaved = (id: string | null) => {
    const p = ueProfiles.profiles.find((x) => x.id === id)
    setCreating(null)
    setTab('config')
    if (p) setParams({ ue: `imsi-${p.mcc}${p.mnc}${p.msin}` }, { replace: true })
  }

  let detail = <p className={ui.empty}>Press + New to create the first UE profile.</p>
  if (creating) {
    const from = creating.from ? ueProfiles.profiles.find((p) => p.id === creating.from) : undefined
    detail = (
      <>
        <DetailHeader icon="ue" name="New UE profile" sub={from ? `copy of ${from.name}` : 'from the defaults'} actions={<Button small onClick={() => setCreating(null)}>Cancel</Button>} />
        <UeProfilesPanel key={`new-${creating.from ?? ''}`} lab={lab} selectedId={null} copyOf={from} onSelect={onSaved} onNotify={notify} />
      </>
    )
  } else if (row) {
    const running = row.status === 'running'
    detail = (
      <>
        <DetailHeader
          icon="ue"
          name={row.ueId}
          sub={profile ? `profile · ${profile.name}` : 'deployed earlier, no saved profile'}
          status={live?.stage === 'failed' && running ? 'unhealthy' : row.status}
          actions={(
            <>
              {toggle(row)}
              <Button small disabled={!running} title="Opens on the Overview" onClick={() => navigate(`/?terminal=${encodeURIComponent(row.ueId)}`)}>Terminal</Button>
              <Button small disabled={!row.hasBeenDeployed} onClick={() => navigate(logsHref(ID.ue(row.ueId)))}>Logs</Button>
              <Button small disabled={!profile} onClick={() => setCreating({ from: profile?.id })}>Duplicate</Button>
              <Button kind="danger" small disabled={!profile || row.status !== 'stopped'} title={row.status !== 'stopped' ? 'Stop the UE first' : undefined} onClick={() => remove(row)}>Delete</Button>
            </>
          )}
          tab={shownTab}
          tabs={profile ? [{ value: 'status', label: 'Status' }, { value: 'config', label: 'Config' }] : undefined}
          onTab={setTab}
        />
        {shownTab === 'config' && profile
          ? <UeProfilesPanel key={profile.id} lab={lab} selectedId={profile.id ?? null} onSelect={onSaved} onNotify={notify} />
          : (
            <Section title="Session">
              <KV rows={[
                { k: 'Stage', v: row.status === 'stopped' ? 'stopped' : live?.stage ? STAGE_LABEL[live.stage] : 'starting' },
                { k: 'gNB', v: row.status === 'stopped' ? (profile?.gnbProfileId ? `${lab.gnbProfiles.profiles.find((g) => g.id === profile.gnbProfileId)?.name ?? 'a deleted gNB profile'} (preset)` : 'Auto, by SD') : <a href="/free-ran" onClick={(e) => { e.preventDefault(); navigate('/free-ran') }}>{gnbName(row)}</a> },
                { k: 'UE IP', v: live?.ip ?? '—' },
                { k: 'Tunnel', v: live?.tun ?? '—' },
                { k: 'PLMN', v: row.plmnId },
                ...(row.sd ? [{ k: 'SD', v: row.sd }] : []),
                { k: 'Last deployed', v: row.lastDeployed },
              ]} />
              {row.status === 'stopped' && blocked && <Hint>{blocked}</Hint>}
              {row.source === 'other' && <Hint>No saved UE profile matches this UE, so it cannot be deployed again.</Hint>}
              {live?.lastError && <Hint>{live.lastError}</Hint>}
            </Section>
          )}
      </>
    )
  }

  return (
    <AppLayout crumbs={[{ label: 'UEs' }]} fill>
      <Workbench
        title="UEs"
        status={<><span>profiles <b>{ueProfiles.profiles.length}</b></span><span>running <b>{ueCounts.running}/{ueCounts.total}</b></span><span>PDU sessions <b>{ready}</b></span><span>gNBs <b>{lab.gnbs.length}</b></span></>}
        body={<Split storageKey="frulab-split-ue-6040" defaultRatio={0.6} left={list} right={detail} leftLabel="UE list" rightLabel="UE detail" />}
      />
    </AppLayout>
  )
}
