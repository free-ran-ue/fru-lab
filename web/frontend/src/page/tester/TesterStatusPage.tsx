import { useCallback, useEffect, useState } from 'react'
import { Link, Navigate, useParams } from 'react-router-dom'
import AppLayout from '../../components/layout/AppLayout'
import Button from '../../components/button/button'
import NotificationContainer from '../../components/notifications/NotificationContainer'
import { useNotifications } from '../../hooks/useNotifications'
import { api, extractErrorMessage } from '../../apiClient'
import type { TesterCpuAllocation, TesterCpuLoad, TesterEntry, TesterFieldError, TesterRunSnapshot, TesterStatus, TesterSystemInfo } from '../../api'
import { formatCpus, parseCpus } from './cpus'
import { CpuMap, CpuUsage } from './TesterCpus'
import { formatBytes, formatBytesPair, formatDuration } from './testerFormat'
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

// fillTone is the meter fill: accent, then warn and bad as it nears full.
function fillTone(ratio: number): string | undefined {
  return ratio >= 0.95 ? styles.fillBad : ratio >= 0.8 ? styles.fillWarn : undefined
}

// avgBusy is the average busy percentage over every CPU, or null.
function avgBusy(load: TesterCpuLoad | null): number | null {
  return load && load.cpus.length > 0 ? load.cpus.reduce((a, c) => a + c.busy, 0) / load.cpus.length : null
}

// Meter is a label and its value over a 6px track.
function Meter({ label, value, ratio, note }: { label: string, value: string, ratio: number, note?: string }) {
  return (
    <div className={styles.meterRow}>
      <div className={styles.meterTop}><span>{label}</span><span className="num">{value}</span></div>
      <div className={styles.track}><i className={fillTone(ratio)} style={{ width: `${Math.min(100, Math.max(0, ratio * 100))}%` }} /></div>
      {note && <span className={styles.meterNote}>{note}</span>}
    </div>
  )
}

function KvRow({ k, v, mono }: { k: string, v: React.ReactNode, mono?: boolean }) {
  return <div className={styles.kvRow}><span>{k}</span><span className={mono ? 'num' : undefined}>{v}</span></div>
}

function Check({ ok, label, why }: { ok: boolean, label: string, why: string }) {
  return (
    <li>
      <span className={ok ? styles.checkOk : styles.checkBad}>{ok ? '✓' : '✕'}</span>
      <span>{label} <small>{why}</small></span>
    </li>
  )
}

// dpFact is a tester's data plane CPUs and the NUMA nodes they sit on,
// e.g. "8-15 · NUMA 0".
function dpFact(info: TesterSystemInfo | undefined): string {
  if (!info) return '—'
  const dp = parseCpus(info.cpus.dpCpus) ?? []
  if (dp.length === 0) return 'not pinned'
  const nodes = info.topology.nodes.filter((n) => (parseCpus(n.cpus) ?? []).some((c) => dp.includes(c))).map((n) => n.node)
  return nodes.length > 0 ? `${info.cpus.dpCpus} · NUMA ${nodes.join(', ')}` : info.cpus.dpCpus
}

