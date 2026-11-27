import { useState } from 'react'
import AppLayout from '../../components/layout/AppLayout'
import NotificationContainer from '../../components/notifications/NotificationContainer'
import { useNotifications } from '../../hooks/useNotifications'
import { extractErrorMessage } from '../../apiClient'
import type { GnbProfile } from '../../api'
import LabCanvas from '../../lab/canvas/LabCanvas'
import { ID, buildGraph } from '../../lab/canvas/graph'
import { useEvents } from '../../lab/console/EventsContext'
import { useLab } from '../../lab/LabContext'
import { Actions, Button, Dot, Hint, InspectorHead, KV, Section, Tabs, ui } from '../../lab/LabUi'
import GnbProfilesPanel from '../../lab/profiles/GnbProfilesPanel'
import { LAB_DEFAULT_GNB, useGnbProfiles, useStoredState } from '../../lab/useProfiles'
import Workbench, { logSourceFor } from '../../lab/workbench/Workbench'
import NodeInspector from '../../lab/workbench/NodeInspector'
import { useConsole } from '../../lab/workbench/Console'

// FreeRanPage runs the free-ran-ue gNB from a gNB profile: the lab default
// or a saved one, on the lab free5GC or on a core outside the lab. Under
// ULCL (Two Slice) it runs the two built-in slice gNBs instead.
export default function FreeRanPage() {
  const lab = useLab()
  const { ueLive, events } = useEvents()
  const { gnb, gnbSlice1, gnbSlice2, nodes, isTwoSlice } = lab
  const gnbProfiles = useGnbProfiles()
  const con = useConsole()
  const { errors, successes, addError, addSuccess, removeNotification } = useNotifications()
  const [selected, setSelected] = useState<string | null>(null)
  const [tab, setTab] = useState<'gnb' | 'profiles'>('gnb')
  const [profileId, setProfileId] = useStoredState('frulab-gnb-profile', '')

  const profile: GnbProfile = gnbProfiles.profiles.find((p) => p.id === profileId) ?? LAB_DEFAULT_GNB
  const gnbStopped = nodes.gnb.status === 'stopped'
  const pending = isTwoSlice ? gnbSlice1.isActionPending || gnbSlice2.isActionPending : gnb.isActionPending
  const deployBlocked = (p: GnbProfile) => (isTwoSlice || p.core === 'external' ? undefined : lab.blocked.gnbDeploy)
  const blocked = gnbStopped ? (isTwoSlice ? lab.blocked.gnbDeploy : deployBlocked(profile)) : lab.blocked.gnbStop
  const graph = buildGraph(lab, 'ran', 'basic', ueLive)
  const inUse = gnbStopped || isTwoSlice ? null : (gnb.gnbDeployment?.profileId ?? '')
  const d = gnb.gnbDeployment
  const ngSetup = [...events].reverse().find((e) => e.node.startsWith('gnb') && /NG Setup complete|Cannot reach the AMF/.test(e.text))

  async function deploy(id: string) {
    try {
      if (isTwoSlice) {
        await Promise.allSettled([gnbSlice1.deploy(), gnbSlice2.deploy()])
        addSuccess('The slice gNBs are starting')
      } else {
        setProfileId(id)
        await gnb.deploy(id || undefined)
        addSuccess(`gNB is starting with ${gnbProfiles.profiles.find((p) => p.id === id)?.name ?? LAB_DEFAULT_GNB.name}`)
        setTab('gnb')
        setSelected(ID.gnb)
        con.setSource({ target: 'gnb' })
        con.show('logs')
      }
    } catch (error) {
      addError(extractErrorMessage(error, 'gNB deploy failed'))
    }
  }

  async function stop() {
    try {
      if (isTwoSlice) await Promise.allSettled([gnbSlice1.stop(), gnbSlice2.stop()])
      else await gnb.stop()
      addSuccess('gNB stopped')
    } catch (error) {
      addError(extractErrorMessage(error, 'Failed to stop gNB'))
    }
  }

  const primary = gnbStopped
    ? <Button kind="primary" busy={pending} disabled={Boolean(blocked)} title={blocked} onClick={() => deploy(profileId)}>{isTwoSlice ? 'Deploy slice gNBs' : `Deploy gNB · ${profile.name}`}</Button>
    : <Button kind="danger" busy={pending} disabled={Boolean(blocked)} title={blocked} onClick={stop}>Stop gNB{isTwoSlice ? 's' : ''}</Button>

  const showLogs = (id: string) => {
    const s = logSourceFor(id)
    if (s) { con.setSource(s); con.show('logs') }
  }

  const gnbInspector = (
    <>
      <InspectorHead icon="gnb" name={isTwoSlice ? 'gNB slices' : 'gNB'} sub="free-ran-ue base station" status={nodes.gnb.status} />
      <Section title="Run">
        {!isTwoSlice && gnbStopped && (
          <div style={{ display: 'flex', gap: '0.375rem', marginBottom: '0.625rem' }}>
            <select className={ui.select} style={{ flex: 1 }} value={profileId} onChange={(e) => setProfileId(e.target.value)} aria-label="gNB profile">
              {[{ ...LAB_DEFAULT_GNB, id: '' }, ...gnbProfiles.profiles].map((p) => <option key={p.id || 'default'} value={p.id ?? ''}>{p.name} · {p.core === 'external' ? `external ${p.amfIp}` : 'lab free5GC'}</option>)}
            </select>
            <Button small onClick={() => setTab('profiles')}>Edit</Button>
          </div>
        )}
        <KV rows={isTwoSlice
          ? [{ k: 'Template', v: 'ULCL (Two Slice)' }, { k: 'slice1', v: gnbSlice1.node.status }, { k: 'slice2', v: gnbSlice2.node.status }, { k: 'UEs attached', v: lab.upUes.length }]
          : [
            { k: 'Profile', v: gnbStopped ? profile.name : d?.profileName ?? '—' },
            { k: 'Core', v: (gnbStopped ? profile.core : d?.core) === 'external' ? `external · ${gnbStopped ? profile.amfIp : d?.amfIp}` : 'lab free5GC' },
            { k: 'NG Setup', v: gnbStopped ? '—' : ngSetup ? <><Dot status={ngSetup.level === 'error' ? 'unhealthy' : 'running'} /> {ngSetup.level === 'error' ? 'failed' : 'done'}</> : 'waiting' },
            { k: 'UEs attached', v: lab.upUes.length },
            { k: 'Last deployed', v: nodes.gnb.lastDeployed },
          ]} />
        {blocked && <Hint>{blocked}</Hint>}
        <Actions>{primary}{!gnbStopped && <Button small onClick={() => showLogs(isTwoSlice ? 'gnb-slice1' : ID.gnb)}>Logs</Button>}</Actions>
      </Section>
      {lab.gnbNetworkFunctions.length > 0 && (
        <Section title="Containers">
          <ul className={ui.list}>{lab.gnbNetworkFunctions.map((s) => <li key={s.name}><Dot status={s.status} />{s.name}<span>{s.status}</span></li>)}</ul>
        </Section>
      )}
    </>
  )

  const ownsSelection = !selected || selected === ID.gnb || selected.startsWith('gnb-')
  const inspector = (
    <>
      <Tabs value={tab} onChange={setTab} tabs={[{ value: 'gnb', label: 'gNB' }, { value: 'profiles', label: `Profiles (${gnbProfiles.profiles.length + 1})` }]} />
      {tab === 'profiles'
        ? (
          <GnbProfilesPanel
            profiles={gnbProfiles}
            selectedId={profileId}
            onSelect={setProfileId}
            inUse={inUse}
            deployBlocked={deployBlocked}
            gnbUp={!gnbStopped}
            busy={pending}
            onDeploy={deploy}
            onNotify={(ok, msg) => (ok ? addSuccess(msg) : addError(msg))}
            twoSlice={isTwoSlice}
          />
        )
        : ownsSelection ? gnbInspector : <NodeInspector id={selected!} onLogs={() => showLogs(selected!)} />}
    </>
  )

  return (
    <AppLayout crumbs={[{ label: 'gNB' }]} fill overlay={<NotificationContainer errors={errors} successes={successes} onClose={removeNotification} />}>
      <Workbench
        title="free-ran"
        status={<><span>gNB <b>{nodes.gnb.status}</b></span><span>profile <b>{gnbStopped ? profile.name : d?.profileName ?? '—'}</b></span><span>core <b>{lab.gnbExternal ? `external ${d?.amfIp}` : nodes.core.status}</b></span><span>UEs <b>{lab.upUes.length}</b></span></>}
        actions={primary}
        console={con}
        selectedId={selected}
        onSelect={(id) => { setSelected(id); setTab('gnb') }}
        canvas={(
          <LabCanvas
            label="The radio side: the UE fleet over Uu to the gNB, and the gNB over N2 and N3 to its core."
            {...graph}
            selectedId={selected}
            onSelect={(id) => { setSelected(id); setTab('gnb'); const s = logSourceFor(id); if (s) con.setSource(s) }}
          />
        )}
        inspector={inspector}
      />
    </AppLayout>
  )
}
