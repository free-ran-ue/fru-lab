import { useCallback, useEffect, useRef, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import AppLayout from '../../components/layout/AppLayout'
import NotificationContainer from '../../components/notifications/NotificationContainer'
import { useNotifications } from '../../hooks/useNotifications'
import { api, extractErrorMessage } from '../../apiClient'
import type { TesterBenchResult, TesterBenchStep, TesterSystemInfo } from '../../api'
import { formatBps } from './testerFormat'
import { DEFAULT_TESTER_ID } from './testerDefaults'
import { testerLabel, useTesters } from './useTesters'
import styles from './bench.module.css'

const POLL_MS = 1000

const STATE_LABELS: Record<TesterBenchResult['state'], string> = {
  idle: 'Not run yet',
  running: 'Measuring',
  done: 'Done',
  failed: 'Failed',
}

function kpps(pps: number): string {
  return `${Math.round(pps / 1000).toLocaleString()} k`
}

// efficiency is how close n senders get to n times one sender.
function efficiency(step: TesterBenchStep, first: TesterBenchStep | undefined): string {
  if (!first || !first.pps) return '—'
  return `${Math.round((step.pps / (first.pps * step.senders)) * 100)} %`
}

// scaling is efficiency as a number (0..1), or null without a first step.
function scaling(step: TesterBenchStep, first: TesterBenchStep | undefined): number | null {
  return first && first.pps ? step.pps / (first.pps * step.senders) : null
}

// stamp is a local date and time, e.g. 2026-11-03 05:41:12.
function stamp(iso: string | null | undefined): string {
  if (!iso) return '—'
  const d = new Date(iso)
  const p = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
}

// niceMax rounds a chart's top up to 4 even steps of 1, 2, 2.5 or 5 × 10^k.
function niceMax(v: number): number {
  if (v <= 0) return 1
  const raw = v / 4
  const mag = 10 ** Math.floor(Math.log10(raw))
  const step = [1, 2, 2.5, 5, 10].map((m) => m * mag).find((x) => x >= raw) ?? 10 * mag
  return step * 4
}

// Chart geometry (viewBox units), as in HostBench.dc.html.
const CH = { w: 600, h: 250, left: 48, right: 588, top: 10, bottom: 210, padX: 20 }

// ScalingChart is Gbit/s by senders (log2 x axis) with the linear
// reference n × one sender, dashed.
function ScalingChart({ senders, steps }: { senders: number[], steps: TesterBenchStep[] }) {
  const xsAll = [...new Set([...senders, ...steps.map((s) => s.senders)])].sort((a, b) => a - b)
  const lo = Math.log2(xsAll[0])
  const hi = Math.log2(xsAll[xsAll.length - 1])
  const x0 = CH.left + CH.padX
  const x1 = CH.right - CH.padX
  const x = (n: number) => (hi === lo ? (x0 + x1) / 2 : x0 + ((Math.log2(n) - lo) / (hi - lo)) * (x1 - x0))
  const gb = (bps: number) => bps / 1e9
  const top = niceMax(Math.max(...steps.map((s) => gb(s.bps)), 0) * 1.15)
  const y = (g: number) => CH.bottom - (g / top) * (CH.bottom - CH.top)
  const first = steps[0]
  const measured = steps.map((s) => `${x(s.senders)},${y(gb(s.bps)).toFixed(1)}`).join(' ')
  const linear = first ? xsAll.map((n) => `${x(n)},${y((gb(first.bps) * n) / first.senders).toFixed(1)}`).join(' ') : ''
  const ticks = [0, 1, 2, 3, 4].map((i) => (top / 4) * i)
  const fmt = (v: number) => (Number.isInteger(v) ? String(v) : v.toFixed(1))
  const last = steps.length > 0 ? steps[steps.length - 1] : undefined
  const best = steps.reduce<TesterBenchStep | undefined>((a, s) => (!a || s.bps > a.bps ? s : a), undefined)
  const label = first && best
    ? `Throughput from ${gb(first.bps).toFixed(1)} Gbit/s with ${first.senders} sender${first.senders === 1 ? '' : 's'} to ${gb(best.bps).toFixed(1)} Gbit/s at best with ${best.senders} senders`
    : 'Throughput by number of senders'
  return (
    <svg viewBox={`0 0 ${CH.w} ${CH.h}`} role="img" aria-label={label}>
      <defs><clipPath id="bench-plot"><rect x={CH.left} y={CH.top} width={CH.right - CH.left} height={CH.bottom - CH.top} /></clipPath></defs>
      {ticks.slice(1).map((t) => <line key={t} className={styles.gridH} x1={CH.left} x2={CH.right} y1={y(t)} y2={y(t)} />)}
      {xsAll.map((n) => <line key={n} className={styles.gridV} x1={x(n)} x2={x(n)} y1={CH.top} y2={CH.bottom} />)}
      <line className={styles.axis} x1={CH.left} x2={CH.right} y1={CH.bottom} y2={CH.bottom} />
      {ticks.map((t) => <text key={t} className={styles.tick} x={CH.left - 6} y={y(t) + 4} textAnchor="end">{fmt(t)}</text>)}
      {xsAll.map((n) => <text key={n} className={styles.tick} x={x(n)} y={CH.bottom + 17} textAnchor="middle">{n}</text>)}
      <text className={styles.axisLabel} x={(CH.left + CH.right) / 2} y={CH.h - 5} textAnchor="middle">Senders (log₂ scale)</text>
      {linear && <polyline className={styles.linear} points={linear} clipPath="url(#bench-plot)" />}
      {steps.length > 1 && <polyline className={styles.measured} points={measured} />}
      {steps.map((s) => {
        const cx = x(s.senders)
        const cy = y(gb(s.bps))
        const anchor = s === first && steps.length > 1 ? 'start' : s === last && steps.length > 1 ? 'end' : 'middle'
        return (
          <g key={s.senders}>
            <rect className={styles.dot} x={cx - 3} y={cy - 3} width={6} height={6} />
            <text className={styles.dotLabel} x={cx} y={cy - 9} textAnchor={anchor}>{gb(s.bps).toFixed(1)}</text>
          </g>
        )
      })}
    </svg>
  )
}

// The bench runs on the tester picked at the top of Settings
// (/api/testers/<id>/bench); ?tester=<id> keeps the choice in the URL.
export default function TesterBenchPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const testerId = searchParams.get('tester') || DEFAULT_TESTER_ID
  const { errors, successes, addError, addSuccess, removeNotification } = useNotifications()
  const [result, setResult] = useState<TesterBenchResult | null>(null)
  const [packetSize, setPacketSize] = useState(1400)
  const [stepSeconds, setStepSeconds] = useState(3)
  const [gso, setGso] = useState(true)
  const [isStarting, setIsStarting] = useState(false)
  // the bench host's NUMA nodes and NICs for the "This host" box; null
  // until (or unless) the tester answers
  const [system, setSystem] = useState<TesterSystemInfo | null>(null)
  const { testers } = useTesters()
  // the tester whose answers are wanted: a reply for one picked before
  // is dropped
  const current = useRef(testerId)
  current.current = testerId

  useEffect(() => {
    setSystem(null)
    api.testerByIdSystem(testerId)
      .then((response) => { if (current.current === testerId) setSystem(response.data) })
      .catch(() => { if (current.current === testerId) setSystem(null) })
  }, [testerId])

  const load = useCallback(async () => {
    const id = testerId
    try {
      const response = await api.testerByIdBenchGet(id)
      if (current.current === id) setResult(response.data)
    } catch (error) {
      if (current.current === id) {
        setResult(null)
        addError(extractErrorMessage(error, 'Failed to load the bench'))
      }
    }
  }, [addError, testerId])

  useEffect(() => {
    setResult(null)
    load()
  }, [load])

  function pickTester(id: string) {
    setSearchParams(id === DEFAULT_TESTER_ID ? {} : { tester: id }, { replace: true })
  }

  // poll while a bench is measuring
  const isRunning = result?.state === 'running'
  useEffect(() => {
    if (!isRunning) return
    const timer = window.setInterval(load, POLL_MS)
    return () => window.clearInterval(timer)
  }, [isRunning, load])

  async function handleStart() {
    setIsStarting(true)
    try {
      const response = await api.testerByIdBenchStart(testerId, { packetSize, stepSeconds, gso })
      setResult(response.data)
      addSuccess('Bench started')
    } catch (error) {
      addError(extractErrorMessage(error, 'Failed to start the bench'))
    } finally {
      setIsStarting(false)
    }
  }

  const steps = result?.steps ?? []
  const first = steps[0]
  const best = steps.reduce<TesterBenchStep | undefined>((a, s) => (!a || s.bps > a.bps ? s : a), undefined)
  const peakPps = steps.reduce((m, s) => Math.max(m, s.pps), 0)
  const sendErrors = steps.reduce((n, s) => n + s.sendErrors, 0)
  const planned = result?.plannedSenders ?? []
  const pending = isRunning ? planned.slice(steps.length) : []
  const estimateSec = (result?.cpus ? planned.length || 1 : 1) * (stepSeconds + 0.5)
  const stateTone = result?.state === 'failed' ? styles.tBad : result?.state === 'done' ? styles.tOk : isRunning ? styles.tRun : ''
  const testerName = testerLabel(testers, testerId)
  const testerKnown = testers === null ? null : testers.some((t) => t.id === testerId)
  const nics = system?.topology.interfaces ?? []
  const host = [
    result?.cpus ? `${result.cpus} CPUs` : '',
    system && system.topology.nodes.length > 0 ? `${system.topology.nodes.length} NUMA` : '',
    nics.map((n) => (n.driver ? `${n.name} (${n.driver})` : n.name)).join(', '),
    result?.kernel ? `kernel ${result.kernel}` : '',
  ].filter(Boolean).join(' · ')
  const gsoText = steps.length === 0 || !result
    ? '—'
    : result.settings.gso ? (steps.every((s) => s.gso) ? 'on' : 'asked for but refused by the kernel') : 'off'
  const ran = result?.settings
  const bestScaling = best ? scaling(best, first) : null

  return (
    <AppLayout crumbs={[{ label: 'Bench' }]}
      overlay={<NotificationContainer errors={errors} successes={successes} onClose={removeNotification} />}>
      <section className={styles.head}>
        <h1 className={styles.title}>Bench</h1>
        {result && <span className={`${styles.tag} ${stateTone}`}><i />{STATE_LABELS[result.state]}</span>}
        <span className={styles.sub}>
          Sender ceiling of <b>{testerName}</b>, the ceiling for any run on it: uplink over loopback to a socket that never reads,
          {' '}1, 2, 4 … senders up to one per CPU. No core network or UPF involved.
        </span>
      </section>

      <section className={styles.panel} aria-label="Settings">
        <div className={styles.ph}>
          <h2>Settings</h2>
          {host && <span className={styles.aux}>host <span className={styles.mono}>{host}</span></span>}
        </div>
        <div className={styles.form}>
          <div className={styles.fld}>
            <label htmlFor="bench-tester">Tester</label>
            <select id="bench-tester" className={styles.select} value={testerId}
              disabled={isRunning || testers === null} onChange={(e) => pickTester(e.target.value)}>
              {testerKnown === false && <option value={testerId}>Removed tester</option>}
              {testers === null && <option value={testerId}>Loading testers…</option>}
              {testers?.map((t) => (
                <option key={t.id} value={t.id}>{t.name}{t.reachable === false ? ' (not answering)' : ''}</option>
              ))}
            </select>
          </div>
          <div className={styles.fld}>
            <label htmlFor="bench-size">Packet size (inner IP)</label>
            <div className={`${styles.inp} ${isRunning ? styles.inpOff : ''}`}>
              <input id="bench-size" type="number" min={64} max={9000}
                value={packetSize} onChange={(e) => setPacketSize(e.target.valueAsNumber)} disabled={isRunning} />
              <span className={styles.u}>bytes</span>
            </div>
          </div>
          <div className={styles.fld}>
            <label htmlFor="bench-seconds">Seconds per step</label>
            <div className={`${styles.inp} ${isRunning ? styles.inpOff : ''}`}>
              <input id="bench-seconds" type="number" min={1} max={30}
                value={stepSeconds} onChange={(e) => setStepSeconds(e.target.valueAsNumber)} disabled={isRunning} />
              <span className={styles.u}>s</span>
            </div>
          </div>
          <div className={styles.fld}>
            <label htmlFor="bench-gso">UDP GSO</label>
            <span className={styles.chk}>
              <input id="bench-gso" type="checkbox" checked={gso} onChange={(e) => setGso(e.target.checked)} disabled={isRunning} />
              On, as runs send
            </span>
          </div>
          <div className={styles.fld}>
            <span className={styles.fldLabel}>Planned senders</span>
            <span className={styles.value}>{planned.length > 0 ? planned.join(' · ') : '1 · 2 · 4 … one per CPU'}</span>
          </div>
        </div>
        <p className={styles.note}>
          Each step runs that many senders, each on its own CPU. UDP GSO hands the kernel up to 64 packets per send, which it (or a NIC with USO) splits;
          turn it off to see plain sendmmsg, as on a kernel older than 4.18.
        </p>
        <div className={styles.fa}>
          <button type="button" className={styles.btn} onClick={handleStart} disabled={isRunning || isStarting}>
            <svg viewBox="0 0 12 12" aria-hidden="true"><path d="M3 1.5v9l7-4.5z" fill="currentColor" /></svg>
            {isRunning ? 'Measuring…' : isStarting ? 'Starting…' : 'Start bench'}
          </button>
          <span>Takes about <span className={styles.mono}>{Math.ceil(estimateSec)} s</span>. Cannot start while a test run is active on {testerName}; they would compete for the CPUs.</span>
        </div>
      </section>

      {result?.error && <p className={styles.error}>{result.error}</p>}

      <section className={styles.meta} aria-label="Result summary">
        <div><span>One sender (≈ 1 CPU)</span><b className={styles.mono}>{first ? `${formatBps(first.bps)} · ${kpps(first.pps)} /s` : '—'}</b></div>
        <div><span>Best</span><b className={styles.mono}>{best ? `${formatBps(best.bps)} · ${best.senders} sender${best.senders === 1 ? '' : 's'}` : '—'}</b></div>
        <div><span>Peak packets/s</span><b className={styles.mono}>{peakPps ? `${(peakPps / 1e6).toFixed(2)} M` : '—'}</b></div>
        <div><span>Scaling at best</span><b className={styles.mono}>{bestScaling !== null ? `${Math.round(bestScaling * 100)} % of linear` : '—'}</b></div>
        <div><span>Send errors</span><b className={`${styles.mono} ${sendErrors > 0 ? styles.badText : ''}`}>{steps.length ? sendErrors.toLocaleString() : '—'}</b></div>
        <div><span>UDP GSO</span><b>{gsoText}</b></div>
        <div><span>Finished</span><b className={styles.mono}>{result?.state === 'running' ? 'measuring…' : stamp(result?.finishedAt)}</b></div>
      </section>

      <section className={styles.grid2}>
        <div className={styles.panel}>
          <div className={styles.ph}>
            <h2>Throughput vs senders</h2>
            <span className={styles.legend}><i />Measured</span>
            <span className={styles.legend}><i className={styles.dash} />Linear (n × one sender)</span>
            <span className={styles.aux}>Gbit/s</span>
          </div>
          {steps.length === 0
            ? <p className={styles.empty}>{isRunning ? 'Measuring the first step…' : result?.state === 'idle' || !result ? 'Start the bench to measure this host.' : 'No steps measured.'}</p>
            : <div className={styles.chart}><ScalingChart senders={planned} steps={steps} /></div>}
        </div>

        <div className={styles.panel}>
          <div className={styles.ph}>
            <h2>Measurements</h2>
            {ran && steps.length > 0 && <span className={`${styles.aux} ${styles.mono}`}>{ran.packetSize} B · {ran.stepSeconds} s / step</span>}
          </div>
          {steps.length === 0 && pending.length === 0
            ? <p className={styles.empty}>No measurements yet.</p>
            : (
              <div className={styles.scroll}>
                <table className={styles.tbl}>
                  <thead>
                    <tr><th>Senders</th><th>Packets/s</th><th>Throughput</th><th>Per sender</th><th>Scaling</th><th>Send errors</th></tr>
                  </thead>
                  <tbody>
                    {steps.map((s) => {
                      const isBest = s === best && steps.length > 1
                      const eff = scaling(s, first)
                      return (
                        <tr key={s.senders} className={isBest ? styles.best : undefined}>
                          <td>{s.senders}</td>
                          <td>{kpps(s.pps)} /s</td>
                          <td className={isBest ? styles.strong : undefined}>{formatBps(s.bps)}</td>
                          <td>{formatBps(s.bps / s.senders)}</td>
                          <td className={eff !== null && eff < 0.7 ? styles.warnText : undefined}>{efficiency(s, first)}</td>
                          <td className={s.sendErrors > 0 ? styles.badText : undefined}>{s.sendErrors.toLocaleString()}</td>
                        </tr>
                      )
                    })}
                    {pending.map((n, i) => (
                      <tr key={`pending-${n}`}>
                        <td>{n}</td>
                        <td colSpan={5} className={styles.pending}>{i === 0 ? 'Measuring…' : 'Waiting'}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          <p className={`${styles.note} ${styles.noteTop}`}>
            Scaling = packets/s ÷ (n × one sender). Packets per second, not bytes, are the limit: bigger packets give more Gbit/s.
          </p>
        </div>
      </section>
    </AppLayout>
  )
}
