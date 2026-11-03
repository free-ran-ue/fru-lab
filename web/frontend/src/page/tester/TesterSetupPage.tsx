import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import AppLayout from '../../components/layout/AppLayout'
import Modal from '../../components/modal/modal'
import Button from '../../components/button/button'
import NotificationContainer from '../../components/notifications/NotificationContainer'
import { useNotifications } from '../../hooks/useNotifications'
import { api, extractErrorMessage } from '../../apiClient'
import type { TesterFieldError, TesterInterface, TesterPingRequestPlaneEnum, TesterPlan, TesterProfile, TesterStoredProfile, TesterValidateResponse } from '../../api'
import { DEFAULT_TESTER_ID, DEFAULT_TESTER_PROFILE, forTester, normalizeProfile } from './testerDefaults'
import { useTesters } from './useTesters'
import { formatBps } from './testerFormat'
import styles from './setup.module.css'
import PingTest, { type PingOutcomeSummary } from './PingTest'
import InterfaceField from './InterfaceField'
import ProfileNameField from './ProfileNameField'
import { byName, freeName, lastProfileId, nameOwner, rememberProfileId } from './testerProfiles'
import Pager from '../../components/pager/Pager'
import { pageOf } from '../../components/pager/paging'

// PREVIEW_PAGE is how many gNBs one page of the plan preview shows.
const PREVIEW_PAGE = 10

// totalRate is the whole run's offered load for one direction.
function totalRate(mbps: number, ues: number, packetSize: number): string {
  if (!mbps || !ues || !packetSize) return 'off'
  const bps = mbps * 1e6 * ues
  return `${formatBps(bps)} · ${Math.round(bps / (packetSize * 8)).toLocaleString()} pps`
}

// about renders an estimate in seconds as "≈ 42 s" or "≈ 3m 20s".
function about(sec: number): string {
  if (!Number.isFinite(sec) || sec <= 0) return '—'
  if (sec < 60) return `≈ ${Math.max(1, Math.round(sec))} s`
  const m = Math.floor(sec / 60)
  const rest = Math.round(sec % 60)
  return m >= 60 ? `≈ ${Math.floor(m / 60)}h ${m % 60}m` : `≈ ${m}m ${rest}s`
}

// offered is one direction's offered load at full scale.
function offered(mbps: number, ues: number): string {
  return mbps > 0 && ues > 0 ? formatBps(mbps * 1e6 * ues) : 'off'
}

const PLANES: { plane: TesterPingRequestPlaneEnum, name: string, peer: string }[] = [
  { plane: 'n2', name: 'N2', peer: 'AMF' },
  { plane: 'n3', name: 'N3', peer: 'UPF' },
  { plane: 'n6', name: 'N6', peer: 'UPF' },
]

interface Check {
  // true passed, false warning, null not run yet
  ok: boolean | null
  title: string
  detail: string
}

function CheckIcon({ ok }: { ok: boolean | null }) {
  if (ok === true) {
    return <svg viewBox="0 0 16 16" fill="none" strokeWidth="2" className={styles.iconOk} role="img" aria-label="Passed"><path d="M3 8.5l3 3 7-7" /></svg>
  }
  if (ok === false) {
    return <svg viewBox="0 0 16 16" fill="none" strokeWidth="2" className={styles.iconWarn} role="img" aria-label="Warning"><path d="M8 3v6M8 12v.5" /></svg>
  }
  return <svg viewBox="0 0 16 16" fill="none" strokeWidth="1.5" className={styles.iconIdle} role="img" aria-label="Not checked yet"><circle cx="8" cy="8" r="5.5" /></svg>
}

// lastSupi is the SUPI of the plan's last UE (the last gNB may own none).
function lastSupi(plan: TesterPlan): string {
  for (let i = plan.gnbs.length - 1; i >= 0; i--) {
    if (plan.gnbs[i].lastSupi) return plan.gnbs[i].lastSupi
  }
  return ''
}

// Sets a value at a dotted path ("network.n2.cidr") on a deep copy, so
// one onChange handler serves every field.
function setPath(profile: TesterProfile, path: string, value: string | number): TesterProfile {
  const next = structuredClone(profile) as unknown as Record<string, unknown>
  const keys = path.split('.')
  let node = next
  for (const key of keys.slice(0, -1)) node = node[key] as Record<string, unknown>
  node[keys[keys.length - 1]] = value
  return next as unknown as TesterProfile
}

// RenameRequest asks for a new name before saving as a new profile;
// then says whether to start a run afterwards.
interface RenameRequest {
  name: string
  then: 'save' | 'start'
}

function getPath(profile: TesterProfile, path: string): string | number {
  return path.split('.').reduce<unknown>((node, key) => (node as Record<string, unknown>)[key], profile) as string | number
}