// Overview is the top of a tester's page: the tester, its run, what a run
// needs, then memory and CPUs.
function Overview({ id, status, latencyMs, run, cpuLoad, saved }: {
  id: string, status: TesterStatus, latencyMs: number, run: TesterRunSnapshot | null
  cpuLoad: TesterCpuLoad | null, saved: TesterCpuAllocation
}) {
  const m = status.memory
  const caps = status.capabilities
  const busy = avgBusy(cpuLoad)
  const cp = parseCpus(saved.cpCpus) ?? []
  const dp = parseCpus(saved.dpCpus) ?? []
  const runActive = run !== null && !FINISHED.includes(run.state)
  const afxdp = caps.bpf && caps.ipcLock
  const ready = caps.netAdmin && caps.sctp
  const hostUsed = m.hostTotal - m.hostAvailable
  return (
    <>
      <div className={styles.split}>
        <section className={`${styles.panel} ${styles.panelKv}`}>
          <div className={styles.panelHead}>
            <h2 className={styles.panelTitle}>Tester</h2>
            <span className={`${styles.state} ${styles.stateOk}`}>Connected · {latencyMs.toFixed(0)} ms</span>
          </div>
          <KvRow k="Host" v={status.hostname} mono />
          <KvRow k="Build" v={status.version || 'unknown'} mono />
          <KvRow k="Uptime" v={formatDuration(status.uptimeSec)} mono />
          <KvRow k="Kernel" v={status.kernel || '—'} mono />
          <KvRow k="Go" v={status.goVersion} mono />
        </section>

        <section className={`${styles.panel} ${styles.panelKv}`}>
          <div className={styles.panelHead}>
            <h2 className={styles.panelTitle}>Run</h2>
            {run && (
              <span className={`${styles.state} ${runActive ? styles.stateOk : run.state === 'failed' ? styles.stateBad : ''}`}>
                {RUN_LABELS[run.state]}
              </span>
            )}
          </div>
          {run && run.state !== 'idle' ? (
            <>
              <KvRow k="Profile" v={run.profileName || '—'} />
              <KvRow k="Run" v={run.runId} mono />
              <KvRow k="Started" v={run.startedAt ? new Date(run.startedAt).toLocaleString() : '—'} mono />
              <KvRow k="UEs" v={`${run.ues.established.toLocaleString()} established`} mono />
            </>
          ) : <p className={styles.panelText}>Nothing has run since fru-tester started.</p>}
          <Link to={`/tester/run/${id}`} className={styles.openLink} style={{ marginLeft: 0, fontSize: '0.8125rem' }}>
            Open this tester&apos;s run
            <svg viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="1.6" aria-hidden="true"><path d="M4 2l4 4-4 4" /></svg>
          </Link>
        </section>

        <section className={styles.panel}>
          <div className={styles.panelHead}>
            <h2 className={styles.panelTitle}>Ready to run</h2>
            <span className={`${styles.state} ${ready ? styles.stateOk : styles.stateBad}`}>
              {ready ? (afxdp ? 'AF_XDP ready' : 'Sockets only') : 'Not ready'}
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

      <div className={styles.split}>
        <section className={`${styles.panel} ${styles.panelKv}`}>
          <div className={styles.panelHead}><h2 className={styles.panelTitle}>Memory</h2></div>
          <Meter label="Host" value={formatBytesPair(hostUsed, m.hostTotal)} ratio={m.hostTotal > 0 ? hostUsed / m.hostTotal : 0} />
          {m.cgroupLimit != null && m.cgroupUsage != null && (
            <Meter label="Container limit" value={formatBytesPair(m.cgroupUsage, m.cgroupLimit)} ratio={m.cgroupLimit > 0 ? m.cgroupUsage / m.cgroupLimit : 0} />
          )}
          <KvRow k="fru-tester" v={`${formatBytes(m.processRss)} resident`} mono />
          <KvRow k="Container" v={m.cgroupLimit == null ? 'no memory limit' : `limit ${formatBytes(m.cgroupLimit)}`} mono />
        </section>

        <section className={`${styles.panel} ${styles.panelKv}`}>
          <div className={styles.panelHead}><h2 className={styles.panelTitle}>CPU</h2></div>
          <Meter label="All CPUs busy" value={busy === null ? 'measuring…' : `${busy.toFixed(0)}% of ${status.cpu.online}`} ratio={(busy ?? 0) / 100}
            note={`load average ${status.cpu.load1.toFixed(2)} · ${status.cpu.load5.toFixed(2)} · ${status.cpu.load15.toFixed(2)}`} />
          <KvRow k="Usable" v={`${status.cpu.allowed} of ${status.cpu.online} CPUs`} mono />
          <KvRow k="Control plane" v={cp.length > 0 ? `${cp.length} CPUs · ${saved.cpCpus}` : 'not pinned'} mono />
          <KvRow k="Data plane" v={dp.length > 0 ? `${dp.length} CPUs · ${saved.dpCpus}` : 'not pinned'} mono />
        </section>
      </div>
    </>
  )
}

