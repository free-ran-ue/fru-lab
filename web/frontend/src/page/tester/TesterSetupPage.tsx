import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import Sidebar from '../../components/sidebar/Sidebar'
import Button from '../../components/button/button'
import NotificationContainer from '../../components/notifications/NotificationContainer'
import { useNotifications } from '../../hooks/useNotifications'
import { api, extractErrorMessage } from '../../apiClient'
import type { TesterFieldError, TesterProfile, TesterValidateResponse } from '../../api'
import { DEFAULT_TESTER_PROFILE } from './testerDefaults'
import styles from './tester.module.css'

const PREVIEW_ROWS = 10

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
  onChange: (path: string, value: string | number) => void
}

function Field({ label, path, profile, errors, numeric = false, onChange }: FieldProps) {
  const message = errors.find((e) => e.field === path)?.message
  const value = getPath(profile, path)
  return (
    <div className={styles.field}>
      <label htmlFor={path}>{label}</label>
      <input
        id={path}
        className={`${styles.input} ${message ? styles.inputError : ''}`}
        type={numeric ? 'number' : 'text'}
        value={numeric && Number.isNaN(value) ? '' : value}
        onChange={(e) => onChange(path, numeric ? e.target.valueAsNumber : e.target.value)}
      />
      {message && <p className={styles.fieldError}>{message}</p>}
    </div>
  )
}

export default function TesterSetupPage() {
  const navigate = useNavigate()
  const { errors, successes, addError, addSuccess, removeNotification } = useNotifications()
  const [profile, setProfile] = useState<TesterProfile>(DEFAULT_TESTER_PROFILE)
  const [isLoading, setIsLoading] = useState(true)
  const [validation, setValidation] = useState<TesterValidateResponse | null>(null)
  const [isBusy, setIsBusy] = useState(false)
  const validateSeq = useRef(0)

  useEffect(() => {
    api.testerProfileGet()
      .then((response) => {
        if (response.status === 200 && response.data) setProfile(response.data)
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
            <p className={styles.subtitle}>Describe the gNBs to simulate. Phase 1 brings up N2 (SCTP + NG Setup) for every gNB and holds it until you stop the run.</p>
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
              <p className={styles.hint}>Checked and planned now; N3 IPs are not configured until the data-plane phase.</p>
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
              <h3 className={styles.cardTitle}>Plan preview</h3>
              {!validation && <p className={styles.hint}>Checking…</p>}
              {validation && !validation.valid && (
                <p className={styles.fieldError}>Fix the {fieldErrors.length} highlighted field{fieldErrors.length === 1 ? '' : 's'} to see the plan.</p>
              )}
              {plan && (
                <div className={styles.tableWrap}>
                  <table className={styles.table}>
                    <thead>
                      <tr><th>#</th><th>Name</th><th>gNB ID</th><th>N2 IP</th><th>N3 IP</th><th>UEs</th></tr>
                    </thead>
                    <tbody>
                      {plan.gnbs.slice(0, PREVIEW_ROWS).map((g) => (
                        <tr key={g.index}>
                          <td>{g.index}</td>
                          <td>{g.name}</td>
                          <td className={styles.mono}>{g.gnbId}</td>
                          <td className={styles.mono}>{g.n2Ip}/{plan.n2Prefix}</td>
                          <td className={styles.mono}>{g.n3Ip}/{plan.n3Prefix}</td>
                          <td>{g.ueCount ? `${g.ueCount} (#${g.ueFirst}–${g.ueLast})` : '0'}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                  {plan.gnbs.length > PREVIEW_ROWS && (
                    <p className={styles.hint}>…and {plan.gnbs.length - PREVIEW_ROWS} more gNBs, last one {plan.gnbs[plan.gnbs.length - 1].name} at {plan.gnbs[plan.gnbs.length - 1].n2Ip}.</p>
                  )}
                </div>
              )}
            </section>
          </>
        )}
      </main>
    </div>
  )
}
