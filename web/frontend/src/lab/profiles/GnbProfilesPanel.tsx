import { useEffect, useState } from 'react'
import type { GnbProfile } from '../../api'
import { extractErrorMessage } from '../../apiClient'
import { Button, Segmented } from '../LabUi'
import { LAB_DEFAULT_GNB, useGnbProfiles } from '../useProfiles'
import { Field, ps, useConfigPreview, Yaml } from './parts'

const HEX6 = /^[0-9a-fA-F]{6}$/
const IP = /^(\d{1,3}\.){3}\d{1,3}$|:/
const LEVELS = ['error', 'warn', 'info', 'debug', 'trace']

interface Props {
  profiles: ReturnType<typeof useGnbProfiles>
  // '' is the lab default
  selectedId: string
  onSelect: (id: string) => void
  // the profile the running gNB uses (null when it is stopped)
  inUse: string | null
  // why deploying `p` is not possible now, if it is not
  deployBlocked: (p: GnbProfile) => string | undefined
  gnbUp: boolean
  busy: boolean
  onDeploy: (id: string) => void
  onNotify: (ok: boolean, msg: string) => void
  // ULCL (Two Slice) runs the built-in slice gNBs instead
  twoSlice: boolean
}

// GnbProfilesPanel lists the gNB profiles and edits the selected one, with
// the gnbcfg.yaml it deploys rendered live beside the form.
export default function GnbProfilesPanel({ profiles, selectedId, onSelect, inUse, deployBlocked, gnbUp, busy, onDeploy, onNotify, twoSlice }: Props) {
  const saved = profiles.profiles.find((p) => p.id === selectedId)
  const base = saved ?? LAB_DEFAULT_GNB
  const [draft, setDraft] = useState<GnbProfile>(base)
  const [saving, setSaving] = useState(false)
  const dirty = JSON.stringify(draft) !== JSON.stringify(base)
  // the untouched lab default (read-only until a field changes)
  const builtIn = !draft.id && selectedId === '' && !dirty
  const isNew = !draft.id && !builtIn
  const preview = useConfigPreview(draft, profiles.render)

  // a different profile picked: edit that one
  useEffect(() => { setDraft(saved ?? LAB_DEFAULT_GNB) }, [selectedId, saved])

  const set = (patch: Partial<GnbProfile>) => setDraft((d) => ({ ...d, ...patch }))
  const external = draft.core === 'external'

  async function save(asNew: boolean) {
    setSaving(true)
    try {
      const p = await profiles.save(asNew ? { ...draft, id: undefined } : draft)
      onSelect(p.id ?? '')
      onNotify(true, `Saved gNB profile ${p.name}`)
    } catch (e) {
      onNotify(false, extractErrorMessage(e, 'Failed to save the profile'))
    } finally {
      setSaving(false)
    }
  }

  async function remove() {
    if (!draft.id || !window.confirm(`Delete gNB profile ${draft.name}?`)) return
    try {
      await profiles.remove(draft.id)
      onSelect('')
      onNotify(true, `Deleted ${draft.name}`)
    } catch (e) {
      onNotify(false, extractErrorMessage(e, 'Failed to delete the profile'))
    }
  }

  const blocked = deployBlocked(draft) ?? (dirty ? 'Save the profile before deploying it' : undefined)

  return (
    <div className={ps.panel}>
      <div className={ps.section}>
        <p className={ps.sectionTitle}>gNB profiles</p>
        <div className={ps.pills}>
          {[{ ...LAB_DEFAULT_GNB, id: '' }, ...profiles.profiles].map((p) => (
            <button key={p.id || 'default'} type="button" className={ps.pill} aria-pressed={(p.id ?? '') === selectedId && !isNew} onClick={() => onSelect(p.id ?? '')}>
              <b>{p.name}</b>
              <span>{p.core === 'external' ? `external · AMF ${p.amfIp}` : 'lab free5GC'}</span>
              {inUse === (p.id ?? '') && <span className={ps.inUse}>● running</span>}
            </button>
          ))}
          <button type="button" className={`${ps.pill} ${ps.pillNew}`} aria-pressed={isNew} onClick={() => setDraft({ ...base, id: undefined, name: `${base.name} copy` })}>+ new profile from the selected one</button>
        </div>
      </div>

      <div className={ps.section}>
        {twoSlice && <p className={`${ps.banner} ${ps.bannerWarn}`}>The core runs ULCL (Two Slice): free-ran deploys its two built-in slice gNBs, so profiles are not used until the core runs another template.</p>}
        {builtIn && <p className={ps.banner}>The built-in lab gNB. Change any field to start a new profile from it.</p>}
        <div className={ps.grid}>
          <Field wide label="Profile name" value={draft.name} onChange={(v) => set({ name: v })} />
          <div className={`${ps.field} ${ps.wide}`}>
            <label>Core network</label>
            <Segmented
              value={draft.core}
              options={[{ value: 'lab', label: 'Lab free5GC' }, { value: 'external', label: 'External core' }]}
              onChange={(v) => set(v === 'external'
                ? { core: v, amfIp: draft.amfIp ?? '', amfPort: draft.amfPort ?? 38412, upfIp: draft.upfIp ?? '', upfPort: draft.upfPort ?? 2152, hostIp: draft.hostIp ?? '' }
                : { core: v, amfIp: undefined, amfPort: undefined, upfIp: undefined, upfPort: undefined, hostIp: undefined })}
            />
          </div>
          {external && (
            <>
              <Field label="AMF N2 IP" value={draft.amfIp ?? ''} onChange={(v) => set({ amfIp: v })} pattern={IP} />
              <Field label="AMF port" type="number" value={draft.amfPort ?? 38412} onChange={(v) => set({ amfPort: Number(v) })} />
              <Field label="UPF N3 IP" value={draft.upfIp ?? ''} onChange={(v) => set({ upfIp: v })} pattern={IP} />
              <Field label="UPF port" type="number" value={draft.upfPort ?? 2152} onChange={(v) => set({ upfPort: Number(v) })} />
              <Field wide label="Host IP" value={draft.hostIp ?? ''} onChange={(v) => set({ hostIp: v })} pattern={IP} hint="An address of this host the core can reach. The gNB runs on the host network and listens here; UEs dial it too." />
            </>
          )}
          <Field label="gNB ID" value={draft.gnbId} onChange={(v) => set({ gnbId: v })} pattern={HEX6} hint="6 hex digits" />
          <Field label="gNB name" value={draft.gnbName ?? ''} onChange={(v) => set({ gnbName: v })} />
          <Field label="MCC" value={draft.mcc} onChange={(v) => set({ mcc: v })} pattern={/^\d{3}$/} />
          <Field label="MNC" value={draft.mnc} onChange={(v) => set({ mnc: v })} pattern={/^\d{2,3}$/} />
          <Field label="TAC" value={draft.tac} onChange={(v) => set({ tac: v })} pattern={HEX6} hint="6 hex digits" />
          <Field label="SST" value={draft.sst} onChange={(v) => set({ sst: v })} pattern={/^\d{1,3}$/} />
          <Field label="SD" value={draft.sd ?? ''} onChange={(v) => set({ sd: v })} pattern={/^([0-9a-fA-F]{6})?$/} hint="6 hex digits, optional" />
          <div className={ps.field}>
            <label htmlFor="gnb-log">Log level</label>
            <select id="gnb-log" className={ps.input} value={draft.logLevel ?? 'info'} onChange={(e) => set({ logLevel: e.target.value })}>
              {LEVELS.map((l) => <option key={l}>{l}</option>)}
            </select>
          </div>
        </div>
      </div>

      <div className={ps.section}>
        <p className={ps.sectionTitle}>gnbcfg.yaml</p>
        <Yaml text={preview.yaml} error={preview.error} />
      </div>

      <div className={ps.actions}>
        {!builtIn && (dirty || isNew) && <Button kind="primary" small busy={saving} onClick={() => save(isNew)}>{isNew ? 'Save new profile' : 'Save changes'}</Button>}
        {draft.id && dirty && <Button small onClick={() => save(true)}>Save as new</Button>}
        {draft.id && !dirty && <Button kind="danger" small onClick={remove}>Delete</Button>}
        <span className={ps.spacer} />
        <Button kind="primary" busy={busy} disabled={gnbUp || twoSlice || Boolean(blocked)} title={gnbUp ? 'Stop the running gNB first' : blocked} onClick={() => onDeploy(draft.id ?? '')}>
          Deploy with this profile
        </Button>
      </div>
    </div>
  )
}
