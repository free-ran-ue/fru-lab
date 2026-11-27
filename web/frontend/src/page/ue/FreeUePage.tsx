import { useState } from 'react'
import AppLayout from '../../components/layout/AppLayout'
import NotificationContainer from '../../components/notifications/NotificationContainer'
import { useNotifications } from '../../hooks/useNotifications'
import { extractErrorMessage } from '../../apiClient'
import LabCanvas from '../../lab/canvas/LabCanvas'
import { ID, buildGraph } from '../../lab/canvas/graph'
import { useEvents } from '../../lab/console/EventsContext'
import { STAGE_LABEL } from '../../lab/console/events'
import { useLab } from '../../lab/LabContext'
import { Actions, Button, Dot, Hint, InspectorHead, KV, Section, Tabs, ui } from '../../lab/LabUi'
import UeProfilesPanel from '../../lab/profiles/UeProfilesPanel'
import { Switch } from '../../lab/profiles/parts'
import Workbench, { logSourceFor } from '../../lab/workbench/Workbench'
import NodeInspector from '../../lab/workbench/NodeInspector'
import { useConsole } from '../../lab/workbench/Console'

// FreeUePage runs the UE fleet: the UEs of every UE profile plus, while the
// lab core runs and the user wants them, the free5GC subscribers. Select a
// UE to deploy or stop it, open a shell in it, or test its data path from
// the console.
export default function FreeUePage() {
  const lab = useLab()
  const { ueLive } = useEvents()
  const { ue, nodes, ueCounts } = lab
  const con = useConsole()
  const { errors, successes, addError, addSuccess, removeNotification } = useNotifications()
  const [selected, setSelected] = useState<string | null>(null)
  const [tab, setTab] = useState<'fleet' | 'profiles'>('fleet')
  const [profileId, setProfileId] = useState<string | null>(null)
  const [find, setFind] = useState('')

  const blocked = lab.blocked.ueDeploy
  const anyPending = lab.upUes.some((r) => ue.pendingInstances[r.ueId])
  const graph = buildGraph(lab, 'ue', 'basic', ueLive)
  const ready = ue.rows.filter((r) => r.status === 'running' && ueLive[r.ueId]?.ip).length
  const failed = ue.rows.filter((r) => r.status !== 'stopped' && ueLive[r.ueId]?.stage === 'failed').length

  async function deploy(ueId: string) {
    try {
      await ue.deploy(ueId)
      addSuccess(`${ueId} is starting`)
      con.setSource({ target: 'ue', instance: ueId })
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

  async function launchStopped() {
    const ids = ue.rows.filter((r) => r.status === 'stopped' && r.source !== 'other').map((r) => r.ueId)
    const failedIds = await ue.deployMany(ids)
    if (failedIds.length) addError(`${failedIds.length} of ${ids.length} UEs failed to start`)
    else addSuccess(`Started ${ids.length} UE${ids.length === 1 ? '' : 's'}`)
  }

  const ueId = selected?.startsWith('ue:') ? selected.slice(3) : null
  const row = ueId ? ue.rows.find((r) => r.ueId === ueId) : null
  const live = ueId ? ueLive[ueId] : undefined

  const ueInspector = row && (
    <>
      <InspectorHead icon="ue" name={row.ueId} sub={row.source === 'profile' ? `profile · ${row.profileName}` : row.source === 'subscriber' ? 'free5GC subscriber' : 'deployed earlier'} status={live?.stage === 'failed' && row.status === 'running' ? 'unhealthy' : row.status} />
      <Section title="Session">
        <KV rows={[
          { k: 'Stage', v: row.status === 'stopped' ? 'stopped' : live?.stage ? STAGE_LABEL[live.stage] : 'starting' },
          { k: 'UE IP', v: row.status !== 'stopped' && live?.ip ? live.ip : '—' },
          { k: 'Tunnel', v: row.status !== 'stopped' && live?.tun ? live.tun : '—' },
          { k: 'PLMN', v: row.plmnId },
          ...(row.sd ? [{ k: 'Slice SD', v: row.sd }] : []),
          { k: 'Last deployed', v: row.lastDeployed },
        ]} />
        {row.status === 'stopped' && blocked && <Hint>{blocked}</Hint>}
        {row.status !== 'stopped' && live?.lastError && <Hint>{live.lastError}</Hint>}
        <Actions>
          {row.status === 'stopped'
            ? <Button kind="primary" small busy={ue.pendingInstances[row.ueId]} disabled={Boolean(blocked) || row.source === 'other'} onClick={() => deploy(row.ueId)}>Deploy</Button>
            : <Button kind="danger" small busy={ue.pendingInstances[row.ueId]} onClick={() => stop(row.ueId)}>Stop</Button>}
          {row.status === 'running' && <Button small onClick={() => con.show('tests')}>Test data path</Button>}
          {row.status === 'running' && <Button small onClick={() => con.show('terminal')}>Terminal</Button>}
          {row.hasBeenDeployed && <Button small onClick={() => { con.setSource({ target: 'ue', instance: row.ueId }); con.show('logs') }}>Logs</Button>}
        </Actions>
      </Section>
    </>
  )

  const fleetInspector = (
    <>
      <InspectorHead icon="fleet" name="UE fleet" sub="one free-ran-ue container per UE" status={nodes.ue.status} statusText={`${ueCounts.running}/${ueCounts.total}`} />
      <Section title="Sources">
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
          <span style={{ flex: 1, fontSize: '0.75rem' }}>free5GC subscribers<br /><span className={ui.headSub}>{lab.coreRunning ? 'in the fleet while on' : 'needs the lab core running'}</span></span>
          <Switch label="Use free5GC subscribers" checked={lab.useSubscribers} disabled={!lab.coreRunning} onChange={lab.setSubscribersWanted} />
        </div>
        <Actions><Button small onClick={() => setTab('profiles')}>UE profiles ({lab.ueProfiles.profiles.length})</Button></Actions>
      </Section>
      <Section title="Fleet" aside={<span>{ready} with PDU session</span>}>
        {blocked && <Hint>{blocked}</Hint>}
        <Actions>
          <Button kind="primary" small disabled={Boolean(blocked) || !ue.rows.some((r) => r.status === 'stopped' && r.source !== 'other')} busy={ue.rows.some((r) => r.status === 'stopped' && ue.pendingInstances[r.ueId])} onClick={launchStopped}>Start all stopped</Button>
          <Button kind="danger" small disabled={!lab.ueUp} busy={anyPending} onClick={stopAll}>Stop all</Button>
        </Actions>
      </Section>
      <Section title="UEs" aside={<span>{ue.rows.length}</span>}>
        {ue.rows.length > 12 && <input className={ui.select} style={{ width: '100%', marginBottom: '0.5rem' }} placeholder="find a UE" value={find} onChange={(e) => setFind(e.target.value)} aria-label="Find a UE" />}
        {ue.rows.length === 0
          ? <p className={ui.empty} style={{ padding: 0 }}>No UEs yet. Create a UE profile, or turn on the free5GC subscribers.</p>
          : (
            <ul className={ui.list}>
              {ue.rows.filter((r) => !find || r.ueId.includes(find)).sort((a, b) => Number(a.status === 'stopped') - Number(b.status === 'stopped')).slice(0, 150).map((r) => {
                const l = r.status === 'stopped' ? undefined : ueLive[r.ueId]
                return (
                  <li key={r.ueId}>
                    <button type="button" onClick={() => setSelected(ID.ue(r.ueId))}>
                      <Dot status={l?.stage === 'failed' ? 'unhealthy' : r.status} />{r.ueId.replace(/^imsi-/, '')}
                      <span style={{ marginLeft: 'auto' }}>{r.status === 'stopped' ? 'stopped' : l?.ip ?? (l?.stage ? STAGE_LABEL[l.stage] : r.status)}</span>
                    </button>
                  </li>
                )
              })}
            </ul>
          )}
      </Section>
    </>
  )

  const inspector = (
    <>
      <Tabs value={tab} onChange={setTab} tabs={[{ value: 'fleet', label: row ? 'UE' : 'Fleet' }, { value: 'profiles', label: `Profiles (${lab.ueProfiles.profiles.length})` }]} />
      {tab === 'profiles'
        ? <UeProfilesPanel lab={lab} selectedId={profileId} onSelect={setProfileId} onNotify={(ok, msg) => (ok ? addSuccess(msg) : addError(msg))} />
        : row ? ueInspector : selected && selected !== ID.fleet ? <NodeInspector id={selected} onLogs={() => { const s = logSourceFor(selected); if (s) { con.setSource(s); con.show('logs') } }} /> : fleetInspector}
    </>
  )

  return (
    <AppLayout crumbs={[{ label: 'UEs' }]} fill overlay={<NotificationContainer errors={errors} successes={successes} onClose={removeNotification} />}>
      <Workbench
        title="free-ue"
        status={<><span>running <b>{ueCounts.running}/{ueCounts.total}</b></span><span>PDU sessions <b>{ready}</b></span>{failed > 0 && <span>failed <b>{failed}</b></span>}<span>gNB <b>{nodes.gnb.status}</b></span></>}
        actions={<>
          <Button disabled={Boolean(blocked) || !ue.rows.some((r) => r.status === 'stopped' && r.source !== 'other')} onClick={launchStopped}>Start all stopped</Button>
          <Button kind="danger" disabled={!lab.ueUp} busy={anyPending} onClick={stopAll}>Stop all</Button>
        </>}
        console={con}
        selectedId={selected}
        onSelect={(id) => { setSelected(id); setTab('fleet') }}
        canvas={(
          <LabCanvas
            label="Every UE, wired over Uu to its gNB; each shows its stage and IP."
            {...graph}
            selectedId={selected}
            onSelect={(id) => { setSelected(id); setTab('fleet'); const s = logSourceFor(id); if (s) con.setSource(s) }}
          />
        )}
        inspector={inspector}
      />
    </AppLayout>
  )
}
