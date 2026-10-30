import { useCallback, useEffect, useState } from 'react'
import { Link, Navigate, useParams } from 'react-router-dom'
import Sidebar from '../../components/sidebar/Sidebar'
import Button from '../../components/button/button'
import NotificationContainer from '../../components/notifications/NotificationContainer'
import { useNotifications } from '../../hooks/useNotifications'
import { api, extractErrorMessage } from '../../apiClient'
import type { TesterCpuAllocation, TesterCpuLoad, TesterEntry, TesterFieldError, TesterRunSnapshot, TesterStatus, TesterSystemInfo } from '../../api'
import { formatCpus, parseCpus } from './cpus'
import { CpuMap, CpuUsage } from './TesterCpus'
import { formatBytes, formatDuration } from './testerFormat'
import styles from './tester.module.css'

type Side = 'cp' | 'dp'

const FINISHED = ['idle', 'stopped', 'failed']

// how often each part of the page is refreshed
const LOAD_POLL_MS = 1000
const STATUS_POLL_MS = 3000

const RUN_LABELS: Record<TesterRunSnapshot['state'], string> = {
  idle: 'No run yet',
  configuring: 'Configuring',
  n2: 'N2 setup',
  running: 'Running',
  stopping: 'Cleaning up',
  stopped: 'Stopped',
  failed: 'Failed',
}

// fieldErrorsOf pulls the field errors out of a 400 from PUT /system/cpus.
function fieldErrorsOf(error: unknown): TesterFieldError[] {
  const data = (error as { response?: { data?: { errors?: TesterFieldError[] } } }).response?.data
  return data?.errors ?? []
}

// meterColor runs from green (empty) to red (full), like the CPU usage.
function meterColor(ratio: number): string {
  return `hsl(${120 - 120 * Math.min(1, Math.max(0, ratio))} 65% 45%)`
}

function Meter({ label, used, total, detail }: { label: string, used: number, total: number, detail?: string }) {
  const ratio = total > 0 ? used / total : 0
  return (
    <div className={styles.meter}>
      <div><span>{label}</span><b>{(ratio * 100).toFixed(0)}%</b></div>
      <div className={styles.meterBar}><i style={{ width: `${Math.min(100, ratio * 100)}%`, background: meterColor(ratio) }} /></div>
      <span>{detail ?? `${formatBytes(used)} of ${formatBytes(total)}`}</span>
    </div>
  )
}

function Check({ ok, label, why }: { ok: boolean, label: string, why: string }) {
  return (
    <li>
      <span className={ok ? styles.checkOk : styles.checkBad}>{ok ? '✓' : '✕'}</span>
      <span>{label} <small>{why}</small></span>
    </li>
  )
}

