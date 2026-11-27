import { useEffect, useState } from 'react'
import type { UeProfile } from '../../api'
import { extractErrorMessage } from '../../apiClient'
import { Button } from '../LabUi'
import type { Lab } from '../useLab'
import { msinAt, NEW_UE_PROFILE, profileUeIds } from '../useProfiles'
import { Field, ps, Switch, useConfigPreview, Yaml } from './parts'

const HEX = (n: number) => new RegExp(`^[0-9a-fA-F]{${n}}$`)

interface Props {
  lab: Lab
  selectedId: string | null
  onSelect: (id: string | null) => void
  onNotify: (ok: boolean, msg: string) => void
}

// UeProfilesPanel picks where the UE fleet comes from: the free5GC
// subscribers (while the lab core runs) and/or UE profiles, each launching
// a run of UEs with consecutive MSINs. It edits the selected profile with
// the first UE's uecfg.yaml rendered live.
export default function UeProfilesPanel({ lab, selectedId, onSelect, onNotify }: Props) {
  const { ueProfiles, ue } = lab
  const saved = ueProfiles.profiles.find((p) => p.id === selectedId)
  const base = saved ?? NEW_UE_PROFILE
  const [draft, setDraft] = useState<UeProfile>(base)
  const [saving, setSaving] = useState(false)
  const [progress, setProgress] = useState<{ done: number; total: number } | null>(null)
  const dirty = JSON.stringify(draft) !== JSON.stringify(base)
  const preview = useConfigPreview(draft, ueProfiles.render)

  useEffect(() => { setDraft(saved ?? NEW_UE_PROFILE) }, [selectedId, saved])

  const set = (patch: Partial<UeProfile>) => setDraft((d) => ({ ...d, ...patch }))
  const subscriberCount = ue.rows.filter((r) => r.source === 'subscriber').length

  const ids = draft.id && !dirty ? profileUeIds(draft) : []
  const rows = ue.rows.filter((r) => ids.includes(r.ueId))
  const stopped = rows.filter((r) => r.status === 'stopped').map((r) => r.ueId)
  const up = rows.filter((r) => r.status !== 'stopped').map((r) => r.ueId)
  const blocked = lab.blocked.ueDeploy

  const lastMsin = /^\d{9,10}$/.test(draft.msinStart) && draft.count > 0 ? msinAt(draft.msinStart, draft.count - 1) : '?'

  async function save() {
    setSaving(true)
    try {
      const p = await ueProfiles.save(draft)
      onSelect(p.id ?? null)
      onNotify(true, `Saved UE profile ${p.name}`)
    } catch (e) {
      onNotify(false, extractErrorMessage(e, 'Failed to save the profile'))
    } finally {
      setSaving(false)
    }
  }

  async function remove() {
    if (!draft.id) return
    if (up.length > 0) {
      onNotify(false, 'Stop this profile\'s UEs before deleting it')
      return
    }
    if (!window.confirm(`Delete UE profile ${draft.name}?`)) return
    try {
      await ueProfiles.remove(draft.id)
      onSelect(null)
      onNotify(true, `Deleted ${draft.name}`)
    } catch (e) {
      onNotify(false, extractErrorMessage(e, 'Failed to delete the profile'))
    }
  }

  async function launch() {
    setProgress({ done: 0, total: stopped.length })
    const failed = await ue.deployMany(stopped, (done) => setProgress({ done, total: stopped.length }))
    setProgress(null)
    onNotify(failed.length === 0, failed.length === 0 ? `Launched ${stopped.length} UE${stopped.length === 1 ? '' : 's'} from ${draft.name}` : `${failed.length} of ${stopped.length} UEs failed to start`)
  }

  async function stopAll() {
    try {
      await ue.stopMany(up)
      onNotify(true, `Stopped ${up.length} UE${up.length === 1 ? '' : 's'}`)
    } catch (e) {
      onNotify(false, extractErrorMessage(e, 'Failed to stop the UEs'))
    }
  }

  return (
    <div className={ps.panel}>
      <div className={ps.section}>
        <p className={ps.sectionTitle}>UE sources</p>
        <div className={ps.switchRow}>
          <p>
            <b>free5GC subscribers {lab.coreRunning && <span className={ps.range}>· {subscriberCount}</span>}</b>
            {lab.coreRunning ? 'Put every subscriber of the running free5GC in the fleet.' : 'Deploy the lab core network to use its subscribers.'}
          </p>
          <Switch label="Use free5GC subscribers" checked={lab.useSubscribers} disabled={!lab.coreRunning} onChange={lab.setSubscribersWanted} />
        </div>
      </div>

      <div className={ps.section}>
        <p className={ps.sectionTitle}>UE profiles</p>
        <div className={ps.pills}>
          {ueProfiles.profiles.map((p) => {
            const running = ue.rows.filter((r) => r.profileId === p.id && r.status !== 'stopped').length
            return (
              <button key={p.id} type="button" className={ps.pill} aria-pressed={p.id === selectedId} onClick={() => onSelect(p.id ?? null)}>
                <b>{p.name}</b>
                <span>{p.count} UE · {p.mcc}{p.mnc}</span>
                {running > 0 && <span className={ps.inUse}>● {running} running</span>}
              </button>
            )
          })}
          <button type="button" className={`${ps.pill} ${ps.pillNew}`} aria-pressed={!draft.id} onClick={() => { onSelect(null); setDraft(NEW_UE_PROFILE) }}>+ new profile</button>
        </div>
      </div>

      <div className={ps.section}>
        {lab.coreRunning && !lab.gnbExternal && <p className={ps.banner}>These UEs register with the lab free5GC only if it has matching subscribers (Core Network → Subscribers).</p>}
        <div className={ps.grid}>
          <Field wide label="Profile name" value={draft.name} onChange={(v) => set({ name: v })} />
          <Field label="MCC" value={draft.mcc} onChange={(v) => set({ mcc: v })} pattern={/^\d{3}$/} />
          <Field label="MNC" value={draft.mnc} onChange={(v) => set({ mnc: v })} pattern={/^\d{2,3}$/} />
          <Field label="First MSIN" value={draft.msinStart} onChange={(v) => set({ msinStart: v })} pattern={/^\d{9,10}$/} hint="9 or 10 digits" />
          <Field label="How many UEs" type="number" min={1} max={500} value={draft.count} onChange={(v) => set({ count: Math.max(0, Math.min(500, Number(v) || 0)) })} />
          <div className={`${ps.field} ${ps.wide}`}>
            <span className={ps.range}>imsi-{draft.mcc}{draft.mnc}{draft.msinStart}{draft.count > 1 && <> … imsi-{draft.mcc}{draft.mnc}{lastMsin}</>}</span>
          </div>
          <Field wide label="Key (K)" value={draft.permanentKey} onChange={(v) => set({ permanentKey: v })} pattern={HEX(32)} />
          <Field wide label="OPc" value={draft.opValue} onChange={(v) => set({ opValue: v })} pattern={HEX(32)} />
          <Field label="AMF" value={draft.amf} onChange={(v) => set({ amf: v })} pattern={HEX(4)} />
          <Field label="SQN" value={draft.sqn} onChange={(v) => set({ sqn: v })} pattern={HEX(12)} />
          <Field label="DNN" value={draft.dnn ?? ''} onChange={(v) => set({ dnn: v })} />
          <Field label="SST" value={draft.sst} onChange={(v) => set({ sst: v })} pattern={/^\d{1,3}$/} />
          <Field label="SD" value={draft.sd ?? ''} onChange={(v) => set({ sd: v })} pattern={/^([0-9a-fA-F]{6})?$/} hint="must match the gNB's slice" />
        </div>
      </div>

      <div className={ps.section}>
        <p className={ps.sectionTitle}>uecfg.yaml · first UE</p>
        <Yaml text={preview.yaml} error={preview.error} />
      </div>

      <div className={ps.actions}>
        {progress && <div className={ps.progress} role="progressbar" aria-valuenow={progress.done} aria-valuemax={progress.total}><i style={{ width: `${(progress.done / Math.max(1, progress.total)) * 100}%` }} /></div>}
        {(dirty || !draft.id) && <Button kind="primary" small busy={saving} onClick={save}>{draft.id ? 'Save changes' : 'Save profile'}</Button>}
        {draft.id && !dirty && <Button kind="danger" small onClick={remove}>Delete</Button>}
        <span className={ps.spacer} />
        {up.length > 0 && <Button kind="danger" busy={rows.some((r) => ue.pendingInstances[r.ueId]) && !progress} onClick={stopAll}>Stop {up.length}</Button>}
        <Button
          kind="primary"
          busy={Boolean(progress)}
          disabled={!draft.id || dirty || stopped.length === 0 || Boolean(blocked)}
          title={!draft.id || dirty ? 'Save the profile first' : blocked}
          onClick={launch}
        >
          {progress ? `Launching ${progress.done}/${progress.total}…` : `Launch ${stopped.length || ''} UE${stopped.length === 1 ? '' : 's'}`}
        </Button>
      </div>
    </div>
  )
}
