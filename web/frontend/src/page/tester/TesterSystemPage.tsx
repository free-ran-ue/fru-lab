import { useCallback, useEffect, useState } from 'react'
import Sidebar from '../../components/sidebar/Sidebar'
import Button from '../../components/button/button'
import NotificationContainer from '../../components/notifications/NotificationContainer'
import { useNotifications } from '../../hooks/useNotifications'
import { api, extractErrorMessage } from '../../apiClient'
import type { TesterCpuAllocation, TesterFieldError, TesterSystemInfo } from '../../api'
import { formatCpus, parseCpus } from './cpus'
import styles from './tester.module.css'

type Side = 'cp' | 'dp'

const FINISHED = ['idle', 'stopped', 'failed']

// fieldErrorsOf pulls the field errors out of a 400 from PUT /system/cpus.
function fieldErrorsOf(error: unknown): TesterFieldError[] {
  const data = (error as { response?: { data?: { errors?: TesterFieldError[] } } }).response?.data
  return data?.errors ?? []
}

// CpuMap shows every CPU by NUMA node, coloured by side; clicking one
// moves it: unassigned → control plane → data plane → unassigned.
function CpuMap({ info, cp, dp, irq, onToggle, disabled }: {
  info: TesterSystemInfo, cp: number[], dp: number[], irq: number[]
  onToggle: (cpu: number) => void, disabled: boolean
}) {
  const allowed = parseCpus(info.topology.allowed) ?? []
  const online = parseCpus(info.topology.online) ?? allowed
  const nodes = info.topology.nodes.length > 0
    ? info.topology.nodes.map((n) => ({ label: `NUMA node ${n.node}`, cpus: parseCpus(n.cpus) ?? [] }))
    : [{ label: 'CPUs', cpus: online }]
  return (
    <div className={styles.cpuMap}>
      {nodes.map((n) => (
        <div key={n.label} className={styles.cpuNode}>
          <div className={styles.cpuNodeHead}>
            <b>{n.label}</b>
            <span className={styles.pingNote}>
              {info.topology.interfaces.filter((i) => `NUMA node ${i.numaNode}` === n.label).map((i) => i.name).join(', ')}
            </span>
          </div>
          <div className={styles.cpuGrid}>
            {n.cpus.map((c) => {
              const side = cp.includes(c) ? styles.cpuCp : dp.includes(c) ? styles.cpuDp : ''
              const usable = allowed.includes(c)
              return (
                <button key={c} type="button" disabled={disabled || !usable}
                  className={`${styles.cpu} ${side} ${usable ? '' : styles.cpuOff}`}
                  title={`CPU ${c}${usable ? '' : ' — not available to fru-tester'}${irq.includes(c) ? ' — NIC interrupts' : ''}`}
                  onClick={() => onToggle(c)}>
                  {c}{irq.includes(c) && <i className={styles.cpuIrq} />}
                </button>
              )
            })}
          </div>
        </div>
      ))}
    </div>
  )
}