// Overview is the row of tiles at the top: the tester, its run, memory,
// CPUs and what a run needs.
function Overview({ status, latencyMs, run, cpuLoad, saved }: {
  status: TesterStatus, latencyMs: number, run: TesterRunSnapshot | null
  cpuLoad: TesterCpuLoad | null, saved: TesterCpuAllocation
}) {
  const m = status.memory
  const caps = status.capabilities
  const avgBusy = cpuLoad && cpuLoad.cpus.length > 0
    ? cpuLoad.cpus.reduce((a, c) => a + c.busy, 0) / cpuLoad.cpus.length
    : null
  const cp = parseCpus(saved.cpCpus) ?? []
  const dp = parseCpus(saved.dpCpus) ?? []
  const runActive = run !== null && !FINISHED.includes(run.state)
  const afxdp = caps.bpf && caps.ipcLock
  return (
    <>
      <div className={styles.overview}>
        <section className={`${styles.card} ${styles.tile}`}>
          <div className={styles.tileHead}>
            <h3>Tester</h3>
            <span className={`${styles.pill} ${styles.pillOk}`}>Connected · {latencyMs.toFixed(0)} ms</span>
          </div>
          <dl className={styles.kv}>
            <dt>Host</dt><dd className={styles.mono}>{status.hostname}</dd>
            <dt>Build</dt><dd className={styles.mono}>{status.version || 'unknown'}</dd>
            <dt>Uptime</dt><dd>{formatDuration(status.uptimeSec)}</dd>
            <dt>Kernel</dt><dd className={styles.mono}>{status.kernel || '—'}</dd>
            <dt>Go</dt><dd className={styles.mono}>{status.goVersion}</dd>
          </dl>
        </section>

        <section className={`${styles.card} ${styles.tile}`}>
          <div className={styles.tileHead}>
            <h3>Run</h3>
            {run && (
              <span className={`${styles.pill} ${runActive ? styles.pillActive : run.state === 'failed' ? styles.pillBad : styles.pillMuted}`}>
                {RUN_LABELS[run.state]}
              </span>
            )}
          </div>
          {run && run.state !== 'idle' ? (
            <dl className={styles.kv}>
              <dt>Profile</dt><dd>{run.profileName || '—'}</dd>
              <dt>Run</dt><dd className={styles.mono}>{run.runId}</dd>
              <dt>Started</dt><dd>{run.startedAt ? new Date(run.startedAt).toLocaleString() : '—'}</dd>
              <dt>UEs</dt><dd>{run.ues.established} established</dd>
            </dl>
          ) : <p className={styles.hint} style={{ margin: 0 }}>Nothing has run since fru-tester started.</p>}
          <Link to="/tester/run" className={styles.linkButton}>Open the Run page →</Link>
        </section>

        <section className={`${styles.card} ${styles.tile}`}>
          <div className={styles.tileHead}>
            <h3>Ready to run</h3>
            <span className={`${styles.pill} ${caps.netAdmin && caps.sctp ? styles.pillOk : styles.pillBad}`}>
              {caps.netAdmin && caps.sctp ? (afxdp ? 'AF_XDP ready' : 'Sockets only') : 'Not ready'}
            </span>
          </div>
          <ul className={styles.checks}>
            <Check ok={caps.sctp} label="SCTP" why={caps.sctp ? 'module loaded' : 'run modprobe sctp on the host'} />
            <Check ok={caps.netAdmin} label="NET_ADMIN" why="adds the gNB IPs and the UE route" />
            <Check ok={caps.bpf} label="BPF" why="AF_XDP engine" />
            <Check ok={caps.ipcLock} label="IPC_LOCK" why="AF_XDP engine" />
          </ul>
        </section>
      </div>

      <div className={styles.overviewWide}>
        <section className={`${styles.card} ${styles.tile}`}>
          <div className={styles.tileHead}><h3>Memory</h3></div>
          <Meter label="Host" used={m.hostTotal - m.hostAvailable} total={m.hostTotal} />
          {m.cgroupLimit != null && m.cgroupUsage != null
            ? <Meter label="Container limit" used={m.cgroupUsage} total={m.cgroupLimit} />
            : null}
          <dl className={styles.kv}>
            <dt>fru-tester</dt><dd>{formatBytes(m.processRss)} resident</dd>
            <dt>Container</dt><dd>{m.cgroupLimit == null ? 'no memory limit' : `limit ${formatBytes(m.cgroupLimit)}`}</dd>
          </dl>
        </section>

        <section className={`${styles.card} ${styles.tile}`}>
          <div className={styles.tileHead}><h3>CPU</h3></div>
          <Meter label="All CPUs busy" used={avgBusy ?? 0} total={100} detail={avgBusy === null ? 'measuring…' : `load average ${status.cpu.load1.toFixed(2)} · ${status.cpu.load5.toFixed(2)} · ${status.cpu.load15.toFixed(2)}`} />
          <dl className={styles.kv}>
            <dt>Usable</dt><dd>{status.cpu.allowed} of {status.cpu.online} CPUs</dd>
            <dt>Control plane</dt><dd>{cp.length > 0 ? `${cp.length} CPUs · ${saved.cpCpus}` : 'not pinned'}</dd>
            <dt>Data plane</dt><dd>{dp.length > 0 ? `${dp.length} CPUs · ${saved.dpCpus}` : 'not pinned'}</dd>
          </dl>
        </section>
      </div>
    </>
  )
}

