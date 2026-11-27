import { useEffect, useState } from 'react'
import type { GnbProfile, HostInterface } from '../../api'
import { api, extractErrorMessage } from '../../apiClient'
import { Button, Segmented } from '../LabUi'
import type { LabGnb } from '../useLab'
import { LAB_DEFAULT_GNB, useGnbProfiles } from '../useProfiles'
import { ConflictNote, Field, ps, useConfigPreview, useProfileCheck, Yaml } from './parts'

const HEX6 = /^[0-9a-fA-F]{6}$/
const IP = /^(\d{1,3}\.){3}\d{1,3}$|:/
const LEVELS = ['error', 'warn', 'info', 'debug', 'trace']
// the fields the form shows a conflict under; any other comes as a banner
const SHOWN = new Set(['name', 'gnbId', 'gnbName', 'mcc', 'mnc', 'tac', 'sst', 'sd', 'n2Ip', 'amfIp', 'amfPort', 'upfIp', 'upfPort', 'hostInterface', 'hostIp'])

// useHostInterfaces loads the host's interfaces once `enabled` turns on
// (an external-core profile is being edited).
function useHostInterfaces(enabled: boolean) {
  const [list, setList] = useState<HostInterface[] | null>(null)
  const [error, setError] = useState<string | null>(null)
  useEffect(() => {
    if (!enabled || list) return
    api.hostInterfaceList()
      .then((r) => { setList(r.data); setError(null) })
      .catch((e) => setError(extractErrorMessage(e, 'Cannot list the host interfaces')))
  }, [enabled, list])
  return { list: list ?? [], loaded: list !== null, error }
}

interface Props {
  profiles: ReturnType<typeof useGnbProfiles>
  // '' is a new, unsaved profile
  selectedId: string
  onSelect: (id: string) => void
  // a new profile starts as a copy of this one (Duplicate); without it,
  // from the defaults
  copyOf?: GnbProfile
  // the selected profile's gNB when it is up
  running?: LabGnb
  onNotify: (ok: boolean, msg: string) => void
}