interface FieldProps {
  label: string
  path: string
  profile: TesterProfile
  errors: TesterFieldError[]
  numeric?: boolean
  options?: string[] // render a select instead of an input
  onChange: (path: string, value: string | number) => void
}

function Field({ label, path, profile, errors, numeric = false, options, onChange }: FieldProps) {
  const message = errors.find((e) => e.field === path)?.message
  const value = getPath(profile, path)
  const className = `${styles.in} ${message ? styles.inError : ''}`
  return (
    <div className={styles.fld}>
      <label htmlFor={path}>{label}</label>
      {options ? (
        <select id={path} className={className} value={value} onChange={(e) => onChange(path, e.target.value)}>
          {options.map((o) => <option key={o} value={o}>{o}</option>)}
        </select>
      ) : (
        <input
          id={path}
          className={className}
          type={numeric ? 'number' : 'text'}
          value={numeric && Number.isNaN(value) ? '' : value}
          onChange={(e) => onChange(path, numeric ? e.target.valueAsNumber : e.target.value)}
        />
      )}
      {message && <p className={styles.fieldError}>{message}</p>}
    </div>
  )
}

// RateFields renders one per-UE stage's pacing knobs.
function RateFields({ stage, fieldProps }: { stage: 'registration' | 'pdu' | 'deregistration', fieldProps: Omit<FieldProps, 'label' | 'path'> }) {
  return (
    <>
      <Field label="Starts per second" path={`rates.${stage}.ratePerSec`} numeric {...fieldProps} />
      <Field label="Max in flight" path={`rates.${stage}.maxInFlight`} numeric {...fieldProps} />
      <Field label="Timeout per attempt (ms)" path={`rates.${stage}.timeoutMs`} numeric {...fieldProps} />
      <Field label="Retries" path={`rates.${stage}.retries`} numeric {...fieldProps} />
    </>
  )
}