export default function TesterSystemPage() {
  const { errors, successes, addError, addSuccess, removeNotification } = useNotifications()
  const [info, setInfo] = useState<TesterSystemInfo | null>(null)
  const [form, setForm] = useState<TesterCpuAllocation>({ cpCpus: '', dpCpus: '' })
  const [saved, setSaved] = useState<TesterCpuAllocation>({ cpCpus: '', dpCpus: '' })
  const [fieldErrors, setFieldErrors] = useState<TesterFieldError[]>([])
  const [runActive, setRunActive] = useState(false)
  const [isSaving, setIsSaving] = useState(false)

  const load = useCallback(() => {
    api.testerSystemGet()
      .then((r) => { setInfo(r.data); setForm(r.data.cpus); setSaved(r.data.cpus) })
      .catch((error) => addError(extractErrorMessage(error, 'Failed to read the tester host')))
    api.testerRunGet()
      .then((r) => setRunActive(!FINISHED.includes(r.data.state)))
      .catch(() => setRunActive(false))
  }, [addError])
  useEffect(() => { load() }, [load])

  const cp = parseCpus(form.cpCpus)
  const dp = parseCpus(form.dpCpus)
  const isDirty = form.cpCpus !== saved.cpCpus || form.dpCpus !== saved.dpCpus
  const message = (field: keyof TesterCpuAllocation, parsed: number[] | null) =>
    fieldErrors.find((e) => e.field === field)?.message ?? (parsed === null ? 'Use a CPU list such as 0-3,8' : undefined)
  const overlap = (cp ?? []).filter((c) => (dp ?? []).includes(c))

  // warnings: the data plane far from its NICs, or split over NUMA nodes
  const warnings: string[] = []
  if (info && dp && dp.length > 0) {
    const nodesOf = new Set(info.topology.nodes.filter((n) => (parseCpus(n.cpus) ?? []).some((c) => dp.includes(c))).map((n) => n.node))
    if (nodesOf.size > 1) warnings.push(`The data plane CPUs span NUMA nodes ${[...nodesOf].join(' and ')}; keep them on one node, the one the N3/N6 NICs are on.`)
    const nics = info.topology.interfaces.filter((i) => i.localCpus !== '')
    const near = nics.filter((i) => dp.every((c) => (parseCpus(i.localCpus) ?? []).includes(c)))
    if (nics.length > 0 && near.length === 0) warnings.push('No NIC is local to all the data plane CPUs: packets cross NUMA nodes on their way to and from the NIC.')
  }
  if (overlap.length > 0) warnings.push(`CPU ${formatCpus(overlap)} is in both lists.`)

  function setField(field: keyof TesterCpuAllocation, value: string) {
    setFieldErrors([])
    setForm((f) => ({ ...f, [field]: value }))
  }

  // toggle moves one CPU to the next side from the CPU map.
  function toggle(c: number) {
    const a = cp ?? []
    const b = dp ?? []
    const next: Side | null = a.includes(c) ? 'dp' : b.includes(c) ? null : 'cp'
    setFieldErrors([])
    setForm({
      cpCpus: formatCpus(next === 'cp' ? [...a, c] : a.filter((x) => x !== c)),
      dpCpus: formatCpus(next === 'dp' ? [...b, c] : b.filter((x) => x !== c)),
    })
  }

  async function handleSave() {
    setIsSaving(true)
    try {
      const r = await api.testerSystemCpusPut(form)
      setForm(r.data)
      setSaved(r.data)
      setFieldErrors([])
      addSuccess('CPU allocation saved; the next run uses it')
    } catch (error) {
      setFieldErrors(fieldErrorsOf(error))
      addError(extractErrorMessage(error, 'Failed to save the CPU allocation'))
    } finally {
      setIsSaving(false)
    }
  }

  const irq = [...new Set((info?.topology.interfaces ?? []).flatMap((i) => parseCpus(i.irqCpus) ?? []))]
  const count = (s: number[] | null) => (s === null ? '' : s.length === 0 ? 'not pinned' : `${s.length} CPU${s.length === 1 ? '' : 's'}`)

  return (
    <div className={styles.layout}>
      <NotificationContainer errors={errors} successes={successes} onClose={removeNotification} />
      <Sidebar />

      <main className={styles.content}>
        <header className={styles.header}>
          <div>
            <h2 className={styles.title}>Throughput Tester · System</h2>
            <p className={styles.subtitle}>
              Which CPUs of the tester host the control plane and the data plane use. Set it before a run: a run keeps the allocation it started with.
            </p>
          </div>
          <div className={styles.headerActions}>
            <Button variant="secondary" onClick={() => { setForm(saved); setFieldErrors([]) }} disabled={!isDirty || isSaving}>Reset</Button>
            <Button onClick={handleSave} disabled={!info || !isDirty || isSaving || runActive || cp === null || dp === null}>
              {isSaving ? 'Saving…' : 'Save'}
            </Button>
          </div>
        </header>

        {runActive && <p className={styles.runError}>A run is active. The CPU allocation can change once it has stopped.</p>}

        <section className={styles.card}>
          <h3 className={styles.cardTitle}>CPU allocation</h3>
          <div className={styles.fieldGrid}>
            {([['cpCpus', 'Control plane CPUs', cp], ['dpCpus', 'Data plane CPUs', dp]] as const).map(([field, label, parsed]) => {
              const msg = message(field, parsed)
              return (
                <div key={field} className={styles.field}>
                  <label htmlFor={field}>{label}</label>
                  <input id={field} className={`${styles.input} ${msg ? styles.inputError : ''}`} type="text" placeholder="e.g. 0-3"
                    value={form[field]} disabled={runActive} onChange={(e) => setField(field, e.target.value)} />
                  <p className={styles.fieldHint}>{count(parsed)}</p>
                  {msg && <p className={styles.fieldError}>{msg}</p>}
                </div>
              )
            })}
          </div>
          {warnings.map((w) => <p key={w} className={styles.cpuWarn}>{w}</p>)}
          <p className={styles.hint}>
            <b>Control plane</b>: N2, registration, PDU sessions, the control-plane loop and fru-tester&apos;s API run only on these CPUs during a run.
            {' '}<b>Data plane</b>: every sender and reader keeps its own thread on these CPUs, one of each per CPU. Leave a list empty to not pin that side.
            {' '}The kernel&apos;s share of receiving runs where the NIC&apos;s interrupts go, which fru-tester does not move: set that on the host so they land on the data plane CPUs.
          </p>
        </section>

        <section className={styles.card}>
          <h3 className={styles.cardTitle}>CPUs</h3>
          {!info && <p className={styles.hint}>Loading…</p>}
          {info && (
            <>
              <div className={styles.legend} style={{ marginTop: 0 }}>
                <span><i className={`${styles.cpuSwatch} ${styles.cpuCp}`} />Control plane</span>
                <span><i className={`${styles.cpuSwatch} ${styles.cpuDp}`} />Data plane</span>
                <span><i className={styles.cpuSwatch} />Not pinned</span>
                <span><i className={`${styles.cpuSwatch} ${styles.cpuOff}`} />Not available</span>
                <span><i className={styles.cpuIrqKey} />NIC interrupts</span>
              </div>
              <CpuMap info={info} cp={cp ?? []} dp={dp ?? []} irq={irq} onToggle={toggle} disabled={runActive} />
              <p className={styles.hint}>
                Click a CPU to move it: not pinned → control plane → data plane. fru-tester may use CPUs {info.topology.allowed || '—'} of {info.topology.online || '—'}.
              </p>
            </>
          )}
        </section>

        <section className={styles.card}>
          <h3 className={styles.cardTitle}>Network interfaces</h3>
          {info && info.topology.interfaces.length === 0 && <p className={styles.hint}>No physical NICs on this host.</p>}
          {info && info.topology.interfaces.length > 0 && (
            <div className={styles.tableWrap}>
              <table className={styles.table}>
                <thead><tr><th>Name</th><th>Driver</th><th>NUMA node</th><th>Local CPUs</th><th>RX queues</th><th>Interrupt CPUs</th></tr></thead>
                <tbody>
                  {info.topology.interfaces.map((i) => (
                    <tr key={i.name}>
                      <td>{i.name}</td>
                      <td className={styles.mono}>{i.driver || '—'}</td>
                      <td>{i.numaNode < 0 ? 'unknown' : i.numaNode}</td>
                      <td className={styles.mono}>{i.localCpus || '—'}</td>
                      <td>{i.queues}</td>
                      <td className={styles.mono}>{i.irqCpus || '—'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          <p className={styles.hint}>A VM usually reports no NUMA node and no interrupt CPUs for its virtual NICs.</p>
        </section>
      </main>
    </div>
  )
}
