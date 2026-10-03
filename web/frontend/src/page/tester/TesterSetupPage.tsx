import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import Sidebar from '../../components/sidebar/Sidebar'
import Button from '../../components/button/button'
import NotificationContainer from '../../components/notifications/NotificationContainer'
import { useNotifications } from '../../hooks/useNotifications'
import { api, extractErrorMessage } from '../../apiClient'
import type { TesterFieldError, TesterPlan, TesterProfile, TesterValidateResponse } from '../../api'
import { DEFAULT_TESTER_PROFILE, normalizeProfile } from './testerDefaults'
import { formatBps } from './testerFormat'
import styles from './tester.module.css'
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
  const className = `${styles.input} ${message ? styles.inputError : ''}`
  return (
    <div className={styles.field}>
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
  const [profile, setProfile] = useState<TesterProfile>(DEFAULT_TESTER_PROFILE)
  const [previewPage, setPreviewPage] = useState(1)
  const [isLoading, setIsLoading] = useState(true)
  const [validation, setValidation] = useState<TesterValidateResponse | null>(null)
  const [isBusy, setIsBusy] = useState(false)
  const validateSeq = useRef(0)

  useEffect(() => {
    api.testerProfileGet()
      .then((response) => {
        if (response.status === 200 && response.data) setProfile(normalizeProfile(response.data))
      })
      .catch((error) => addError(extractErrorMessage(error, 'Failed to load the saved profile')))
      .finally(() => setIsLoading(false))
  }, [addError])

  // Re-validate 400 ms after the last edit; a sequence number drops
  // answers that arrive after a newer request was sent.
  useEffect(() => {
    if (isLoading) return
    const seq = ++validateSeq.current
    const timer = window.setTimeout(() => {
      api.testerProfileValidate(profile)
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
  }, [profile, isLoading, addError])

  function handleChange(path: string, value: string | number) {
    setProfile((prev) => setPath(prev, path, value))
  }

  async function handleSave() {
    setIsBusy(true)
    try {
      await api.testerProfilePut(profile)
      addSuccess('Profile saved')
    } catch (error) {
      addError(extractErrorMessage(error, 'Failed to save the profile'))
    } finally {
      setIsBusy(false)
    }
  }

  async function handleStart() {
    setIsBusy(true)
    try {
      await api.testerProfilePut(profile)
      await api.testerRunStart(profile)
      navigate('/tester/run')
    } catch (error) {
      addError(extractErrorMessage(error, 'Failed to start the run'))
      setIsBusy(false)
    }
  }

  const fieldErrors = validation?.errors ?? []
  const plan = validation?.valid ? validation.plan : null
  const fieldProps = { profile, errors: fieldErrors, onChange: handleChange }

  return (
    <div className={styles.layout}>
      <NotificationContainer errors={errors} successes={successes} onClose={removeNotification} />
      <Sidebar />

      <main className={styles.content}>
        <header className={styles.header}>
          <div>
            <h2 className={styles.title}>Throughput Tester · Setup</h2>
            <p className={styles.subtitle}>Describe the gNBs and UEs to simulate. A run brings up N2 for every gNB, registers its UEs and establishes one PDU session each, then holds everything until you stop it.</p>
          </div>
          <div className={styles.headerActions}>
            <Button variant="secondary" onClick={handleSave} disabled={isLoading || isBusy}>Save</Button>
            <Button onClick={handleStart} disabled={isLoading || isBusy || !validation?.valid}>
              {isBusy ? 'Starting…' : 'Start run'}
            </Button>
          </div>
        </header>

        {isLoading ? (
          <p className={styles.emptyState}>Loading profile…</p>
        ) : (
          <>
            <section className={styles.card}>
              <h3 className={styles.cardTitle}>Scale</h3>
              <div className={styles.fieldGrid}>
                <Field label="Profile name" path="name" {...fieldProps} />
                <Field label="gNB count" path="scale.gnbCount" numeric {...fieldProps} />
                <Field label="UE count" path="scale.ueCount" numeric {...fieldProps} />
              </div>
              <p className={styles.hint}>
                UEs fill gNBs in order: each gNB takes up to {plan ? plan.uesPerGnb : '⌈UEs ÷ gNBs⌉'} UEs and the last one may be partly filled.
              </p>
            </section>

            <section className={styles.card}>
              <h3 className={styles.cardTitle}>gNB template</h3>
              <div className={styles.fieldGrid}>
                <Field label="First gNB ID (hex)" path="gnb.gnbIdStart" {...fieldProps} />
                <Field label="Name pattern ({i} = index)" path="gnb.namePattern" {...fieldProps} />
                <Field label="MCC" path="gnb.mcc" {...fieldProps} />
                <Field label="MNC" path="gnb.mnc" {...fieldProps} />
                <Field label="TAC (hex)" path="gnb.tac" {...fieldProps} />
                <Field label="SST" path="gnb.sst" numeric {...fieldProps} />
                <Field label="SD (hex, optional)" path="gnb.sd" {...fieldProps} />
              </div>
            </section>

            <section className={styles.card}>
              <h3 className={styles.cardTitle}>UE template</h3>
              <div className={styles.fieldGrid}>
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
              <p className={styles.hint}>
                UEs use the gNB template's PLMN; the MSIN is incremented per UE. The tester does not create subscribers: add
                {plan && plan.gnbs.length > 0
                  ? <> <span className={styles.mono}>{plan.gnbs[0].firstSupi}</span> … <span className={styles.mono}>{lastSupi(plan)}</span></>
                  : ' every UE'}
                {' '}to the core with these keys and a slice the gNB advertises.
              </p>
            </section>

            <section className={styles.card}>
              <h3 className={styles.cardTitle}>N2 · gNB ↔ AMF</h3>
              <div className={styles.fieldGrid}>
                <Field label="Local interface" path="network.n2.interface" {...fieldProps} />
                <Field label="gNB IP CIDR" path="network.n2.cidr" {...fieldProps} />
                <Field label="First gNB IP" path="network.n2.startIp" {...fieldProps} />
                <Field label="AMF IP" path="network.n2.amfIp" {...fieldProps} />
                <Field label="AMF port" path="network.n2.amfPort" numeric {...fieldProps} />
              </div>
              <p className={styles.hint}>fru-tester adds one IP per gNB to this interface when the run starts and removes them when it stops. IPs already on the host and the AMF IP are skipped.</p>
            </section>

            <section className={styles.card}>
              <h3 className={styles.cardTitle}>N3 · gNB ↔ UPF</h3>
              <div className={styles.fieldGrid}>
                <Field label="Local interface" path="network.n3.interface" {...fieldProps} />
                <Field label="gNB IP CIDR" path="network.n3.cidr" {...fieldProps} />
                <Field label="First gNB IP" path="network.n3.startIp" {...fieldProps} />
                <Field label="UPF IP" path="network.n3.upfIp" {...fieldProps} />
                <Field label="UPF port" path="network.n3.upfPort" numeric {...fieldProps} />
              </div>
              <p className={styles.hint}>Each gNB gets its own N3 IP: uplink leaves from it and the UPF sends downlink to it.</p>
            </section>

            <section className={styles.card}>
              <h3 className={styles.cardTitle}>N6 · data network side</h3>
              <div className={styles.fieldGrid}>
                <Field label="Local interface" path="network.n6.interface" {...fieldProps} />
                <Field label="Sink IP" path="network.n6.sinkIp" {...fieldProps} />
                <Field label="UPF N6 IP" path="network.n6.upfIp" {...fieldProps} />
                <Field label="UE IP pool" path="network.n6.uePool" {...fieldProps} />
              </div>
              <p className={styles.hint}>Uplink leaves the UPF addressed to the sink IP (added to the interface if missing). Downlink is sent from it to each UE's IP; the tester routes the UE pool via the UPF N6 IP for the run and removes the route afterwards.</p>
            </section>

            <section className={styles.card}>
              <h3 className={styles.cardTitle}>Traffic per UE</h3>
              <div className={styles.fieldGrid}>
                <Field label="Uplink (Mbps)" path="traffic.ulMbps" numeric {...fieldProps} />
                <Field label="Downlink (Mbps)" path="traffic.dlMbps" numeric {...fieldProps} />
                <Field label="Packet size (bytes, inner IP)" path="traffic.packetSize" numeric {...fieldProps} />
                <Field label="UDP port" path="traffic.port" numeric {...fieldProps} />
                <Field label="Max run time (min, 0 = no limit)" path="traffic.maxDurationMin" numeric {...fieldProps} />
                <Field label="Downlink batch (ms per UE, 0 = off)" path="traffic.dlBatchMs" numeric {...fieldProps} />
                <Field label="Senders (0 = one per CPU)" path="traffic.senders" numeric {...fieldProps} />
                <Field label="Uplink sink sockets (0 = one per CPU)" path="traffic.sinkSockets" numeric {...fieldProps} />
              </div>
              <p className={styles.hint}>
                Every UE starts sending as soon as its PDU session is up. At full scale: uplink
                {' '}<span className={styles.mono}>{totalRate(profile.traffic.ulMbps, profile.scale.ueCount, profile.traffic.packetSize)}</span>,
                downlink <span className={styles.mono}>{totalRate(profile.traffic.dlMbps, profile.scale.ueCount, profile.traffic.packetSize)}</span>.
                0 turns a direction off. The packet size can go up to the network&apos;s MTU minus 44 bytes of GTP-U (1456 at
                MTU 1500); bigger packets carry more for the same packets per second.
                {' '}With a max run time the run stops itself, exactly as if you pressed Stop.
                {' '}Downlink batch sends each UE that many milliseconds of packets in a row, so the kernel can take them in one
                send (UDP GSO); larger saves CPU but makes each UE&apos;s downlink burstier.
                {' '}Senders and sink sockets default to one per CPU. On a big host far fewer are enough for the tester (the
                Bench page shows how much one sender sends), but when the UPF runs on the same host its packet work runs on
                the senders&apos; CPUs, so fewer senders also give it fewer CPUs.
              </p>
            </section>

            <section className={styles.card}>
              <h3 className={styles.cardTitle}>N2 setup timing</h3>
              <div className={styles.fieldGrid}>
                <Field label="Timeout per attempt (ms)" path="rates.n2.timeoutMs" numeric {...fieldProps} />
                <Field label="Retries" path="rates.n2.retries" numeric {...fieldProps} />
              </div>
              <p className={styles.hint}>A failed attempt with retries left goes to the back of the queue.</p>
            </section>

            <section className={styles.card}>
              <h3 className={styles.cardTitle}>Registration pacing</h3>
              <div className={styles.fieldGrid}>
                <RateFields stage="registration" fieldProps={fieldProps} />
              </div>
              <p className={styles.hint}>Starts per second caps how fast new registrations begin; max in flight caps how many wait for the core at once.</p>
            </section>

            <section className={styles.card}>
              <h3 className={styles.cardTitle}>PDU session pacing</h3>
              <div className={styles.fieldGrid}>
                <RateFields stage="pdu" fieldProps={fieldProps} />
              </div>
              <p className={styles.hint}>A UE moves on to its PDU session as soon as it is registered.</p>
            </section>

            <section className={styles.card}>
              <h3 className={styles.cardTitle}>Deregistration pacing (on Stop)</h3>
              <div className={styles.fieldGrid}>
                <RateFields stage="deregistration" fieldProps={fieldProps} />
              </div>
              <p className={styles.hint}>After Stop, every UE that registered deregisters; the core releases its PDU session with it. Then each gNB's SCTP association is closed.</p>
            </section>

            <section className={styles.card}>
              <h3 className={styles.cardTitle}>Plan preview</h3>
              {!validation && <p className={styles.hint}>Checking…</p>}
              {validation && !validation.valid && (
                <p className={styles.fieldError}>Fix the {fieldErrors.length} highlighted field{fieldErrors.length === 1 ? '' : 's'} to see the plan.</p>
              )}
              {plan && (
                <div className={styles.tableWrap}>
                  <table className={styles.table}>
                    <thead>
                      <tr><th>#</th><th>Name</th><th>gNB ID</th><th>N2 IP</th><th>N3 IP</th><th>UEs</th><th>SUPIs</th></tr>
                    </thead>
                    <tbody>
                      {pageOf(plan.gnbs, previewPage, PREVIEW_PAGE).items.map((g) => (
                        <tr key={g.index}>
                          <td>{g.index}</td>
                          <td>{g.name}</td>
                          <td className={styles.mono}>{g.gnbId}</td>
                          <td className={styles.mono}>{g.n2Ip}/{plan.n2Prefix}</td>
                          <td className={styles.mono}>{g.n3Ip}/{plan.n3Prefix}</td>
                          <td>{g.ueCount ? `${g.ueCount} (#${g.ueFirst}–${g.ueLast})` : '0'}</td>
                          <td className={styles.mono}>{g.ueCount ? `${g.firstSupi} … ${g.lastSupi.slice(-4)}` : '—'}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                  <Pager page={pageOf(plan.gnbs, previewPage, PREVIEW_PAGE)} total={plan.gnbs.length} onChange={setPreviewPage} />
                </div>
              )}
            </section>
          </>
        )}
      </main>
    </div>
  )
}
