import { useEffect, useState } from 'react'
import type { GnbProfile, HostInterface } from '../../api'
import { api, extractErrorMessage } from '../../apiClient'
import { Button } from '../LabUi'
import { useLab } from '../LabContext'
import NumberInput from '../NumberInput'
import { isFree5gcCore, type LabGnb } from '../useLab'
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
  // a copy drops n2Ip: on free5GC or a free5GC core it is assigned on deploy
  const initial = (): GnbProfile => saved ?? (copyOf ? { ...copyOf, id: undefined, updatedAt: undefined, n2Ip: undefined, name: `${copyOf.name} copy` } : LAB_DEFAULT_GNB)
  const [base, setBase] = useState<GnbProfile>(initial)
  const [draft, setDraft] = useState<GnbProfile>(base)
  const [saving, setSaving] = useState(false)
  const dirty = JSON.stringify(draft) !== JSON.stringify(base)
  const isNew = !draft.id
  const external = draft.core === 'external'
  const preview = useConfigPreview(draft, profiles.render)
  const check = useProfileCheck(draft, profiles.check)

  // a different profile picked (or the saved one reloaded): edit that one
  useEffect(() => {
    const b = initial()
    setBase(b)
    setDraft(b)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedId, saved])

  const set = (patch: Partial<GnbProfile>) => setDraft((d) => ({ ...d, ...patch }))
  // the core this gNB deploys onto by default: the lab free5GC, a free5GC
  // core network or a core outside the lab (cores are added on Core
  // Network, not here). A profile from before, with its AMF / UPF typed
  // in, shows them until it is moved to a saved core.
  const cores = useLab().coreNetworks.networks
  const free5gcCores = cores.filter((n) => isFree5gcCore(n))
  const externalCores = cores.filter((n) => !isFree5gcCore(n))
  const chosen = draft.coreNetwork ? cores.find((n) => n.id === draft.coreNetwork) : undefined
  // a saved core outside the lab: its addresses and N2 interface are used
  const onSaved = chosen && !isFree5gcCore(chosen) ? chosen : undefined
  const coreChoice = draft.coreNetwork ? draft.coreNetwork : draft.core === 'external' ? 'manual' : 'lab'
  // on free5GC or a free5GC core network the gNB's N2/N3 address is assigned on deploy
  const assigned = !external
  const hosts = useHostInterfaces(external)
  function pickCore(v: string) {
    const clear = { amfIp: undefined, amfPort: undefined, upfIp: undefined, upfPort: undefined, hostInterface: undefined, hostIp: undefined, n2Ip: undefined }
    const n = cores.find((c) => c.id === v)
    if (v === 'lab' || !n) set({ ...clear, core: 'lab', coreNetwork: undefined })
    else if (isFree5gcCore(n)) set({ ...clear, core: 'lab', coreNetwork: v })
    // an external core: its n2Interface is where the gNB listens; the host
    // IP is typed in (kept when moving between external cores)
    else set({ ...clear, core: 'external', coreNetwork: v, hostInterface: n.n2Interface || undefined, hostIp: external ? draft.hostIp : undefined })
  }
  const conflict = check.byField

  // external core typed in by hand: the interface the gNB listens on, then one of its addresses
  const iface = hosts.list.find((h) => h.name === draft.hostInterface)
  const ifaceMissing = Boolean(draft.hostInterface) && hosts.loaded && !iface
  function pickInterface(name: string) {
    const h = hosts.list.find((x) => x.name === name)
    set({ hostInterface: name || undefined, hostIp: h?.addresses[0] ?? undefined })
  }
  // a saved external core: the host IP is typed in, an address of its N2 interface
  const coreIface = onSaved?.n2Interface ? hosts.list.find((h) => h.name === onSaved.n2Interface) : undefined
  const hostIpProblem = onSaved
    ? !draft.hostIp?.trim()
      ? 'Required: the address the gNB listens on'
      : !IP.test(draft.hostIp.trim())
        ? 'An IP address'
        : coreIface && !coreIface.addresses.includes(draft.hostIp.trim())
          ? `Not an address of ${onSaved.n2Interface} on this host`
          : undefined
    : undefined

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
        <Button kind="primary" small busy={saving} disabled={hasConflicts || Boolean(hostIpProblem) || (!dirty && !isNew)} title={hasConflicts ? 'Resolve the conflicts first' : hostIpProblem} onClick={save}>
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
            <label htmlFor="gnb-core">Core network</label>
            <select id="gnb-core" className={ps.input} value={coreChoice} onChange={(e) => pickCore(e.target.value)}>
              <option value="lab">free5GC (the lab core)</option>
              {free5gcCores.length > 0 && (
                <optgroup label="free5GC on this host">
                  {free5gcCores.map((n) => <option key={n.id} value={n.id}>{n.name}</option>)}
                </optgroup>
              )}
              {externalCores.length > 0 && (
                <optgroup label="External cores">
                  {externalCores.map((n) => <option key={n.id} value={n.id}>{n.name} · AMF {n.n2Ip}{n.enabled ? '' : ' · disabled'}</option>)}
                </optgroup>
              )}
              {coreChoice === 'manual' && <option value="manual" disabled>Typed-in AMF {draft.amfIp || ''} (pick a saved core instead)</option>}
              {draft.coreNetwork && !chosen && <option value={draft.coreNetwork} disabled>A deleted core network (pick another)</option>}
            </select>
            <span className={ps.fieldHint}>Where this gNB deploys by default (and with Deploy selected); a row's Deploy can still pick another. To add a core, use Core Network → Networks.</span>
          </div>
          {assigned && (
            <div className={`${ps.field} ${ps.wide}`}>
              <label>N2 / N3 IP</label>
              <span className={ps.readonly}>Assigned on deploy</span>
              <span className={ps.fieldHint}>N2/N3 assigned on deploy: the lowest free address from .20 up on {chosen ? `${chosen.name}'s` : "the lab's"} cn-ran network{chosen?.n2Interface ? ` (${chosen.n2Interface})` : ''}. The running gNB shows it on its Status tab.</span>
            </div>
          )}
          {onSaved && (
            <>
              <div className={ps.field}><label>AMF (N2)</label><span className={ps.readonly}>{onSaved.n2Ip}</span></div>
              <div className={ps.field}><label>UPF (N3)</label><span className={ps.readonly}>{onSaved.n3Ip}</span></div>
              <div className={ps.field}>
                <label>Host interface</label>
                <span className={ps.readonly}>{onSaved.n2Interface ? `listens on ${onSaved.n2Interface}` : '—'}</span>
                <span className={ps.fieldHint}>{onSaved.n2Interface ? `${onSaved.name}'s N2 interface` : `${onSaved.name} has no N2 interface; set one on Core Network`}</span>
              </div>
              <div className={ps.field}>
                <label htmlFor="gnb-hostip">Host IP</label>
                <input
                  id="gnb-hostip"
                  className={`${ps.input} ${hostIpProblem || conflict('hostIp') ? ps.inputBad : ''}`}
                  list="gnb-hostip-options"
                  spellCheck={false}
                  autoComplete="off"
                  placeholder={coreIface?.addresses[0] ?? 'an address of the interface'}
                  value={draft.hostIp ?? ''}
                  onChange={(e) => set({ hostIp: e.target.value || undefined })}
                />
                <datalist id="gnb-hostip-options">
                  {(coreIface?.addresses ?? []).map((a) => <option key={a} value={a} />)}
                </datalist>
                <span className={ps.fieldHint}>{hostIpProblem ?? `An address of ${onSaved.n2Interface || 'its N2 interface'}${coreIface?.addresses.length ? `: ${coreIface.addresses.join(', ')}` : ''}`}</span>
                <ConflictNote conflict={conflict('hostIp')} onUse={(v) => set({ hostIp: v })} />
              </div>
              <span className={`${ps.fieldHint} ${ps.wide}`}>The gNB runs on the host network and listens on this address; the core must reach it, and UEs dial it too.</span>
            </>
          )}
          {external && !onSaved && (
            <>
              <Field label="AMF N2 IP" value={draft.amfIp ?? ''} onChange={(v) => set({ amfIp: v })} pattern={IP} conflict={conflict('amfIp')} />
              <Field label="AMF port" type="number" value={draft.amfPort ?? ''} onChange={(v) => set({ amfPort: v === '' ? undefined : Number(v) })} hint="empty is 38412" conflict={conflict('amfPort')} />
              <Field label="UPF N3 IP" value={draft.upfIp ?? ''} onChange={(v) => set({ upfIp: v })} pattern={IP} conflict={conflict('upfIp')} />
              <Field label="UPF port" type="number" value={draft.upfPort ?? ''} onChange={(v) => set({ upfPort: v === '' ? undefined : Number(v) })} hint="empty is 2152" conflict={conflict('upfPort')} />
              <div className={ps.field}>
                <label htmlFor="gnb-iface">Host interface</label>
                <select id="gnb-iface" className={`${ps.input} ${ifaceMissing || conflict('hostInterface') ? ps.inputBad : ''}`} value={draft.hostInterface ?? ''} onChange={(e) => pickInterface(e.target.value)}>
                  <option value="">{!hosts.loaded ? 'loading…' : 'pick an interface'}</option>
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
          <div className={`${ps.field} ${ps.wide}`}>
            <label htmlFor="gnb-radio">Radio range</label>
            <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
              <label style={{ display: 'inline-flex', alignItems: 'center', gap: '0.375rem', fontSize: '0.75rem' }}>
                <input type="checkbox" checked={!draft.radioRange} onChange={(e) => set({ radioRange: e.target.checked ? undefined : 500 })} />
                Unlimited
              </label>
              {Boolean(draft.radioRange) && (
                <>
                  <NumberInput id="gnb-radio" className={ps.input} style={{ width: '8rem' }} min={1} max={1000000} value={draft.radioRange ?? 500} onChange={(v) => set({ radioRange: v })} />
                  <span style={{ fontSize: '0.75rem' }}>metres</span>
                </>
              )}
            </div>
            <span className={ps.fieldHint}>{draft.radioRange
              ? `Signal on the map: 100% at the gNB, 1% at ${draft.radioRange} m, then falling with the square of the distance (never 0). The UE stays attached whatever the map shows.`
              : 'Signal on the map is 100% everywhere, so this gNB never hands a UE over.'}</span>
          </div>
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