// TesterPanel is everything about one tester.
function TesterPanel({ id, addError, addSuccess, onReach }: {
  id: string
  addError: (m: string) => void
  addSuccess: (m: string) => void
  onReach: (s: { status: TesterStatus | null, unreachable: string | null }) => void
}) {
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

  useEffect(() => { onReach({ status, unreachable }) }, [onReach, status, unreachable])

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
      {unreachable && (
        <section className={styles.panel}>
          <div className={styles.panelHead}>
            <h2 className={styles.panelTitle}>Tester</h2>
            <span className={`${styles.state} ${styles.stateBad}`}>Unreachable</span>
          </div>
          <p className={styles.runError}>{unreachable}</p>
          <p className={styles.panelText}>
            Check that fru-tester is running at that URL and that its <code>apiToken</code> is the one fru-lab sends. This page keeps trying every few seconds.
          </p>
        </section>
      )}

      {!unreachable && !status && <p className={styles.emptyState}>Connecting to fru-tester…</p>}

      {status && !unreachable && (
        <Overview id={id} status={status} latencyMs={latencyMs} run={run} cpuLoad={cpuLoad} saved={saved} />
      )}

      {info && !unreachable && (
        <>
          <section className={styles.panel}>
            <div className={styles.panelHead}>
              <h2 className={styles.panelTitle}>CPU allocation</h2>
              <div className={styles.panelActions}>
                <Button variant="secondary" onClick={() => { setForm(saved); setFieldErrors([]) }} disabled={!isDirty || isSaving}>Reset</Button>
                <Button onClick={handleSave} disabled={!isDirty || isSaving || runActive || cp === null || dp === null}>
                  {isSaving ? 'Saving…' : 'Save'}
                </Button>
              </div>
            </div>
            {runActive && <p className={styles.runError}>A run is active. The CPU allocation can change once it has stopped.</p>}
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
            <div className={styles.legend}>
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

          <section className={styles.panel}>
            <div className={styles.panelHead}>
              <h2 className={styles.panelTitle}>CPU usage</h2>
              <span className={styles.panelNote}>updated every second</span>
            </div>
            <CpuUsage info={info} load={busy} cp={parseCpus(saved.cpCpus) ?? []} dp={parseCpus(saved.dpCpus) ?? []} />
            <div className={styles.usageScale}><span>0%</span><i /><span>100%</span></div>
            <p className={styles.hint}>
              How busy each CPU of the host is, during a run too. Every process and the kernel&apos;s interrupt work count, not only fru-tester.
              {' '}CP and DP mark the saved allocation.
            </p>
          </section>

          <section className={`${styles.panel} ${styles.panelFlush}`}>
            <div className={styles.panelHead}><h2 className={styles.panelTitle}>Network interfaces</h2></div>
            {info.topology.interfaces.length === 0
              ? <p className={styles.tableEmpty}>No physical NICs on this host: virtual interfaces (macvlan, veth, bridges) are not listed. The data plane then uses sockets.</p>
              : (
                <div className={styles.tableScroll}>
                  <table className={styles.dataTable}>
                    <thead><tr><th>Name</th><th>Driver</th><th>NUMA node</th><th>Local CPUs</th><th className={styles.r}>RX queues</th><th>Interrupt CPUs</th></tr></thead>
                    <tbody>
                      {info.topology.interfaces.map((i) => (
                        <tr key={i.name}>
                          <td className="num">{i.name}</td>
                          <td className="num">{i.driver || '—'}</td>
                          <td className="num">{i.numaNode < 0 ? 'unknown' : i.numaNode}</td>
                          <td className="num">{i.localCpus || '—'}</td>
                          <td className={`num ${styles.r}`}>{i.queues}</td>
                          <td className="num">{i.irqCpus || '—'}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            <p className={styles.tableEmpty} style={{ borderTop: '1px solid var(--border-soft)', fontSize: '0.75rem' }}>
              Physical NICs only: the ones AF_XDP can use. A VM usually reports no NUMA node and no interrupt CPUs for its virtual NICs.
            </p>
          </section>
        </>
      )}
    </>
  )
}

// FleetEntry is what the list knows of one tester.
interface FleetEntry {
  status: TesterStatus | null
  down: string | null
  run: TesterRunSnapshot | null
  busy: number | null
  system?: TesterSystemInfo
  lastSeen: number | null
}

const EMPTY: FleetEntry = { status: null, down: null, run: null, busy: null, lastSeen: null }

// useFleet polls every tester: its status, run, CPU load, and its CPU
// layout once each time it is reached.
function useFleet(testers: TesterEntry[] | null): Record<string, FleetEntry> {
  const [fleet, setFleet] = useState<Record<string, FleetEntry>>({})
  useEffect(() => {
    if (!testers) return
    let cancelled = false
    const reached = new Set<string>()
    const set = (id: string, f: (e: FleetEntry) => FleetEntry) =>
      setFleet((cur) => ({ ...cur, [id]: f(cur[id] ?? EMPTY) }))
    const read = () => {
      if (document.hidden) return
      for (const t of testers) {
        api.testerByIdStatus(t.id)
          .then((r) => {
            if (cancelled) return
            set(t.id, (e) => ({ ...e, status: r.data, down: null, lastSeen: Date.now() }))
            if (!reached.has(t.id)) {
              reached.add(t.id)
              api.testerByIdSystem(t.id)
                .then((s) => { if (!cancelled) set(t.id, (e) => ({ ...e, system: s.data })) })
                .catch(() => {})
            }
          })
          .catch((error) => {
            if (cancelled) return
            reached.delete(t.id)
            set(t.id, (e) => ({ ...e, down: extractErrorMessage(error, 'fru-tester did not answer') }))
          })
        api.testerByIdRun(t.id)
          .then((r) => { if (!cancelled) set(t.id, (e) => ({ ...e, run: r.data })) })
          .catch(() => {})
        api.testerByIdCpuLoad(t.id)
          .then((r) => { if (!cancelled) set(t.id, (e) => ({ ...e, busy: avgBusy(r.data) })) })
          .catch(() => {})
      }
    }
    read()
    const timer = window.setInterval(read, STATUS_POLL_MS)
    return () => { cancelled = true; window.clearInterval(timer) }
  }, [testers])
  return fleet
}

const OpenArrow = () => (
  <svg viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="1.6" aria-hidden="true"><path d="M4 2l4 4-4 4" /></svg>
)

// TesterCard is one tester in the list; it opens that tester's page.
function TesterCard({ tester, entry, onRemove }: { tester: TesterEntry, entry: FleetEntry, onRemove: (t: TesterEntry, name: string) => void }) {
  const { status, down, run, busy, system, lastSeen } = entry
  const runActive = run !== null && !FINISHED.includes(run.state)
  const name = status?.name || tester.url
  const m = status?.memory
  const to = `/tester/status/${tester.id}`

  let pill = <span className={styles.state}>Connecting</span>
  if (down) pill = <span className={`${styles.state} ${styles.stateBad}`}>Unreachable</span>
  else if (status) pill = runActive
    ? <span className={`${styles.state} ${run?.state === 'stopping' ? styles.stateWarn : styles.stateOk}`}>{RUN_LABELS[run!.state]}</span>
    : <span className={styles.state}>Idle</span>

  let foot = ''
  if (down) foot = lastSeen ? `Last seen ${new Date(lastSeen).toLocaleTimeString()}` : 'No answer since this page opened'
  else if (run && run.state !== 'idle') {
    foot = runActive
      ? `Run ${run.runId} · ${run.registration.expected.toLocaleString()} UEs`
      : `Last run ${run.runId} · ${RUN_LABELS[run.state]}`
  } else if (status) foot = 'No run yet'

  return (
    <div className={styles.tcard}>
      <Link to={to} className={styles.tcardLink}>
        <div className={styles.tcardHead}>
          <div className={styles.tcardName}>
            <div className={styles.tcardNameRow}>
              <span className={styles.tcardTitle}>{name}</span>
              {tester.builtIn && <span className={styles.defaultTag} title="From config.yaml · runs next to fru-lab">DEFAULT</span>}
            </div>
            <span className={`num ${styles.tcardUrl}`}>{tester.url}</span>
          </div>
          {pill}
        </div>
        {down && (
          <div className={styles.tcardDown}>
            <span>{lastSeen ? `No answer for ${formatDuration(Math.round((Date.now() - lastSeen) / 1000))}` : 'No answer'}</span>
            <span>{down}</span>
          </div>
        )}
        {!down && !status && <div className={styles.tcardWait}>Connecting to fru-tester…</div>}
        {!down && status && m && (
          <div className={styles.tcardBody}>
            <Meter label="CPU" value={busy === null ? '—' : `${busy.toFixed(0)}% of ${status.cpu.online}`} ratio={(busy ?? 0) / 100} />
            <Meter label="Memory" value={formatBytesPair(m.hostTotal - m.hostAvailable, m.hostTotal)}
              ratio={m.hostTotal > 0 ? (m.hostTotal - m.hostAvailable) / m.hostTotal : 0} />
            <dl className={styles.facts}>
              <div><dt>DP CPUs</dt><dd>{dpFact(system)}</dd></div>
              <div><dt>Kernel</dt><dd>{status.kernel || '—'}</dd></div>
              <div><dt>Uptime</dt><dd>{formatDuration(status.uptimeSec)}</dd></div>
              <div><dt>Version</dt><dd>{status.version || 'unknown'}</dd></div>
            </dl>
          </div>
        )}
      </Link>
      <div className={styles.tcardFoot}>
        <span>{foot}</span>
        {!tester.builtIn && <button type="button" className={styles.removeLink} onClick={() => onRemove(tester, name)}>Remove</button>}
        <Link to={to} className={styles.openLink} tabIndex={-1}>Open<OpenArrow /></Link>
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
    <form className={styles.panel} onSubmit={handleAdd}>
      <div className={styles.panelHead}><h2 className={styles.panelTitle}>Add a tester</h2></div>
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
      <div className={styles.panelActions}>
        <Button type="submit" disabled={isAdding || url.trim() === '' || token.trim() === ''}>{isAdding ? 'Checking…' : 'Add'}</Button>
        <Button type="button" variant="secondary" onClick={onCancel} disabled={isAdding}>Cancel</Button>
      </div>
      <p className={styles.panelText} style={{ fontSize: '0.75rem' }}>fru-lab asks the tester for its status first and adds it only if it answers. Its name is the testerName in its tester.yaml.</p>
    </form>
  )
}

const PlusIcon = () => (
  <svg className={styles.btnIcon} viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.8" aria-hidden="true"><path d="M7 2v10M2 7h10" /></svg>
)

// TesterStatusPage lists every tester; each card opens its own page.
export default function TesterStatusPage() {
  const { errors, successes, addError, addSuccess, removeNotification } = useNotifications()
  const [testers, setTesters] = useState<TesterEntry[] | null>(null)
  const [adding, setAdding] = useState(false)
  const fleet = useFleet(testers)

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

  // the fleet summary, from what the cards have loaded
  const entries = (testers ?? []).map((t) => fleet[t.id]).filter((e): e is FleetEntry => e !== undefined)
  const withSystem = entries.filter((e) => e.system)
  const summary = [
    { label: 'Testers', value: testers === null ? '—' : String(testers.length) },
    { label: 'Running', value: String(entries.filter((e) => !e.down && e.run && !FINISHED.includes(e.run.state)).length) },
    { label: 'Unreachable', value: String(entries.filter((e) => e.down).length) },
    {
      label: 'DP CPUs isolated',
      value: withSystem.length === 0 ? '—' : String(withSystem.reduce((a, e) => a + (parseCpus(e.system!.cpus.dpCpus) ?? []).length, 0)),
    },
  ]

  return (
    <AppLayout crumbs={[{ label: 'Testers' }]}
      overlay={<NotificationContainer errors={errors} successes={successes} onClose={removeNotification} />}>
      <section className={styles.pageHead}>
        <div className={styles.pageHeadText}>
          <h1 className={styles.pageTitle}>Testers</h1>
          <p className={styles.pageSub}>Every fru-tester this console drives. Open one for CPU isolation, interfaces and its current run.</p>
        </div>
        <div className={styles.pageActions}>
          <Button onClick={() => setAdding(true)} disabled={adding}><PlusIcon />Add tester</Button>
        </div>
      </section>

      <section aria-label="Fleet summary" className={styles.kpis}>
        {summary.map((s) => (
          <div key={s.label} className={styles.kpi}>
            <span className={styles.kpiLabel}>{s.label}</span>
            <span className={`num ${styles.kpiValue}`}>{s.value}</span>
          </div>
        ))}
      </section>

      {adding && (
        <AddTesterForm
          onCancel={() => setAdding(false)}
          onAdded={(name) => { setAdding(false); addSuccess(`Added ${name}`); load() }}
        />
      )}
      {testers !== null && testers.length === 0 && !adding && (
        <p className={styles.emptyState}>No testers yet: set backend.tester in config.yaml, or add one.</p>
      )}
      <section aria-label="Testers" className={styles.cardGrid}>
        {(testers ?? []).map((t) => <TesterCard key={t.id} tester={t} entry={fleet[t.id] ?? EMPTY} onRemove={handleRemove} />)}
      </section>
    </AppLayout>
  )
}

// TesterDetailPage is one tester's page.
export function TesterDetailPage() {
  const { testerId = '' } = useParams()
  const { errors, successes, addError, addSuccess, removeNotification } = useNotifications()
  const [entry, setEntry] = useState<TesterEntry | null>(null)
  const [known, setKnown] = useState<boolean | null>(null)
  const [reach, setReach] = useState<{ status: TesterStatus | null, unreachable: string | null }>({ status: null, unreachable: null })

  useEffect(() => {
    api.testersList()
      .then((r) => {
        const t = r.data.find((x) => x.id === testerId) ?? null
        setEntry(t)
        setKnown(t !== null)
      })
      .catch(() => setKnown(true)) // let the panel say what is wrong
  }, [testerId])

  if (known === false) return <Navigate to="/tester/status" replace />
  const name = reach.status?.name || entry?.url || 'Tester'
  return (
    <AppLayout crumbs={[{ label: 'Testers', to: '/tester/status' }, { label: name }]}
      overlay={<NotificationContainer errors={errors} successes={successes} onClose={removeNotification} />}>
      <section className={styles.pageHead}>
        <div className={styles.pageHeadText}>
          <div className={styles.titleRow}>
            <h1 className={styles.pageTitle}>{name}</h1>
            {entry?.builtIn && <span className={styles.defaultTag} title="From config.yaml · runs next to fru-lab">DEFAULT</span>}
            {reach.unreachable
              ? <span className={`${styles.state} ${styles.stateLg} ${styles.stateBad}`}>Unreachable</span>
              : reach.status
                ? <span className={`${styles.state} ${styles.stateLg} ${styles.stateOk}`}>Connected</span>
                : null}
          </div>
          <p className={styles.pageSub}>
            {entry && <span className="num">{entry.url}</span>}{entry && ' · '}
            Whether it answers, its memory and CPUs, what a run needs, and which CPUs the control plane and the data plane use.
          </p>
        </div>
      </section>
      {known && <TesterPanel id={testerId} addError={addError} addSuccess={addSuccess} onReach={setReach} />}
    </AppLayout>
  )
}