// TesterPanel is everything about one tester. fru-lab drives one tester
// for now; with several, the page lists one panel per tester.
function TesterPanel({ id }: { id: string }) {
  const { errors, successes, addError, addSuccess, removeNotification } = useNotifications()
  const [status, setStatus] = useState<TesterStatus | null>(null)
  const [latencyMs, setLatencyMs] = useState(0)
  const [unreachable, setUnreachable] = useState<string | null>(null)
  const [info, setInfo] = useState<TesterSystemInfo | null>(null)
  const [run, setRun] = useState<TesterRunSnapshot | null>(null)
  const [cpuLoad, setCpuLoad] = useState<TesterCpuLoad | null>(null)
  const [form, setForm] = useState<TesterCpuAllocation>({ cpCpus: '', dpCpus: '' })
  const [saved, setSaved] = useState<TesterCpuAllocation>({ cpCpus: '', dpCpus: '' })
  const [fieldErrors, setFieldErrors] = useState<TesterFieldError[]>([])
  const [isSaving, setIsSaving] = useState(false)

  const loadSystem = useCallback(() => {
    api.testerByIdSystem(id)
      .then((r) => { setInfo(r.data); setForm(r.data.cpus); setSaved(r.data.cpus) })
      .catch((error) => addError(extractErrorMessage(error, 'Failed to read the tester host')))
  }, [addError, id])

  // the tester, its run: every few seconds; reaching it again reloads the
  // CPU layout, which a restarted tester may have changed
  useEffect(() => {
    let cancelled = false
    let wasDown = true
    const readStatus = () => {
      if (document.hidden) return
      const t0 = performance.now()
      api.testerByIdStatus(id)
        .then((r) => {
          if (cancelled) return
          setLatencyMs(performance.now() - t0)
          setStatus(r.data)
          setUnreachable(null)
          if (wasDown) loadSystem()
          wasDown = false
        })
        .catch((error) => {
          if (cancelled) return
          setUnreachable(extractErrorMessage(error, 'fru-tester did not answer'))
          wasDown = true
        })
      api.testerByIdRun(id)
        .then((r) => { if (!cancelled) setRun(r.data) })
        .catch(() => {})
    }
    readStatus()
    const timer = window.setInterval(readStatus, STATUS_POLL_MS)
    return () => { cancelled = true; window.clearInterval(timer) }
  }, [loadSystem, id])

  // the CPU load, live
  useEffect(() => {
    let cancelled = false
    const readLoad = () => {
      if (document.hidden) return
      api.testerByIdCpuLoad(id)
        .then((r) => { if (!cancelled) setCpuLoad(r.data) })
        .catch(() => { if (!cancelled) setCpuLoad(null) })
    }
    readLoad()
    const timer = window.setInterval(readLoad, LOAD_POLL_MS)
    return () => { cancelled = true; window.clearInterval(timer) }
  }, [id])

  const runActive = run !== null && !FINISHED.includes(run.state)
  const busy = new Map((cpuLoad?.cpus ?? []).map((l) => [l.cpu, l.busy]))
  const cp = parseCpus(form.cpCpus)
  const dp = parseCpus(form.dpCpus)
  const isDirty = form.cpCpus !== saved.cpCpus || form.dpCpus !== saved.dpCpus
  const message = (field: keyof TesterCpuAllocation, parsed: number[] | null) =>
    fieldErrors.find((e) => e.field === field)?.message ?? (parsed === null ? 'Use a CPU list such as 0-3,8' : undefined)
  const overlap = (cp ?? []).filter((c) => (dp ?? []).includes(c))

  // warnings: the data plane far from its NICs, or split over NUMA nodes
  const warnings: string[] = []
  if (info && dp && dp.length > 0) {
    const nodesOfDp = new Set(info.topology.nodes.filter((n) => (parseCpus(n.cpus) ?? []).some((c) => dp.includes(c))).map((n) => n.node))
    if (nodesOfDp.size > 1) warnings.push(`The data plane CPUs span NUMA nodes ${[...nodesOfDp].join(' and ')}; keep them on one node, the one the N3/N6 NICs are on.`)
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
      const r = await api.testerByIdCpusPut(id, form)
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
    <>
      <NotificationContainer errors={errors} successes={successes} onClose={removeNotification} />

      {unreachable && (
        <section className={`${styles.card} ${styles.tile}`}>
          <div className={styles.tileHead}>
            <h3>Tester</h3>
            <span className={`${styles.pill} ${styles.pillBad}`}>Unreachable</span>
          </div>
          <p className={styles.runError}>{unreachable}</p>
          <p className={styles.hint} style={{ margin: 0 }}>
            Check that fru-tester is running at that URL and that its <code>apiToken</code> is the one fru-lab sends. This page keeps trying every few seconds.
          </p>
        </section>
      )}

      {!unreachable && !status && <p className={styles.emptyState}>Connecting to fru-tester…</p>}

      {status && !unreachable && (
        <Overview status={status} latencyMs={latencyMs} run={run} cpuLoad={cpuLoad} saved={saved} />
      )}

      {info && !unreachable && (
        <>
          <h2 className={styles.sectionTitle}>CPU isolation</h2>
          <section className={styles.card}>
            <div className={styles.cardHead}>
              <h3 className={styles.cardTitle}>CPU allocation</h3>
              <div className={styles.headerActions}>
                <Button variant="secondary" onClick={() => { setForm(saved); setFieldErrors([]) }} disabled={!isDirty || isSaving}>Reset</Button>
                <Button onClick={handleSave} disabled={!isDirty || isSaving || runActive || cp === null || dp === null}>
                  {isSaving ? 'Saving…' : 'Save'}
                </Button>
              </div>
            </div>
            {runActive && <p className={styles.runError} style={{ marginBottom: '1rem' }}>A run is active. The CPU allocation can change once it has stopped.</p>}
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
            <div className={styles.legend} style={{ marginTop: '1.25rem' }}>
              <span><i className={`${styles.cpuSwatch} ${styles.cpuCp}`} />Control plane</span>
              <span><i className={`${styles.cpuSwatch} ${styles.cpuDp}`} />Data plane</span>
              <span><i className={styles.cpuSwatch} />Not pinned</span>
              <span><i className={`${styles.cpuSwatch} ${styles.cpuOff}`} />Not available</span>
              <span><i className={styles.cpuIrqKey} />NIC interrupts</span>
            </div>
            <CpuMap info={info} cp={cp ?? []} dp={dp ?? []} irq={irq} onToggle={toggle} disabled={runActive} />
            <p className={styles.hint}>
              Click a CPU to move it: not pinned → control plane → data plane. fru-tester may use CPUs {info.topology.allowed || '—'} of {info.topology.online || '—'}.
              {' '}<b>Control plane</b>: N2, registration, PDU sessions, the control-plane loop and fru-tester&apos;s API run only on these CPUs during a run.
              {' '}<b>Data plane</b>: every sender and reader keeps its own thread on these CPUs. With N CPUs and G gNBs the socket engine runs G×⌈N/G⌉ uplink senders, N uplink readers (N6 sink), N downlink senders and G downlink readers (one per gNB); AF_XDP runs one reader per NIC queue and up to N senders per NIC. Leave a list empty to not pin that side.
              {' '}The kernel&apos;s share of receiving runs where the NIC&apos;s interrupts go, which fru-tester does not move: set that on the host so they land on the data plane CPUs.
            </p>
          </section>

          <section className={styles.card}>
            <h3 className={styles.cardTitle}>CPU usage</h3>
            <CpuUsage info={info} load={busy} cp={parseCpus(saved.cpCpus) ?? []} dp={parseCpus(saved.dpCpus) ?? []} />
            <div className={styles.usageScale}><span>0%</span><i /><span>100%</span></div>
            <p className={styles.hint}>
              How busy each CPU of the host is, updated every second, during a run too. Every process and the kernel&apos;s interrupt work count, not only fru-tester.
              {' '}CP and DP mark the saved allocation.
            </p>
          </section>

          <h2 className={styles.sectionTitle}>Network</h2>
          <section className={styles.card}>
            <h3 className={styles.cardTitle}>Network interfaces</h3>
            {info.topology.interfaces.length === 0 && <p className={styles.hint}>No physical NICs on this host: virtual interfaces (macvlan, veth, bridges) are not listed. The data plane then uses sockets.</p>}
            {info.topology.interfaces.length > 0 && (
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
            <p className={styles.hint}>Physical NICs only: the ones AF_XDP can use. A VM usually reports no NUMA node and no interrupt CPUs for its virtual NICs.</p>
          </section>
        </>
      )}
    </>
  )
}

// TesterCard is one tester in the list: whether it answers and the gist
// of its state. Clicking it opens that tester's Status page.
function TesterCard({ tester, onRemove }: { tester: TesterEntry, onRemove: (t: TesterEntry, name: string) => void }) {
  const [status, setStatus] = useState<TesterStatus | null>(null)
  const [latencyMs, setLatencyMs] = useState(0)
  const [unreachable, setUnreachable] = useState<string | null>(null)
  const [run, setRun] = useState<TesterRunSnapshot | null>(null)

  useEffect(() => {
    let cancelled = false
    const read = () => {
      if (document.hidden) return
      const t0 = performance.now()
      api.testerByIdStatus(tester.id)
        .then((r) => {
          if (cancelled) return
          setLatencyMs(performance.now() - t0)
          setStatus(r.data)
          setUnreachable(null)
        })
        .catch((error) => { if (!cancelled) setUnreachable(extractErrorMessage(error, 'fru-tester did not answer')) })
      api.testerByIdRun(tester.id)
        .then((r) => { if (!cancelled) setRun(r.data) })
        .catch(() => {})
    }
    read()
    const timer = window.setInterval(read, STATUS_POLL_MS)
    return () => { cancelled = true; window.clearInterval(timer) }
  }, [tester.id])

  const m = status?.memory
  const runActive = run !== null && !FINISHED.includes(run.state)
  const name = status?.name || tester.url
  return (
    <div className={`${styles.card} ${styles.tile} ${styles.testerCard}`}>
      <Link to={`/tester/status/${tester.id}`} className={styles.testerCardLink}>
        <div className={styles.tileHead}>
          <h3>{name}</h3>
          {unreachable
            ? <span className={`${styles.pill} ${styles.pillBad}`}>Unreachable</span>
            : status
              ? <span className={`${styles.pill} ${styles.pillOk}`}>Connected · {latencyMs.toFixed(0)} ms</span>
              : <span className={`${styles.pill} ${styles.pillMuted}`}>Connecting…</span>}
        </div>
        <dl className={styles.kv}>
          <dt>URL</dt><dd className={styles.mono}>{tester.url}</dd>
          {status && !unreachable && m && (
            <>
              <dt>Host</dt><dd className={styles.mono}>{status.hostname}</dd>
              <dt>Run</dt><dd>{run ? RUN_LABELS[run.state] : '—'}{runActive && run?.profileName ? ` · ${run.profileName}` : ''}</dd>
              <dt>Memory</dt><dd>{formatBytes(m.hostTotal - m.hostAvailable)} of {formatBytes(m.hostTotal)} used</dd>
              <dt>CPUs</dt><dd>{status.cpu.allowed} of {status.cpu.online} usable · load {status.cpu.load1.toFixed(2)}</dd>
              <dt>Uptime</dt><dd>{formatDuration(status.uptimeSec)}</dd>
            </>
          )}
        </dl>
        {unreachable && <p className={styles.runError}>{unreachable}</p>}
      </Link>
      <div className={styles.testerCardFoot}>
        {tester.builtIn
          ? <span className={styles.fieldHint}>From config.yaml · runs next to fru-lab</span>
          : <button type="button" className={styles.linkButton} onClick={() => onRemove(tester, name)}>Remove</button>}
      </div>
    </div>
  )
}

// AddTesterForm adds a tester by its URL and API token; fru-lab checks
// that it answers before keeping it.
function AddTesterForm({ onAdded, onCancel }: { onAdded: (name: string) => void, onCancel: () => void }) {
  const [url, setUrl] = useState('http://')
  const [token, setToken] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [isAdding, setIsAdding] = useState(false)

  async function handleAdd(e: React.FormEvent) {
    e.preventDefault()
    setIsAdding(true)
    setError(null)
    try {
      const r = await api.testersAdd({ url, apiToken: token })
      onAdded(r.data.name)
    } catch (err) {
      setError(extractErrorMessage(err, 'Failed to add the tester'))
    } finally {
      setIsAdding(false)
    }
  }

  return (
    <form className={`${styles.card} ${styles.tile}`} onSubmit={handleAdd}>
      <div className={styles.tileHead}><h3>Add a tester</h3></div>
      <div className={styles.fieldGrid}>
        <div className={styles.field}>
          <label htmlFor="testerUrl">URL</label>
          <input id="testerUrl" className={styles.input} type="text" placeholder="http://10.0.0.2:9100" autoFocus
            value={url} onChange={(e) => setUrl(e.target.value)} />
          <p className={styles.fieldHint}>Where fru-lab reaches it: its listen address in tester.yaml.</p>
        </div>
        <div className={styles.field}>
          <label htmlFor="testerToken">API token</label>
          <input id="testerToken" className={styles.input} type="password" autoComplete="off"
            value={token} onChange={(e) => setToken(e.target.value)} />
          <p className={styles.fieldHint}>The apiToken in its tester.yaml.</p>
        </div>
      </div>
      {error && <p className={styles.runError}>{error}</p>}
      <div className={styles.headerActions}>
        <Button type="submit" disabled={isAdding || url.trim() === '' || token.trim() === ''}>{isAdding ? 'Checking…' : 'Add'}</Button>
        <Button type="button" variant="secondary" onClick={onCancel} disabled={isAdding}>Cancel</Button>
      </div>
      <p className={styles.hint} style={{ margin: 0 }}>fru-lab asks the tester for its status first and adds it only if it answers. Its name is the testerName in its tester.yaml.</p>
    </form>
  )
}

// TesterStatusPage lists every tester; each card opens its own page.
export default function TesterStatusPage() {
  const { errors, successes, addError, addSuccess, removeNotification } = useNotifications()
  const [testers, setTesters] = useState<TesterEntry[] | null>(null)
  const [adding, setAdding] = useState(false)

  const load = useCallback(() => {
    api.testersList()
      .then((r) => setTesters(r.data))
      .catch((error) => addError(extractErrorMessage(error, 'Failed to list the testers')))
  }, [addError])
  useEffect(() => { load() }, [load])

  async function handleRemove(t: TesterEntry, name: string) {
    if (!window.confirm(`Remove ${name} (${t.url}) from fru-lab? The tester itself keeps running.`)) return
    try {
      await api.testersDelete(t.id)
      addSuccess(`Removed ${name}`)
      load()
    } catch (error) {
      addError(extractErrorMessage(error, 'Failed to remove the tester'))
    }
  }

  return (
    <div className={styles.layout}>
      <NotificationContainer errors={errors} successes={successes} onClose={removeNotification} />
      <Sidebar />
      <main className={styles.content}>
        <header className={styles.header}>
          <div>
            <h2 className={styles.title}>Throughput Tester · Status</h2>
            <p className={styles.subtitle}>
              The testers fru-lab drives. Open one to see its memory and CPUs, what a run needs, and which CPUs the control plane and the data plane use.
            </p>
          </div>
          <div className={styles.headerActions}>
            <Button onClick={() => setAdding(true)} disabled={adding}>Add tester</Button>
          </div>
        </header>
        {adding && (
          <AddTesterForm
            onCancel={() => setAdding(false)}
            onAdded={(name) => { setAdding(false); addSuccess(`Added ${name}`); load() }}
          />
        )}
        {testers !== null && testers.length === 0 && !adding && (
          <p className={styles.emptyState}>No testers yet: set backend.tester in config.yaml, or add one.</p>
        )}
        <div className={styles.testerGrid}>
          {(testers ?? []).map((t) => <TesterCard key={t.id} tester={t} onRemove={handleRemove} />)}
        </div>
      </main>
    </div>
  )
}

// TesterDetailPage is one tester's Status page.
export function TesterDetailPage() {
  const { testerId = '' } = useParams()
  const [known, setKnown] = useState<boolean | null>(null)
  const [name, setName] = useState('')

  useEffect(() => {
    api.testersList()
      .then((r) => setKnown(r.data.some((t) => t.id === testerId)))
      .catch(() => setKnown(true)) // let the panel say what is wrong
    api.testerByIdStatus(testerId).then((r) => setName(r.data.name)).catch(() => {})
  }, [testerId])

  if (known === false) return <Navigate to="/tester/status" replace />
  return (
    <div className={styles.layout}>
      <Sidebar />
      <main className={styles.content}>
        <header className={styles.header}>
          <div>
            <Link to="/tester/status" className={styles.linkButton}>← All testers</Link>
            <h2 className={styles.title}>Throughput Tester · {name || 'Tester'}</h2>
            <p className={styles.subtitle}>
              Whether it answers, its memory and CPUs, what a run needs, and which CPUs the control plane and the data plane use.
            </p>
          </div>
        </header>
        {known && <TesterPanel id={testerId} />}
      </main>
    </div>
  )
}