export default function TesterSetupPage() {
  const navigate = useNavigate()
  const { errors, successes, addError, addSuccess, removeNotification } = useNotifications()
  const [searchParams, setSearchParams] = useSearchParams()
  const [profile, setProfile] = useState<TesterProfile>(DEFAULT_TESTER_PROFILE)
  // the saved profiles (null while loading), the one open (null: not saved
  // yet) and what it looked like when last opened or saved
  const [profiles, setProfiles] = useState<TesterStoredProfile[] | null>(null)
  const [currentId, setCurrentId] = useState<string | null>(null)
  const [savedJson, setSavedJson] = useState('')
  const [isChoosingSave, setIsChoosingSave] = useState(false)
  const [rename, setRename] = useState<RenameRequest | null>(null)
  const [previewPage, setPreviewPage] = useState(1)
  const [isLoading, setIsLoading] = useState(true)
  const [validation, setValidation] = useState<TesterValidateResponse | null>(null)
  const [isBusy, setIsBusy] = useState(false)
  const validateSeq = useRef(0)
  // the last ping test per plane, for the pre-flight checks
  const [pings, setPings] = useState<Partial<Record<TesterPingRequestPlaneEnum, PingOutcomeSummary>>>({})
  const handlePing = useCallback((plane: TesterPingRequestPlaneEnum, outcome: PingOutcomeSummary) => {
    setPings((cur) => ({ ...cur, [plane]: outcome }))
  }, [])
  // the host's interfaces for the N2/N3/N6 menus: null while loading,
  // undefined when they could not be listed (the fields fall back to text)
  const [interfaces, setInterfaces] = useState<TesterInterface[] | null | undefined>(null)
  // the testers to choose from; the profile names the one that runs it
  const { testers } = useTesters()
  const testerId = profile.testerId || DEFAULT_TESTER_ID
  const tester = testers?.find((t) => t.id === testerId)
  // false: the profile's tester was removed from fru-lab
  const testerKnown = testers === null ? null : tester !== undefined

  const loadInterfaces = useCallback(() => {
    if (testerKnown === false) {
      setInterfaces(undefined)
      return
    }
    setInterfaces(null)
    api.testerByIdNetworkInterfaces(testerId)
      .then((response) => setInterfaces(response.data))
      .catch((error) => {
        setInterfaces(undefined)
        addError(extractErrorMessage(error, "Failed to list the tester host's interfaces"))
      })
  }, [addError, testerId, testerKnown])

  useEffect(() => { loadInterfaces() }, [loadInterfaces])

  // Open the profile the URL names (?profile=<id>, or ?new for a new
  // one), else the one this browser had open last, else the first saved
  // one, else a new one; then drop the query so a reload keeps the choice
  // made here.
  useEffect(() => {
    const wanted = searchParams.has('new') ? null : searchParams.get('profile') ?? lastProfileId()
    api.testerProfileList()
      .then((response) => {
        const list = response.data
        setProfiles(list)
        const found = list.find((p) => p.id === wanted) ?? (searchParams.has('new') ? undefined : list[0])
        if (found) openProfile(found)
        else openNew(list)
      })
      .catch((error) => {
        setProfiles([])
        addError(extractErrorMessage(error, 'Failed to load the saved profiles'))
      })
      .finally(() => {
        setIsLoading(false)
        setSearchParams({}, { replace: true })
      })
    // once, on arrival
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  function openProfile(stored: TesterStoredProfile) {
    const p = normalizeProfile(stored.profile)
    setProfile(p)
    setSavedJson(JSON.stringify(p))
    setCurrentId(stored.id)
    rememberProfileId(stored.id)
  }

  function openNew(list: TesterStoredProfile[]) {
    const p = { ...structuredClone(DEFAULT_TESTER_PROFILE), name: freeName(list, DEFAULT_TESTER_PROFILE.name) }
    setProfile(p)
    setSavedJson(JSON.stringify(p))
    setCurrentId(null)
    // the remembered profile stays: Cancel and a reload go back to it
  }

  // handleCancel drops the new profile: back to the profile open before
  // it, or to the Profiles page if there was none.
  function handleCancel() {
    if (!leave()) return
    const previous = list.find((p) => p.id === lastProfileId())
    if (previous) openProfile(previous)
    else navigate('/tester/profiles')
  }

  // leave asks before dropping unsaved edits.
  function leave(): boolean {
    return !isDirty || window.confirm(`Discard the unsaved changes to "${profile.name}"?`)
  }

  // Re-validate 400 ms after the last edit; a sequence number drops
  // answers that arrive after a newer request was sent.
  useEffect(() => {
    if (isLoading) return
    const seq = ++validateSeq.current
    if (testerKnown === false) {
      setValidation(null)
      return
    }
    const timer = window.setTimeout(() => {
      api.testerByIdProfileValidate(profile.testerId || DEFAULT_TESTER_ID, forTester(profile))
        .then((response) => {
          if (seq === validateSeq.current) setValidation(response.data)
        })
        .catch((error) => {
          if (seq === validateSeq.current) {
            setValidation(null)
            addError(extractErrorMessage(error, 'Failed to validate the profile'))
          }
        })
    }, 400)
    return () => window.clearTimeout(timer)
  }, [profile, isLoading, addError, testerKnown])

  function handleChange(path: string, value: string | number) {
    setProfile((prev) => {
      const next = setPath(prev, path, value)
      // "every UE carries traffic" keeps holding when the UE count changes
      if (path === 'scale.ueCount' && prev.scale.throughputUeCount === prev.scale.ueCount) {
        next.scale.throughputUeCount = next.scale.ueCount
      }
      return next
    })
  }

  const isDirty = JSON.stringify(profile) !== savedJson
  const cpUes = Math.max(0, (profile.scale.ueCount || 0) - (profile.scale.throughputUeCount || 0))
  const list = profiles ?? []
  const savedName = list.find((p) => p.id === currentId)?.profile.name ?? ''
  const nameError = profile.name.trim() === ''
    ? 'Give the profile a name'
    : nameOwner(list, profile.name, currentId) ? 'Another saved profile already has this name' : undefined

  // stored puts a created or updated profile into the list and opens it.
  function stored(p: TesterStoredProfile) {
    setProfiles((cur) => byName([...(cur ?? []).filter((x) => x.id !== p.id), p]))
    openProfile(p)
  }

  // saveNew saves the form as a new profile named name, first asking for
  // another name if a saved profile already has it.
  async function saveNew(name: string, then: RenameRequest['then']) {
    if (name.trim() === '' || nameOwner(list, name, null)) {
      setRename({ name: freeName(list, name.trim() ? `${name.trim()} copy` : 'profile'), then })
      return
    }
    setIsBusy(true)
    try {
      const response = await api.testerProfileCreate({ ...profile, name: name.trim() })
      stored(response.data)
      if (then === 'start') {
        await start(normalizeProfile(response.data.profile))
        return
      }
      addSuccess(`Saved as new profile "${name.trim()}"`)
    } catch (error) {
      addError(extractErrorMessage(error, 'Failed to save the profile'))
    }
    setIsBusy(false)
  }

  // update saves the form over the open profile.
  async function update(): Promise<boolean> {
    if (currentId === null) return false
    try {
      const response = await api.testerProfileUpdate(currentId, profile)
      stored(response.data)
      return true
    } catch (error) {
      addError(extractErrorMessage(error, 'Failed to update the profile'))
      return false
    }
  }

  function handleSave() {
    if (currentId === null) void saveNew(profile.name, 'save')
    else setIsChoosingSave(true)
  }

  async function handleUpdate() {
    setIsChoosingSave(false)
    setIsBusy(true)
    if (await update()) addSuccess(`Updated profile "${profile.name.trim()}"`)
    setIsBusy(false)
  }

  function handleSaveAsNew() {
    setIsChoosingSave(false)
    void saveNew(profile.name, 'save')
  }

  function handleRename() {
    if (!rename) return
    setRename(null)
    void saveNew(rename.name, rename.then)
  }

  // start runs p on its tester, then follows it on that tester's run page.
  async function start(p: TesterProfile) {
    const id = p.testerId || DEFAULT_TESTER_ID
    try {
      await api.testerByIdRunStart(id, forTester(p))
      navigate(`/tester/run/${id}`)
    } catch (error) {
      addError(extractErrorMessage(error, 'Failed to start the run'))
      setIsBusy(false)
    }
  }

  // handleStart saves first: over the open profile, or as a new one.
  async function handleStart() {
    if (currentId === null) {
      void saveNew(profile.name, 'start')
      return
    }
    setIsBusy(true)
    if (isDirty && !(await update())) {
      setIsBusy(false)
      return
    }
    await start(profile)
  }

  const renameError = rename && (rename.name.trim() === ''
    ? 'Give the profile a name'
    : nameOwner(list, rename.name, null) ? 'A saved profile already has this name' : undefined)

  const fieldErrors = validation?.errors ?? []
  const plan = validation?.valid ? validation.plan : null
  const fieldProps = { profile, errors: fieldErrors, onChange: handleChange }

  // the plan preview, worked out from the form
  const tpUes = profile.scale.throughputUeCount || 0
  const regSec = tpUes / (profile.rates.registration.ratePerSec || NaN)
  const pduSec = Math.max(regSec, tpUes / (profile.rates.pdu.ratePerSec || NaN))
  const preview: [string, string][] = [
    ['UEs', `${(profile.scale.ueCount || 0).toLocaleString()} on ${(profile.scale.gnbCount || 0).toLocaleString()} gNB${profile.scale.gnbCount === 1 ? '' : 's'}`],
    ['Throughput UEs', tpUes.toLocaleString()],
    ['All registered after', about(regSec)],
    ['All PDU sessions after', about(pduSec)],
    ['Offered uplink', offered(profile.traffic.ulMbps, tpUes)],
    ['Offered downlink', offered(profile.traffic.dlMbps, tpUes)],
    ['Stops after', profile.traffic.maxDurationMin > 0 ? `${profile.traffic.maxDurationMin} min` : 'Stop (no limit)'],
  ]
  if (cpUes > 0) preview.splice(2, 0, ['Control-plane loop UEs', cpUes.toLocaleString()])

  // pre-flight checks: only what this page already knows
  const checks: Check[] = []
  if (testerKnown !== null) {
    checks.push(testerKnown === false
      ? { ok: false, title: 'Tester removed', detail: 'choose another next to Start run' }
      : { ok: tester?.reachable ?? null, title: `${tester?.name ?? testerId} answers`, detail: tester?.reachable === false ? 'status request failed' : tester?.url ?? '' })
  }
  checks.push(!validation
    ? { ok: null, title: 'Profile is valid', detail: 'checking…' }
    : validation.valid
      ? { ok: true, title: 'Profile is valid', detail: plan ? `${plan.gnbs.length} gNBs, up to ${plan.uesPerGnb} UEs each` : '' }
      : { ok: false, title: 'Profile has errors', detail: `${fieldErrors.length} field${fieldErrors.length === 1 ? '' : 's'} to fix` })
  if (interfaces) {
    for (const { plane, name } of PLANES) {
      const ifName = profile.network[plane].interface
      const found = interfaces.find((i) => i.name === ifName)
      checks.push(!ifName
        ? { ok: false, title: `${name} interface`, detail: 'not chosen' }
        : found
          ? { ok: found.state === 'up' || found.state === 'unknown', title: `${ifName} on the tester host`, detail: `${name} · ${found.state} · MTU ${found.mtu}` }
          : { ok: false, title: `${ifName} not on the tester host`, detail: `${name} interface` })
    }
  }
  for (const { plane, name, peer } of PLANES) {
    const r = pings[plane]
    checks.push(r
      ? { ok: r.ok, title: `${peer} ${r.ok ? 'answers' : r.title} on ${name}`, detail: r.detail }
      : { ok: null, title: `${peer} on ${name}`, detail: `not tested: Ping ${peer} in the ${name} section` })
  }

  const network = (plane: 'n2' | 'n3' | 'n6', title: string, hint: string, children: ReactNode, foot: ReactNode, ping: string) => (
    <section className={styles.sec}>
      <div className={styles.secH}><h2>{title}</h2><span className={styles.hint}>{hint}</span></div>
      <div className={styles.secB}>
        <InterfaceField label="Local interface" path={`network.${plane}.interface`} value={profile.network[plane].interface}
          errors={fieldErrors} interfaces={interfaces} onChange={handleChange} onRefresh={loadInterfaces} />
        {children}
      </div>
      <p className={styles.secFoot}>{foot}</p>
      <PingTest plane={plane} label={ping} profile={profile} testerId={testerId} onOutcome={handlePing} />
    </section>
  )

  const headerActions = (
    <div className={styles.actions}>
      {!isLoading && currentId === null && (
        <button type="button" className={styles.btn} onClick={handleCancel} disabled={isBusy}>Cancel</button>
      )}
      <button type="button" className={styles.btn} onClick={handleSave} disabled={isLoading || isBusy}>Save</button>
      <label className={`${styles.joined} ${testerKnown === false ? styles.joinedError : ''}`}>
        <span>Tester</span>
        <select aria-label="Tester to run on"
          value={testerId} disabled={isLoading || testers === null}
          title={tester ? `Run on ${tester.name} (${tester.url}); saved with the profile` : "This profile's tester was removed from fru-lab: choose another"}
          onChange={(e) => handleChange('testerId', e.target.value)}>
          {testerKnown === false && <option value={testerId}>Removed tester</option>}
          {testers === null && <option value={testerId}>Loading testers…</option>}
          {(testers ?? []).map((t) => (
            <option key={t.id} value={t.id}>{t.name}{t.reachable === false ? ' (unreachable)' : ''}</option>
          ))}
        </select>
      </label>
      <button type="button" className={`${styles.btn} ${styles.btnPrimary}`} onClick={handleStart}
        disabled={isLoading || isBusy || testerKnown === false || !validation?.valid || (currentId !== null && nameError !== undefined)}>
        <svg viewBox="0 0 12 12" aria-hidden="true"><path d="M3 1.5v9l7-4.5z" fill="currentColor" /></svg>
        {isBusy ? 'Starting…' : 'Start run'}
      </button>
    </div>
  )

  return (
    <AppLayout
      crumbs={[{ label: 'Setup' }, { label: isLoading ? '…' : profile.name.trim() || 'unnamed profile' }]}
      crumbExtra={!isLoading && (currentId === null || isDirty) && (
        <span className={styles.unsaved} title={currentId === null ? 'New profile, not saved yet' : 'Unsaved changes'}>UNSAVED</span>
      )}
      headerActions={headerActions}
      overlay={<NotificationContainer errors={errors} successes={successes} onClose={removeNotification} />}>
      {testerKnown === false && !isLoading && (
        <p className={styles.runError}>This profile&apos;s tester was removed from fru-lab: choose another one next to Start run.</p>
      )}

      <div className={styles.cols}>
        <div className={styles.plan}>
          <div className={styles.titleBlock}>
            <h1 className={styles.title}>Test plan</h1>
            <p className={styles.subtitle}>What the tester simulates: how many UEs, how fast they attach, and the traffic each one carries.</p>
          </div>

          {isLoading ? (
            <p className={styles.loading}>Loading profile…</p>
          ) : (
            <>
              <section className={styles.sec}>
                <div className={styles.secH}><h2>Scale</h2><span className={styles.hint}>profile, UEs and gNBs</span></div>
                <div className={styles.secB}>
                  <ProfileNameField value={profile.name} profiles={profiles} currentId={currentId}
                    message={nameError ?? fieldErrors.find((e) => e.field === 'name')?.message}
                    onChange={(name) => handleChange('name', name)}
                    onPick={(p) => { if (p.id !== currentId && leave()) openProfile(p) }}
                    onNew={() => { if (leave()) openNew(list) }} />
                  <Field label="gNB count" path="scale.gnbCount" numeric {...fieldProps} />
                  <Field label="UE count" path="scale.ueCount" numeric {...fieldProps} />
                  <Field label="Throughput UEs" path="scale.throughputUeCount" numeric {...fieldProps} />
                </div>
                <p className={styles.secFoot}>
                  UEs fill gNBs in order: each gNB takes up to {plan ? plan.uesPerGnb : '⌈UEs ÷ gNBs⌉'} UEs and the last one may be partly filled.
                  {' '}{cpUes > 0
                    ? <>UEs #1–{profile.scale.throughputUeCount} establish a PDU session and carry traffic; the other <b>{cpUes.toLocaleString()}</b> are the control-plane test: once every throughput UE is done they register and deregister in a loop until Stop, with no PDU session.</>
                    : <>Every UE carries traffic; lower Throughput UEs to run a control-plane test with the rest.</>}
                </p>
              </section>

              <section className={styles.sec}>
                <div className={styles.secH}><h2>gNB template</h2><span className={styles.hint}>applied to every gNB</span></div>
                <div className={styles.secB}>
                  <Field label="First gNB ID (hex)" path="gnb.gnbIdStart" {...fieldProps} />
                  <Field label="Name pattern ({i} = index)" path="gnb.namePattern" {...fieldProps} />
                  <Field label="MCC" path="gnb.mcc" {...fieldProps} />
                  <Field label="MNC" path="gnb.mnc" {...fieldProps} />
                  <Field label="TAC (hex)" path="gnb.tac" {...fieldProps} />
                  <Field label="SST" path="gnb.sst" numeric {...fieldProps} />
                  <Field label="SD (hex, optional)" path="gnb.sd" {...fieldProps} />
                </div>
              </section>

              <section className={styles.sec}>
                <div className={styles.secH}><h2>UE template</h2><span className={styles.hint}>applied to every UE</span></div>
                <div className={styles.secB}>
                  <Field label="First MSIN" path="ue.msinStart" {...fieldProps} />
                  <Field label="Key (K)" path="ue.key" {...fieldProps} />
                  <Field label="OPc" path="ue.opc" {...fieldProps} />
                  <Field label="AMF" path="ue.amf" {...fieldProps} />
                  <Field label="SQN" path="ue.sqn" {...fieldProps} />
                  <Field label="Integrity" path="ue.integrity" options={['nia0', 'nia1', 'nia2', 'nia3']} {...fieldProps} />
                  <Field label="Ciphering" path="ue.ciphering" options={['nea0', 'nea1', 'nea2', 'nea3']} {...fieldProps} />
                  <Field label="DNN" path="ue.dnn" {...fieldProps} />
                  <Field label="SST" path="ue.sst" numeric {...fieldProps} />
                  <Field label="SD (hex, optional)" path="ue.sd" {...fieldProps} />
                </div>
                <p className={styles.secFoot}>
                  UEs use the gNB template&apos;s PLMN; the MSIN is incremented per UE. The tester does not create subscribers: add
                  {plan && plan.gnbs.length > 0
                    ? <> <span className={styles.mono}>{plan.gnbs[0].firstSupi}</span> … <span className={styles.mono}>{lastSupi(plan)}</span></>
                    : ' every UE'}
                  {' '}to the core with these keys and a slice the gNB advertises.
                </p>
              </section>

              <section className={styles.sec}>
                <div className={styles.secH}><h2>Registration pacing</h2><span className={styles.hint}>procedures per second, in flight at once</span></div>
                <div className={styles.secB}>
                  <RateFields stage="registration" fieldProps={fieldProps} />
                </div>
                <p className={styles.secFoot}>Starts per second caps how fast new registrations begin; max in flight caps how many wait for the core at once.</p>
              </section>

              <section className={styles.sec}>
                <div className={styles.secH}><h2>PDU session pacing</h2><span className={styles.hint}>starts after Registration Accept</span></div>
                <div className={styles.secB}>
                  <RateFields stage="pdu" fieldProps={fieldProps} />
                </div>
                <p className={styles.secFoot}>A UE moves on to its PDU session as soon as it is registered.</p>
              </section>

              <section className={styles.sec}>
                <div className={styles.secH}><h2>Deregistration pacing</h2><span className={styles.hint}>on Stop</span></div>
                <div className={styles.secB}>
                  <RateFields stage="deregistration" fieldProps={fieldProps} />
                </div>
                <p className={styles.secFoot}>After Stop, every UE that registered deregisters; the core releases its PDU session with it. Then each gNB&apos;s SCTP association is closed.</p>
              </section>

              <section className={styles.sec}>
                <div className={styles.secH}><h2>N2 setup timing</h2><span className={styles.hint}>NG Setup per gNB</span></div>
                <div className={styles.secB}>
                  <Field label="Timeout per attempt (ms)" path="rates.n2.timeoutMs" numeric {...fieldProps} />
                  <Field label="Retries" path="rates.n2.retries" numeric {...fieldProps} />
                </div>
                <p className={styles.secFoot}>A failed attempt with retries left goes to the back of the queue.</p>
              </section>

              <section className={styles.sec}>
                <div className={styles.secH}>
                  <h2>Control-plane loop</h2>
                  {cpUes === 0 ? <span className={styles.offTag}>off: no control-plane UEs</span> : <span className={styles.hint}>register, hold, deregister, repeat</span>}
                </div>
                <div className={styles.secB}>
                  <Field label="Registrations per second" path="rates.cpLoop.ratePerSec" numeric {...fieldProps} />
                  <Field label="Max UEs in a cycle" path="rates.cpLoop.maxInFlight" numeric {...fieldProps} />
                  <Field label="Timeout per procedure (ms)" path="rates.cpLoop.timeoutMs" numeric {...fieldProps} />
                  <Field label="Stay registered (ms)" path="rates.cpLoop.holdMs" numeric {...fieldProps} />
                </div>
                <p className={styles.secFoot}>
                  Each cycle registers a control-plane UE (full authentication), keeps it registered, then deregisters it, and the UE queues for its next cycle.
                  A failed procedure is counted with its cause and the UE simply tries again later. Max UEs in a cycle includes the time they stay registered.
                </p>
              </section>

              <section className={styles.sec}>
                <div className={styles.secH}><h2>Traffic per UE</h2><span className={styles.hint}>data plane</span></div>
                <div className={styles.secB}>
                  <Field label="Uplink (Mbps)" path="traffic.ulMbps" numeric {...fieldProps} />
                  <Field label="Downlink (Mbps)" path="traffic.dlMbps" numeric {...fieldProps} />
                  <Field label="Uplink packet size (bytes, inner IP)" path="traffic.ulPacketSize" numeric {...fieldProps} />
                  <Field label="Downlink packet size (bytes, inner IP)" path="traffic.dlPacketSize" numeric {...fieldProps} />
                  <Field label="Data plane engine" path="traffic.engine" options={['auto', 'socket', 'afxdp']} {...fieldProps} />
                  <Field label="UDP port" path="traffic.port" numeric {...fieldProps} />
                  <Field label="Max run time (min, 0 = no limit)" path="traffic.maxDurationMin" numeric {...fieldProps} />
                  <Field label="Downlink batch (ms per UE, 0 = off)" path="traffic.dlBatchMs" numeric {...fieldProps} />
                </div>
                <p className={styles.secFoot}>
                  Every throughput UE starts sending as soon as its PDU session is up. At full scale: uplink
                  {' '}<span className={styles.mono}>{totalRate(profile.traffic.ulMbps, profile.scale.throughputUeCount, profile.traffic.ulPacketSize)}</span>,
                  downlink <span className={styles.mono}>{totalRate(profile.traffic.dlMbps, profile.scale.throughputUeCount, profile.traffic.dlPacketSize)}</span>.
                  0 turns a direction off. Each direction has its own packet size, which can go up to the network&apos;s MTU minus 44 bytes of GTP-U (1456 at
                  MTU 1500); bigger packets carry more for the same packets per second.
                  {' '}With a max run time the run stops itself, exactly as if you pressed Stop.
                  {' '}Downlink batch sends each UE that many milliseconds of packets in a row, so the kernel can take them in one
                  send (UDP GSO); larger saves CPU but makes each UE&apos;s downlink burstier.
                </p>
                <p className={styles.secFoot}>
                  <b>Data plane engine</b>: <span className={styles.mono}>socket</span> sends and receives through the
                  kernel&apos;s UDP sockets; <span className={styles.mono}>afxdp</span> moves whole frames through AF_XDP on the N3 and
                  N6 interfaces, skipping the kernel&apos;s IP and UDP code, for far less CPU per packet (zero-copy when the NIC
                  driver supports it); <span className={styles.mono}>auto</span> uses AF_XDP when both interfaces are NICs and falls
                  back to sockets otherwise. The Run page shows which one ran.
                </p>
              </section>

              {network('n2', 'N2', 'gNB ↔ AMF', (
                <>
                  <Field label="gNB IP CIDR" path="network.n2.cidr" {...fieldProps} />
                  <Field label="First gNB IP" path="network.n2.startIp" {...fieldProps} />
                  <Field label="AMF IP" path="network.n2.amfIp" {...fieldProps} />
                  <Field label="AMF port" path="network.n2.amfPort" numeric {...fieldProps} />
                </>
              ), 'fru-tester adds one IP per gNB to this interface when the run starts and removes them when it stops. IPs already on the host and the AMF IP are skipped.', 'Ping AMF')}

              {network('n3', 'N3', 'gNB ↔ UPF', (
                <>
                  <Field label="gNB IP CIDR" path="network.n3.cidr" {...fieldProps} />
                  <Field label="First gNB IP" path="network.n3.startIp" {...fieldProps} />
                  <Field label="UPF IP" path="network.n3.upfIp" {...fieldProps} />
                  <Field label="UPF port" path="network.n3.upfPort" numeric {...fieldProps} />
                </>
              ), 'Each gNB gets its own N3 IP: uplink leaves from it and the UPF sends downlink to it.', 'Ping UPF (N3)')}

              {network('n6', 'N6', 'data network side', (
                <>
                  <Field label="Sink IP (CIDR)" path="network.n6.sinkIp" {...fieldProps} />
                  <Field label="UPF N6 IP" path="network.n6.upfIp" {...fieldProps} />
                  <Field label="UE IP pool" path="network.n6.uePool" {...fieldProps} />
                </>
              ), (
                <>
                  Uplink leaves the UPF addressed to the sink IP. Enter it with its prefix length, as it goes on the interface
                  (for example 172.26.6.1/16): if the host does not have it, the tester adds it to the interface with that prefix for
                  the run, which also gives the host a route to that subnet. Downlink is sent from it to each UE&apos;s IP; the tester
                  routes the UE pool via the UPF N6 IP for the run. Both are removed afterwards.
                </>
              ), 'Ping UPF (N6)')}

              <section className={styles.sec}>
                <div className={styles.secH}><h2>gNB plan</h2><span className={styles.hint}>addresses and UEs per gNB</span></div>
                {!validation && <p className={styles.planNote}>Checking…</p>}
                {validation && !validation.valid && (
                  <p className={`${styles.planNote} ${styles.planNoteError}`}>Fix the {fieldErrors.length} highlighted field{fieldErrors.length === 1 ? '' : 's'} to see the plan.</p>
                )}
                {plan && (
                  <>
                    <div className={styles.tableWrap}>
                      <table className={styles.table}>
                        <thead>
                          <tr>
                            <th className={styles.th}>#</th><th className={styles.th}>Name</th><th className={styles.th}>gNB ID</th>
                            <th className={styles.th}>N2 IP</th><th className={styles.th}>N3 IP</th><th className={styles.th}>UEs</th><th className={styles.th}>SUPIs</th>
                          </tr>
                        </thead>
                        <tbody>
                          {pageOf(plan.gnbs, previewPage, PREVIEW_PAGE).items.map((g) => (
                            <tr key={g.index}>
                              <td className={`${styles.td} num`}>{g.index}</td>
                              <td className={styles.td}>{g.name}</td>
                              <td className={`${styles.td} num`}>{g.gnbId}</td>
                              <td className={`${styles.td} num`}>{g.n2Ip}/{plan.n2Prefix}</td>
                              <td className={`${styles.td} num`}>{g.n3Ip}/{plan.n3Prefix}</td>
                              <td className={`${styles.td} num`}>{g.ueCount ? `${g.ueCount} (#${g.ueFirst}–${g.ueLast})` : '0'}{g.cpUeCount > 0 && <span className={styles.pingNote}> · {g.cpUeCount} control-plane</span>}</td>
                              <td className={`${styles.td} num`}>{g.ueCount ? `${g.firstSupi} … ${g.lastSupi.slice(-4)}` : '—'}</td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                    {plan.gnbs.length > PREVIEW_PAGE && (
                      <div className={styles.pager}>
                        <Pager page={pageOf(plan.gnbs, previewPage, PREVIEW_PAGE)} total={plan.gnbs.length} onChange={setPreviewPage} />
                      </div>
                    )}
                  </>
                )}
              </section>
            </>
          )}
        </div>

        <aside aria-label="Plan preview" className={styles.aside}>
          <section className={styles.sec}>
            <div className={styles.secH}><h2>Plan preview</h2></div>
            <div className={styles.kv}>
              {preview.map(([k, v]) => (
                <div key={k} className={styles.kvRow}><span>{k}</span><span className="num">{v}</span></div>
              ))}
              <p className={styles.kvNote}>Times assume the core keeps up with the configured rates.</p>
            </div>
          </section>
          <section className={styles.sec}>
            <div className={styles.secH}><h2>Pre-flight checks</h2></div>
            <div className={styles.checks}>
              {checks.map((c, i) => (
                <div key={i} className={styles.check}>
                  <CheckIcon ok={c.ok} />
                  <div className={styles.checkText}><span>{c.title}</span><span className={styles.hint}>{c.detail}</span></div>
                </div>
              ))}
            </div>
          </section>
        </aside>
      </div>

      <Modal
        isOpen={isChoosingSave}
        onClose={() => setIsChoosingSave(false)}
        title="Save profile"
        onSubmit={handleUpdate}
        submitLabel="Update current profile"
        submitDisabled={nameError !== undefined}
        actions={<Button variant="secondary" onClick={handleSaveAsNew}>Save as new</Button>}
      >
        <p className={styles.modalText}>
          <b>Update current profile</b> overwrites &quot;{savedName}&quot;{!sameTrimmed(savedName, profile.name) && <> and renames it to &quot;{profile.name.trim()}&quot;</>}.
          {' '}<b>Save as new</b> keeps &quot;{savedName}&quot; as it was and saves these settings as another profile.
        </p>
        {nameError && <p className={styles.fieldError}>{nameError}; rename it or save as new.</p>}
      </Modal>

      <Modal
        isOpen={rename !== null}
        onClose={() => setRename(null)}
        title="Name the new profile"
        onSubmit={handleRename}
        submitLabel={rename?.then === 'start' ? 'Save and start' : 'Save'}
        submitDisabled={renameError !== undefined}
      >
        <form onSubmit={(e) => { e.preventDefault(); if (!renameError) handleRename() }}>
          <p className={styles.modalText}>
            {profile.name.trim() === '' ? 'The profile has no name yet.' : <>A saved profile is already named &quot;{profile.name.trim()}&quot;.</>}
            {' '}Choose a name for the new one.
          </p>
          <div className={styles.fld}>
            <label htmlFor="rename">Profile name</label>
            <input id="rename" className={`${styles.in} ${renameError ? styles.inError : ''}`} type="text" autoFocus
              value={rename?.name ?? ''} onChange={(e) => setRename((r) => r && { ...r, name: e.target.value })} />
            {renameError && <p className={styles.fieldError}>{renameError}</p>}
          </div>
        </form>
      </Modal>
    </AppLayout>
  )
}

function sameTrimmed(a: string, b: string): boolean {
  return a.trim() === b.trim()
}
