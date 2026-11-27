import { useEffect, useState } from 'react'
import type { UeProfile } from '../../api'
import { extractErrorMessage } from '../../apiClient'
import { Button } from '../LabUi'
import type { Lab } from '../useLab'
import { NEW_UE_PROFILE, profileUeId } from '../useProfiles'
import { Field, ps, useConfigPreview, useProfileCheck, Yaml } from './parts'

const HEX = (n: number) => new RegExp(`^[0-9a-fA-F]{${n}}$`)
// the fields the form shows a conflict under; any other comes as a banner
const SHOWN = new Set(['name', 'mcc', 'mnc', 'msin', 'permanentKey', 'opValue', 'amf', 'sqn', 'dnn', 'sst', 'sd'])

interface Props {
  lab: Lab
  // null is a new, unsaved profile
  selectedId: string | null
  onSelect: (id: string | null) => void
  onNotify: (ok: boolean, msg: string) => void
  // a new profile starts as a copy of this one (Duplicate); without it,
  // from the defaults
  copyOf?: UeProfile
}

// UeProfilesPanel is the Config tab of a UE: the profile's form (every
// profile is one UE, imsi-<mcc><mnc><msin>) with Save / Discard on a bar
// at its top, its uecfg.yaml rendered live, and clashes with other saved
// profiles under each field. Deploying and deleting live in the detail
// header, not here.
export default function UeProfilesPanel({ lab, selectedId, onSelect, onNotify, copyOf }: Props) {
  const { ueProfiles, ue } = lab
  const saved = ueProfiles.profiles.find((p) => p.id === selectedId)
  const initial = (): UeProfile => saved ?? (copyOf ? { ...copyOf, id: undefined, updatedAt: undefined, name: `${copyOf.name} copy` } : NEW_UE_PROFILE)
  const [base, setBase] = useState<UeProfile>(initial)
  const [draft, setDraft] = useState<UeProfile>(base)
  const [saving, setSaving] = useState(false)
  const dirty = JSON.stringify(draft) !== JSON.stringify(base)
  const isNew = !draft.id
  const preview = useConfigPreview(draft, ueProfiles.render)
  const check = useProfileCheck(draft, ueProfiles.check)

  useEffect(() => {
    const b = initial()
    setBase(b)
    setDraft(b)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedId, saved])

  const set = (patch: Partial<UeProfile>) => setDraft((d) => ({ ...d, ...patch }))
  const conflict = check.byField

  // this profile's UE, once saved
  const row = draft.id ? ue.rows.find((r) => r.profileId === draft.id) : undefined
  const up = row && row.status !== 'stopped'
  const hasConflicts = check.conflicts.length > 0
  const others = check.conflicts.filter((c) => !SHOWN.has(c.field))

  async function save() {
    setSaving(true)
    try {
      const p = await ueProfiles.save(draft)
      onSelect(p.id ?? null)
      setBase(p)
      setDraft(p)
      onNotify(true, `Saved UE profile ${p.name}`)
    } catch (e) {
      onNotify(false, extractErrorMessage(e, 'Failed to save the profile'))
    } finally {
      setSaving(false)
    }
  }




  return (
    <div className={ps.panel}>
      <div className={ps.saveBar}>
        <span className={ps.saveState}>{hasConflicts ? `${check.conflicts.length} conflict${check.conflicts.length === 1 ? '' : 's'} to resolve` : isNew ? 'Not saved yet' : dirty ? 'Unsaved changes' : 'Saved'}</span>
        {draft.id && dirty && <Button small onClick={() => setDraft(base)}>Discard</Button>}
        <Button kind="primary" small busy={saving} disabled={hasConflicts || (!dirty && !isNew)} title={hasConflicts ? 'Resolve the conflicts first' : undefined} onClick={save}>
          {isNew ? 'Save profile' : 'Save'}
        </Button>
      </div>
      <div className={ps.section}>
        {up && <p className={ps.banner}>This profile's UE is running. Changes apply the next time it is deployed.</p>}
        {isNew && <p className={ps.banner}>A new profile{draft.name ? ` (${draft.name})` : ''}. Every field is editable; save it, then deploy it from the header.</p>}
        {lab.coreRunning && <p className={ps.banner}>On the lab free5GC this UE registers only if it has a matching subscriber (Core Network → Subscribers).</p>}
        {others.map((c) => <p key={c.field} className={`${ps.banner} ${ps.bannerWarn}`}>{c.message}</p>)}
        <div className={ps.grid}>
          <Field wide label="Profile name" value={draft.name} onChange={(v) => set({ name: v })} conflict={conflict('name')} />
          <Field label="MCC" value={draft.mcc} onChange={(v) => set({ mcc: v })} pattern={/^\d{3}$/} conflict={conflict('mcc')} />
          <Field label="MNC" value={draft.mnc} onChange={(v) => set({ mnc: v })} pattern={/^\d{2,3}$/} conflict={conflict('mnc')} />
          <Field wide label="MSIN" value={draft.msin} onChange={(v) => set({ msin: v })} pattern={/^\d{9,10}$/} hint={<>9 or 10 digits · <span className={ps.range}>{profileUeId(draft)}</span></>} conflict={conflict('msin')} />
          <Field wide label="Key (K)" value={draft.permanentKey} onChange={(v) => set({ permanentKey: v })} pattern={HEX(32)} conflict={conflict('permanentKey')} />
          <Field wide label="OPc" value={draft.opValue} onChange={(v) => set({ opValue: v })} pattern={HEX(32)} conflict={conflict('opValue')} />
          <Field label="AMF" value={draft.amf} onChange={(v) => set({ amf: v })} pattern={HEX(4)} conflict={conflict('amf')} />
          <Field label="SQN" value={draft.sqn} onChange={(v) => set({ sqn: v })} pattern={HEX(12)} conflict={conflict('sqn')} />
          <Field label="DNN" value={draft.dnn ?? ''} onChange={(v) => set({ dnn: v })} conflict={conflict('dnn')} />
          <Field label="SST" value={draft.sst} onChange={(v) => set({ sst: v })} pattern={/^\d{1,3}$/} conflict={conflict('sst')} />
          <Field label="SD" value={draft.sd ?? ''} onChange={(v) => set({ sd: v })} pattern={/^([0-9a-fA-F]{6})?$/} hint="Auto picks a gNB with this SD" conflict={conflict('sd')} />
          <div className={`${ps.field} ${ps.wide}`}>
            <label htmlFor="ue-gnb">gNB</label>
            <select id="ue-gnb" className={ps.input} value={draft.gnbProfileId ?? ''} onChange={(e) => set({ gnbProfileId: e.target.value || undefined })}>
              <option value="">Auto: a running gNB with this SD</option>
              {lab.gnbProfiles.profiles.map((g) => <option key={g.id} value={g.id}>{g.name}{g.sd ? ` · SD ${g.sd}` : ''}</option>)}
            </select>
            <span className={ps.fieldHint}>Where this UE deploys by default (and with Deploy selected); a row's Deploy can still pick another.</span>
          </div>
        </div>
      </div>

      <div className={ps.section}>
        <p className={ps.sectionTitle}>uecfg.yaml</p>
        <Yaml text={preview.yaml} error={preview.error} />
      </div>

    </div>
  )
}