// GnbProfilesPanel is the Config tab of a gNB: the profile's form (every
// profile is one gNB) with Save / Discard on a bar at its top, the
// gnbcfg.yaml it deploys rendered live, and clashes with other saved
// profiles shown under each field. Deploying and deleting live in the
// detail header, not here.
export default function GnbProfilesPanel({ profiles, selectedId, onSelect, copyOf, running, onNotify }: Props) {
  const saved = profiles.profiles.find((p) => p.id === selectedId)
  const initial = (): GnbProfile => saved ?? (copyOf ? { ...copyOf, id: undefined, updatedAt: undefined, name: `${copyOf.name} copy` } : LAB_DEFAULT_GNB)
  const [base, setBase] = useState<GnbProfile>(initial)
  const [draft, setDraft] = useState<GnbProfile>(base)
  const [saving, setSaving] = useState(false)
  const dirty = JSON.stringify(draft) !== JSON.stringify(base)
  const isNew = !draft.id
  const external = draft.core === 'external'
  const preview = useConfigPreview(draft, profiles.render)
  const check = useProfileCheck(draft, profiles.check)
  const hosts = useHostInterfaces(external)

  // a different profile picked (or the saved one reloaded): edit that one
  useEffect(() => {
    const b = initial()
    setBase(b)
    setDraft(b)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedId, saved])

  const set = (patch: Partial<GnbProfile>) => setDraft((d) => ({ ...d, ...patch }))
  const conflict = check.byField

  // external core: the interface the gNB listens on, then one of its addresses
  const iface = hosts.list.find((h) => h.name === draft.hostInterface)
  const ifaceMissing = Boolean(draft.hostInterface) && hosts.loaded && !iface
  function pickInterface(name: string) {
    const h = hosts.list.find((x) => x.name === name)
    set({ hostInterface: name || undefined, hostIp: h?.addresses[0] ?? undefined })
  }

  async function save() {
    setSaving(true)
    try {
      const p = await profiles.save(draft)
      onSelect(p.id ?? '')
      setBase(p)
      setDraft(p)
      onNotify(true, `Saved gNB profile ${p.name}`)
    } catch (e) {
      onNotify(false, extractErrorMessage(e, 'Failed to save the profile'))
    } finally {
      setSaving(false)
    }
  }


  const hasConflicts = check.conflicts.length > 0
  const others = check.conflicts.filter((c) => !SHOWN.has(c.field))

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
        {isNew && <p className={ps.banner}>A new profile{draft.name ? ` (${draft.name})` : ''}. Every field is editable; save it, then deploy it from the header.</p>}
        {running && <p className={ps.banner}>This profile's gNB runs as {running.name}. Changes apply the next time it is deployed.</p>}
        {others.map((c) => <p key={c.field} className={`${ps.banner} ${ps.bannerWarn}`}>{c.message}</p>)}
        <div className={ps.grid}>
          <Field wide label="Profile name" value={draft.name} onChange={(v) => set({ name: v })} conflict={conflict('name')} />
          <div className={`${ps.field} ${ps.wide}`}>
            <label>Core network</label>
            <Segmented
              value={draft.core}
              options={[{ value: 'lab', label: 'Lab free5GC' }, { value: 'external', label: 'External core' }]}
              onChange={(v) => set(v === 'external'
                ? { core: v, amfIp: draft.amfIp ?? '', amfPort: draft.amfPort ?? 38412, upfIp: draft.upfIp ?? '', upfPort: draft.upfPort ?? 2152, n2Ip: undefined }
                : { core: v, amfIp: undefined, amfPort: undefined, upfIp: undefined, upfPort: undefined, hostInterface: undefined, hostIp: undefined, n2Ip: draft.n2Ip ?? LAB_DEFAULT_GNB.n2Ip })}
            />
          </div>
          {!external && (
            <Field wide label="N2 IP" value={draft.n2Ip ?? ''} onChange={(v) => set({ n2Ip: v })} pattern={IP} conflict={conflict('n2Ip')} hint="The gNB's N2/N3 address on the lab network: 10.0.1.20 - 10.0.1.254. The address UEs dial is assigned on deploy." />
          )}
          {external && (
            <>
              <Field label="AMF N2 IP" value={draft.amfIp ?? ''} onChange={(v) => set({ amfIp: v })} pattern={IP} conflict={conflict('amfIp')} />
              <Field label="AMF port" type="number" value={draft.amfPort ?? 38412} onChange={(v) => set({ amfPort: Number(v) })} conflict={conflict('amfPort')} />
              <Field label="UPF N3 IP" value={draft.upfIp ?? ''} onChange={(v) => set({ upfIp: v })} pattern={IP} conflict={conflict('upfIp')} />
              <Field label="UPF port" type="number" value={draft.upfPort ?? 2152} onChange={(v) => set({ upfPort: Number(v) })} conflict={conflict('upfPort')} />
              <div className={ps.field}>
                <label htmlFor="gnb-iface">Host interface</label>
                <select id="gnb-iface" className={`${ps.input} ${ifaceMissing || conflict('hostInterface') ? ps.inputBad : ''}`} value={draft.hostInterface ?? ''} onChange={(e) => pickInterface(e.target.value)}>
                  <option value="">{hosts.loaded ? 'pick an interface' : 'loading…'}</option>
                  {hosts.list.map((h) => <option key={h.name} value={h.name}>{h.name} · {h.addresses.join(', ')}</option>)}
                  {ifaceMissing && <option value={draft.hostInterface}>{draft.hostInterface} (not on this host)</option>}
                </select>
                {hosts.error && <span className={ps.fieldHint}>{hosts.error}</span>}
                <ConflictNote conflict={conflict('hostInterface')} onUse={pickInterface} />
              </div>
              <div className={ps.field}>
                <label htmlFor="gnb-hostip">Host IP</label>
                {iface && iface.addresses.length > 1
                  ? (
                    <select id="gnb-hostip" className={`${ps.input} ${conflict('hostIp') ? ps.inputBad : ''}`} value={draft.hostIp ?? ''} onChange={(e) => set({ hostIp: e.target.value })}>
                      {!iface.addresses.includes(draft.hostIp ?? '') && <option value={draft.hostIp ?? ''}>{draft.hostIp || 'pick an address'}</option>}
                      {iface.addresses.map((a) => <option key={a} value={a}>{a}</option>)}
                    </select>
                  )
                  : <span id="gnb-hostip" className={ps.readonly}>{draft.hostIp || '—'}</span>}
                <ConflictNote conflict={conflict('hostIp')} onUse={(v) => set({ hostIp: v })} />
              </div>
              <span className={`${ps.fieldHint} ${ps.wide}`}>The gNB runs on the host network and listens on this address; the core must reach it, and UEs dial it too.</span>
            </>
          )}
          <Field label="gNB ID" value={draft.gnbId} onChange={(v) => set({ gnbId: v })} pattern={HEX6} hint="6 hex digits" conflict={conflict('gnbId')} />
          <Field label="gNB name" value={draft.gnbName ?? ''} onChange={(v) => set({ gnbName: v })} conflict={conflict('gnbName')} />
          <Field label="MCC" value={draft.mcc} onChange={(v) => set({ mcc: v })} pattern={/^\d{3}$/} conflict={conflict('mcc')} />
          <Field label="MNC" value={draft.mnc} onChange={(v) => set({ mnc: v })} pattern={/^\d{2,3}$/} conflict={conflict('mnc')} />
          <Field label="TAC" value={draft.tac} onChange={(v) => set({ tac: v })} pattern={HEX6} hint="6 hex digits" conflict={conflict('tac')} />
          <Field label="SST" value={draft.sst} onChange={(v) => set({ sst: v })} pattern={/^\d{1,3}$/} conflict={conflict('sst')} />
          <Field label="SD" value={draft.sd ?? ''} onChange={(v) => set({ sd: v })} pattern={/^([0-9a-fA-F]{6})?$/} hint="6 hex digits, optional" conflict={conflict('sd')} />
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

    </div>
  )
}
