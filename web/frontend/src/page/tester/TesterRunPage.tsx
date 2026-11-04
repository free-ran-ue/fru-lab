import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, Navigate, useParams } from 'react-router-dom'
import AppLayout from '../../components/layout/AppLayout'
import Button from '../../components/button/button'
import NotificationContainer from '../../components/notifications/NotificationContainer'
import { useNotifications } from '../../hooks/useNotifications'
import { api, extractErrorMessage } from '../../apiClient'
import type {
  TesterCauseCount, TesterCpLoopSnapshot, TesterCpPoint, TesterGnbTraffic, TesterRunSnapshot, TesterStageSnapshot,
  TesterTrafficDirection, TesterTrafficPoint, TesterUeSummary,
} from '../../api'
import { buildTesterStreamUrl, formatBps, formatBits, formatClock, formatLoss, formatMs } from './testerFormat'
import { useTesters } from './useTesters'
import styles from './tester.module.css'
import Pager from '../../components/pager/Pager'
import type { Page } from '../../components/pager/paging'

// The gNB table is paged on the tester: with 10 000 gNBs the whole list
// is about 3 MB, too much to send every second.
const GNB_PAGE = 50

// GNB_FILTERS are the gNB table's state filters, as gnbState values.
const GNB_FILTERS: { label: string, states: string }[] = [
  { label: 'All', states: '' },
  { label: 'Not up', states: 'pending,connecting,failed,lost' },
  { label: 'Failed or lost', states: 'failed,lost' },
]

// gnbQuery is the page request the tester takes (GnbPageRequest).
function gnbQuery(page: number, states: string) {
  return { gnbOffset: (page - 1) * GNB_PAGE, gnbLimit: GNB_PAGE, gnbState: states }
}

const RECONNECT_MS = 2000

const STATE_LABELS: Record<TesterRunSnapshot['state'], string> = {
  idle: 'No run yet',
  configuring: 'Configuring',
  n2: 'N2 setup',
  running: 'Running',
  stopping: 'Cleaning up',
  stopped: 'Stopped',
  failed: 'Failed',
}

// STATE_TITLES say more than the pill has room for.
const STATE_TITLES: Partial<Record<TesterRunSnapshot['state'], string>> = {
  configuring: 'Configuring gNB IPs',
  n2: 'N2 setup in progress',
  stopping: 'Cleaning up: deregistering UEs, closing N2',
}

// UE_STATES orders the UE summary chips: key, label, tone.
const UE_STATES: [keyof TesterUeSummary, string, 'ok' | 'bad' | 'accent' | 'idle'][] = [
  ['established', 'Established', 'ok'],
  ['establishing', 'Establishing', 'accent'],
  ['registered', 'Registered', 'accent'],
  ['registering', 'Registering', 'accent'],
  ['pending', 'Pending', 'idle'],
  ['failed', 'Failed', 'bad'],
  ['skipped', 'gNB down', 'idle'],
  ['cancelled', 'Cancelled', 'idle'],
  ['deregistering', 'Deregistering', 'accent'],
  ['deregistered', 'Deregistered', 'idle'],
]

const TONES = { ok: styles.stateOk, bad: styles.stateBad, accent: styles.stateAccent, warn: styles.stateWarn, idle: '' }

// gnbTraffic renders one gNB's received bytes and loss for a direction.
function gnbTraffic(t: TesterGnbTraffic | undefined, dir: 'ul' | 'dl'): string {
  if (!t) return '—'
  const tx = dir === 'ul' ? t.ulTxBytes : t.dlTxBytes
  const rx = dir === 'ul' ? t.ulRxBytes : t.dlRxBytes
  if (!tx) return '—'
  return `${formatBits(rx)} (${formatLoss(rx < tx ? 1 - rx / tx : 0)} lost)`
}

// formatPps renders a packet rate, e.g. 3.21 Mpps.
function formatPps(pps: number): string {
  if (pps >= 1e6) return `${(pps / 1e6).toFixed(2)} Mpps`
  if (pps >= 1e3) return `${(pps / 1e3).toFixed(1)} kpps`
  return `${Math.round(pps)} pps`
}

// niceStep rounds a grid step up to 1, 2, 2.5 or 5 x 10^n so the grid
// lines land on round rates.
function niceStep(v: number): number {
  const p = 10 ** Math.floor(Math.log10(v))
  return ([1, 2, 2.5, 5, 10].find((m) => m * p >= v) ?? 10) * p
}

// clock renders seconds since traffic started as m:ss or h:mm:ss.
function clock(sec: number): string {
  const s = Math.max(0, Math.round(sec))
  const h = Math.floor(s / 3600)
  const mm = Math.floor(s / 60) % 60
  const ss = String(s % 60).padStart(2, '0')
  return h > 0 ? `${h}:${String(mm).padStart(2, '0')}:${ss}` : `${mm}:${ss}`
}

// axisBps is a short rate for the chart axis, e.g. 500M or 1.5G.
function axisBps(bps: number): string {
  if (!bps) return '0'
  const [v, u] = bps >= 1e9 ? [bps / 1e9, 'G'] : bps >= 1e6 ? [bps / 1e6, 'M'] : [bps / 1e3, 'k']
  return `${Number(v.toFixed(2))}${u}`
}

// useWidth tracks an element's rendered width, so the chart is drawn in
// real pixels and its text stays small however wide the panel is.
function useWidth<T extends HTMLElement>(): [(el: T | null) => void, number] {
  const [el, setEl] = useState<T | null>(null)
  const [width, setWidth] = useState(0)
  useEffect(() => {
    if (!el) return
    const ro = new ResizeObserver(([e]) => setWidth(Math.floor(e.contentRect.width)))
    ro.observe(el)
    return () => ro.disconnect()
  }, [el])
  return [setEl, width]
}

// remScale is the root font size over 16px: the UI's sizes are in rem and
// follow the window, so the charts' margins follow it too.
function remScale(): number {
  return (parseFloat(getComputedStyle(document.documentElement).fontSize) || 16) / 16
}

// timeTicks picks round time ticks that fit plotW.
function timeTicks(t1: number, plotW: number): number[] {
  const span = Math.max(1, t1)
  const maxTicks = Math.max(2, Math.floor(plotW / 70))
  const step = [1, 2, 5, 10, 15, 30, 60, 120, 300, 600, 900, 1800, 3600, 7200, 10800, 21600, 43200, 86400]
    .find((s) => span / s <= maxTicks) ?? 86400
  const ticks: number[] = []
  for (let t = 0; t <= t1; t += step) ticks.push(t)
  return ticks
}

// sampleStep is the usual gap between two samples, in seconds.
function sampleStep(series: { t: number }[]): number | null {
  if (series.length < 2) return null
  const gaps = series.slice(1).map((p, i) => p.t - series[i].t).sort((a, b) => a - b)
  return gaps[Math.floor(gaps.length / 2)]
}

// ThroughputChart plots one direction's sent (Tx, dashed) and received
// (Rx, solid) rate over the whole run; the two lines overlap when nothing
// is lost. Downlink and uplink each get their own chart so their scales
// don't hide each other.
function ThroughputChart({ series, dir }: { series: TesterTrafficPoint[], dir: 'dl' | 'ul' }) {
  const [ref, W] = useWidth<HTMLDivElement>()
  const [hover, setHover] = useState<number | null>(null)
  const tx = (p: TesterTrafficPoint) => (dir === 'dl' ? p.dlTxBps : p.ulTxBps)
  const rx = (p: TesterTrafficPoint) => (dir === 'dl' ? p.dlRxBps : p.ulRxBps)
  if (series.length < 2) {
    return <div ref={ref} className={styles.wideChartEmpty}>The chart starts after two seconds of traffic.</div>
  }
  const u = remScale()
  const H = Math.max(Math.round(140 * u), Math.round((W * 160) / 760))
  const pad = { l: 40 * u, r: 10 * u, t: 12 * u, b: 22 * u }
  const t1 = series[series.length - 1].t
  const peak = Math.max(1000, ...series.flatMap((p) => [tx(p), rx(p)]))
  const gridStep = niceStep(peak / 4)
  const top = Math.ceil(peak / gridStep) * gridStep
  const plotW = Math.max(1, W - pad.l - pad.r)
  const span = Math.max(1, t1)
  const x = (t: number) => pad.l + (t / span) * plotW
  const y = (v: number) => H - pad.b - (v / top) * (H - pad.t - pad.b)
  const line = (f: (p: TesterTrafficPoint) => number) => series.map((p) => `${x(p.t).toFixed(1)},${y(f(p)).toFixed(1)}`).join(' ')
  const h = hover === null ? null : series[hover]
  const onMove = (ev: React.MouseEvent<SVGSVGElement>) => {
    const t = ((ev.clientX - ev.currentTarget.getBoundingClientRect().left - pad.l) / plotW) * span
    let best = 0
    series.forEach((p, k) => { if (Math.abs(p.t - t) < Math.abs(series[best].t - t)) best = k })
    setHover(best)
  }
  const grid = Array.from({ length: Math.round(top / gridStep) + 1 }, (_, k) => k * gridStep)
  const rxLine = dir === 'dl' ? styles.lineDl : styles.lineUl
  const rxDot = dir === 'dl' ? styles.dotDl : styles.dotUl
  const rxKey = dir === 'dl' ? styles.keyDl : styles.keyUl
  return (
    <div ref={ref} className={styles.wideChart} style={{ height: H || undefined }}>
      {W > 0 && (
        <svg width={W} height={H} className={styles.chart} role="img" aria-label={`${dir === 'dl' ? 'Downlink' : 'Uplink'} sent and received rate over the run`}
          onMouseMove={onMove} onMouseLeave={() => setHover(null)}>
          {grid.map((v) => (
            <g key={v}>
              <line x1={pad.l} x2={W - pad.r} y1={y(v)} y2={y(v)} className={v === 0 ? styles.axisLine : styles.chartGrid} />
              <text x={pad.l - 8} y={y(v) + 4} textAnchor="end" className={styles.chartAxis}>{axisBps(v)}</text>
            </g>
          ))}
          {timeTicks(t1, plotW).map((t) => (
            <text key={t} x={x(t)} y={H - 5} textAnchor={t === 0 ? 'start' : x(t) > W - pad.r - 24 ? 'end' : 'middle'} className={styles.chartAxis}>{clock(t)}</text>
          ))}
          <polyline points={line(tx)} className={styles.lineTx} />
          <polyline points={line(rx)} className={rxLine} />
          {h && (
            <g>
              <line x1={x(h.t)} x2={x(h.t)} y1={pad.t} y2={H - pad.b} className={styles.chartCursor} />
              <circle cx={x(h.t)} cy={y(tx(h))} r={3} className={styles.dotTx} />
              <circle cx={x(h.t)} cy={y(rx(h))} r={3} className={rxDot} />
            </g>
          )}
        </svg>
      )}
      {h && W > 0 && (
        <div className={styles.chartTip} style={{ left: Math.min(x(h.t) + 10, W - 200) }}>
          <b>{clock(h.t)}</b>
          <span><i className={styles.keyTx} />Sent {formatBps(tx(h))}</span>
          <span><i className={rxKey} />Received {formatBps(rx(h))}</span>
        </div>
      )}
    </div>
  )
}

// CpChart plots the control-plane loop's per-second registrations,
// deregistrations and failures from the loop's start.
function CpChart({ series }: { series: TesterCpPoint[] }) {
  const [ref, W] = useWidth<HTMLDivElement>()
  const [hover, setHover] = useState<number | null>(null)
  const u = remScale()
  const H = Math.round(150 * u)
  const pad = { l: 40 * u, r: 8 * u, t: 8 * u, b: 20 * u }
  if (series.length < 2) {
    return <div ref={ref} className={styles.chartEmpty}>The chart starts after two seconds of the loop.</div>
  }
  const t1 = series[series.length - 1].t
  const peak = Math.max(1, ...series.flatMap((p) => [p.regPerSec, p.deregPerSec, p.failPerSec]))
  const gridStep = niceStep(peak / 4)
  const top = Math.ceil(peak / gridStep) * gridStep
  const plotW = Math.max(1, W - pad.l - pad.r)
  const span = Math.max(1, t1)
  const x = (t: number) => pad.l + (t / span) * plotW
  const y = (v: number) => H - pad.b - (v / top) * (H - pad.t - pad.b)
  const line = (f: (p: TesterCpPoint) => number) => series.map((p) => `${x(p.t).toFixed(1)},${y(f(p)).toFixed(1)}`).join(' ')
  const h = hover === null ? null : series[hover]
  const onMove = (ev: React.MouseEvent<SVGSVGElement>) => {
    const t = ((ev.clientX - ev.currentTarget.getBoundingClientRect().left - pad.l) / plotW) * span
    let best = 0
    series.forEach((p, k) => { if (Math.abs(p.t - t) < Math.abs(series[best].t - t)) best = k })
    setHover(best)
  }
  const perSec = (v: number) => `${Number(v.toFixed(1)).toLocaleString()}/s`
  return (
    <div ref={ref} className={styles.chartBox}>
      {W > 0 && (
        <svg width={W} height={H} className={styles.chart} role="img" aria-label="Control-plane procedures per second over time"
          onMouseMove={onMove} onMouseLeave={() => setHover(null)}>
          {Array.from({ length: Math.round(top / gridStep) + 1 }, (_, k) => k * gridStep).map((v) => (
            <g key={v}>
              <line x1={pad.l} x2={W - pad.r} y1={y(v)} y2={y(v)} className={v === 0 ? styles.axisLine : styles.chartGrid} />
              <text x={pad.l - 6} y={y(v) + 3.5} textAnchor="end" className={styles.chartAxis}>{Number(v.toFixed(1))}</text>
            </g>
          ))}
          {timeTicks(t1, plotW).map((t) => (
            <text key={t} x={x(t)} y={H - 5} textAnchor={t === 0 ? 'start' : x(t) > W - pad.r - 24 ? 'end' : 'middle'} className={styles.chartAxis}>{clock(t)}</text>
          ))}
          <polyline points={line((p) => p.failPerSec)} className={styles.lineFail} />
          <polyline points={line((p) => p.deregPerSec)} className={styles.lineDereg} />
          <polyline points={line((p) => p.regPerSec)} className={styles.lineRx} />
          {h && <line x1={x(h.t)} x2={x(h.t)} y1={pad.t} y2={H - pad.b} className={styles.chartCursor} />}
        </svg>
      )}
      {h && W > 0 && (
        <div className={styles.chartTip} style={{ left: Math.min(x(h.t) + 10, W - 160) }}>
          <b>{clock(h.t)}</b>
          <span><i className={styles.swatchRx} />Registered {perSec(h.regPerSec)}</span>
          <span><i className={styles.swatchDereg} />Deregistered {perSec(h.deregPerSec)}</span>
          <span><i className={styles.swatchFail} />Failed {perSec(h.failPerSec)}</span>
        </div>
      )}
    </div>
  )
}

const failuresOf = (s: TesterStageSnapshot) => s.rejected + s.timedOut + s.failed

// ProcTable is the control-plane procedures: one row per stage.
function ProcTable({ rows }: { rows: [string, TesterStageSnapshot][] }) {
  return (
    <div className={styles.tableScroll}>
      <table className={styles.procTable}>
        <thead>
          <tr>
            <th>Stage</th><th>Attempts</th><th>Accepted</th><th>Rejected</th><th>Timed out</th>
            <th title="SCTP / connection errors">Conn. errors</th><th>In flight</th><th>Average</th><th>p95</th><th>p99</th>
          </tr>
        </thead>
        <tbody>
          {rows.map(([name, s]) => (
            <tr key={name} title={`Retries ${s.retries} · not started ${s.skipped} · max ${formatMs(s.maxMs)} · total time ${formatMs(s.totalTimeMs)}`}>
              <td>{name}</td>
              <td>{s.attempted.toLocaleString()}</td>
              <td>{s.accepted.toLocaleString()}</td>
              <td className={s.rejected > 0 ? styles.tdBad : undefined}>{s.rejected.toLocaleString()}</td>
              <td className={s.timedOut > 0 ? styles.tdBad : undefined}>{s.timedOut.toLocaleString()}</td>
              <td className={s.failed > 0 ? styles.tdBad : undefined}>{s.failed.toLocaleString()}</td>
              <td>{s.inFlight.toLocaleString()}</td>
              <td>{formatMs(s.avgMs)}</td>
              <td>{formatMs(s.p95Ms)}</td>
              <td>{formatMs(s.p99Ms)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

// mergeCauses adds up the causes of several stages, largest first.
function mergeCauses(stages: TesterStageSnapshot[]): TesterCauseCount[] {
  const m = new Map<string, number>()
  for (const s of stages) for (const c of s.causes) m.set(c.cause, (m.get(c.cause) ?? 0) + c.count)
  return [...m].map(([cause, count]) => ({ cause, count })).sort((a, b) => b.count - a.count || a.cause.localeCompare(b.cause))
}

function CauseBars({ causes }: { causes: TesterCauseCount[] }) {
  const total = causes.reduce((a, c) => a + c.count, 0)
  return (
    <>
      {causes.map((c) => (
        <div key={c.cause} className={styles.causeRow}>
          <div className={styles.causeTop}><span>{c.cause}</span><span className="num">{c.count.toLocaleString()}</span></div>
          <div className={`${styles.track} ${styles.causeFill}`}><i style={{ width: `${total ? (c.count / total) * 100 : 0}%` }} /></div>
        </div>
      ))}
    </>
  )
}

const CP_STATE: Record<TesterCpLoopSnapshot['state'], [string, keyof typeof TONES]> = {
  off: ['Off', 'idle'],
  waiting: ['Waiting for the throughput UEs', 'idle'],
  running: ['Running', 'ok'],
  stopped: ['Stopped', 'idle'],
}

// CpLoopPanel is the control-plane test: UEs registering and
// deregistering in a loop.
function CpLoopPanel({ cp }: { cp: TesterCpLoopSnapshot }) {
  const [label, tone] = CP_STATE[cp.state]
  const last = cp.series.length ? cp.series[cp.series.length - 1] : null
  const avg = (n: number) => (cp.durationSec > 0 ? `${(n / cp.durationSec).toFixed(1)}/s` : '—')
  const causes = mergeCauses([cp.registration, cp.deregistration, cp.cleanup])
  const tiles: [string, string][] = [
    ['Cycles completed', cp.cycles.toLocaleString()],
    ['Registered now', cp.registered.toLocaleString()],
    ['Running for', cp.startedAt ? formatClock(cp.durationSec) : '—'],
    ['Registrations · last s / avg', `${last ? `${last.regPerSec.toFixed(0)}/s` : '—'} / ${avg(cp.registration.accepted)}`],
    ['Deregistrations · last s / avg', `${last ? `${last.deregPerSec.toFixed(0)}/s` : '—'} / ${avg(cp.deregistration.accepted)}`],
    ['Failed · last second', last ? `${last.failPerSec.toFixed(0)}/s` : '—'],
  ]
  return (
    <section className={`${styles.panel} ${styles.panelFlush}`}>
      <div className={styles.panelHead}>
        <h2 className={styles.panelTitle}>Control-plane loop</h2>
        <span className={`${styles.state} ${TONES[tone]}`}>{label}</span>
      </div>
      <div className={styles.panelBody}>
        <p className={styles.panelText}>{cp.ues.toLocaleString()} UEs register, stay registered, deregister and start over; they have no PDU session.</p>
        <div className={styles.kpis}>
          {tiles.map(([k, v]) => (
            <div key={k} className={styles.kpi}><span className={styles.kpiLabel}>{k}</span><span className={`num ${styles.kpiValue}`} style={{ fontSize: '1.25rem' }}>{v}</span></div>
          ))}
        </div>
        <div className={styles.panelHead}>
          <span className={styles.legendKey}><i className={styles.keyDl} />Registrations/s</span>
          <span className={styles.legendKey}><i className={styles.keyDereg} style={{ background: 'repeating-linear-gradient(90deg, var(--violet) 0 0.25rem, transparent 0.25rem 0.4375rem)' }} />Deregistrations/s</span>
          <span className={styles.legendKey}><i className={styles.keyFail} />Failed/s</span>
        </div>
        <CpChart series={cp.series} />
      </div>
      <ProcTable rows={[['Registration', cp.registration], ['Deregistration', cp.deregistration]]} />
      {causes.length > 0 && (
        <div className={styles.panelBody} style={{ paddingTop: '1rem', borderTop: '1px solid var(--border-soft)', gap: '0.875rem' }}>
          <div className={styles.panelHead}><h3 className={styles.panelTitle}>Failure causes</h3><span className={`num ${styles.panelNote}`}>{causes.reduce((a, c) => a + c.count, 0).toLocaleString()} total</span></div>
          <CauseBars causes={causes} />
        </div>
      )}
    </section>
  )
}

// DIRECTION_ROWS are the per-direction numbers of the details table.
const DIRECTION_ROWS: [string, (d: TesterTrafficDirection) => string][] = [
  ['Sent', (d) => `${formatBps(d.txBps)} · ${formatPps(d.txPps)}`],
  ['Received', (d) => `${formatBps(d.rxBps)} · ${formatPps(d.rxPps)}`],
  ['Total sent / received', (d) => `${formatBits(d.txBytes)} / ${formatBits(d.rxBytes)}`],
  ['Loss', (d) => formatLoss(d.lossRate)],
  ['Latency p50 / p99', (d) => `${formatMs(d.latency.p50Ms)} / ${formatMs(d.latency.p99Ms)}`],
  ['Out of order', (d) => d.outOfOrder.toLocaleString()],
  ['Send errors', (d) => d.sendErrors.toLocaleString()],
  ['Misrouted', (d) => d.misrouted.toLocaleString()],
]

const ACTIVE_STATES: TesterRunSnapshot['state'][] = ['configuring', 'n2', 'running']

// useNow ticks every second while on, for the elapsed time.
function useNow(on: boolean): number {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    if (!on) return
    const t = window.setInterval(() => setNow(Date.now()), 1000)
    return () => window.clearInterval(t)
  }, [on])
  return now
}

const StopIcon = () => (
  <svg className={styles.btnIcon} viewBox="0 0 14 14" aria-hidden="true"><rect x="2" y="2" width="10" height="10" rx="1.5" fill="currentColor" /></svg>
)

// TesterRunDetailPage is the run on one tester, live.
export default function TesterRunDetailPage() {
  const { testerId = '' } = useParams()
  const { testers } = useTesters()
  const tester = testers?.find((t) => t.id === testerId)
  const { errors, successes, addError, addSuccess, removeNotification } = useNotifications()
  const [snapshot, setSnapshot] = useState<TesterRunSnapshot | null>(null)
  const [isConnected, setIsConnected] = useState(false)
  const [isStopping, setIsStopping] = useState(false)
  const [gnbPage, setGnbPage] = useState(1)
  const [gnbStates, setGnbStates] = useState('')
  // the page the stream should carry: read when it (re)connects, and sent
  // to it when the page or filter changes
  const query = useRef(gnbQuery(1, ''))
  const socketRef = useRef<WebSocket | null>(null)

  useEffect(() => {
    setSnapshot(null)
    setGnbPage(1)
    setGnbStates('')
    query.current = gnbQuery(1, '')
    api.testerByIdRun(testerId, 0, GNB_PAGE)
      .then((response) => setSnapshot(response.data))
      .catch((error) => addError(extractErrorMessage(error, 'Failed to load the run')))
  }, [addError, testerId])

  const showGnbs = useCallback((page: number, states: string) => {
    setGnbPage(page)
    setGnbStates(states)
    query.current = gnbQuery(page, states)
    const socket = socketRef.current
    if (socket?.readyState === WebSocket.OPEN) socket.send(JSON.stringify(query.current))
  }, [])

  // Keep one stream open while the page is mounted; reconnect after drops
  // (fru-tester restart, network blip) so the numbers never silently freeze.
  useEffect(() => {
    let socket: WebSocket | null = null
    let retryTimer: number | undefined
    let closedByPage = false

    const connect = () => {
      socket = new WebSocket(buildTesterStreamUrl(testerId, query.current))
      socketRef.current = socket
      socket.onopen = () => setIsConnected(true)
      socket.onmessage = (event) => setSnapshot(JSON.parse(event.data) as TesterRunSnapshot)
      socket.onclose = () => {
        setIsConnected(false)
        if (!closedByPage) retryTimer = window.setTimeout(connect, RECONNECT_MS)
      }
    }
    connect()
    return () => {
      closedByPage = true
      window.clearTimeout(retryTimer)
      socket?.close()
      socketRef.current = null
    }
  }, [testerId])

  const isActive = snapshot ? ACTIVE_STATES.includes(snapshot.state) : false
  const isCleaningUp = snapshot?.state === 'stopping'
  const now = useNow(isActive || isCleaningUp)

  // Stop during cleanup skips the deregistrations still pending.
  const handleStop = useCallback(async () => {
    if (isCleaningUp && !window.confirm('Skip the remaining deregistrations? SCTP is still closed and the IPs removed.')) return
    setIsStopping(true)
    try {
      await api.testerByIdRunStop(testerId)
      addSuccess(isCleaningUp ? 'Skipping the rest of the cleanup' : 'Stopping: deregistering UEs, then closing N2')
    } catch (error) {
      addError(extractErrorMessage(error, 'Failed to stop the run'))
    } finally {
      setIsStopping(false)
    }
  }, [addError, addSuccess, isCleaningUp, testerId])

  if (testers !== null && !tester) return <Navigate to="/tester/run" replace />

  const s = snapshot
  const hasRun = s !== null && s.state !== 'idle'
  const overlay = <NotificationContainer errors={errors} successes={successes} onClose={removeNotification} />
  const crumbs = [{ label: 'Runs', to: '/tester/run' }, { label: hasRun ? <span className="num">{s.runId}</span> : (tester?.name ?? 'Run') }]

  // the pill: the state, and while running the stage it is at
  let pillText = s ? STATE_LABELS[s.state] : ''
  if (s?.state === 'running') {
    const at = !s.n2.done ? 'N2 setup' : !s.registration.done ? 'Registration' : !s.pdu.done ? 'PDU sessions' : 'Traffic'
    pillText = `Running · ${at}`
  }
  const pillTone = !s ? '' : s.state === 'failed' ? styles.stateBad : s.state === 'stopping' ? styles.stateWarn : isActive ? styles.stateOk : ''

  const elapsed = s?.startedAt
    ? ((s.stoppedAt ? new Date(s.stoppedAt).getTime() : now) - new Date(s.startedAt).getTime()) / 1000
    : null

  return (
    <AppLayout crumbs={crumbs} overlay={overlay}>
      <section className={styles.pageHead}>
        <div className={styles.runHeadText}>
          <div className={styles.titleRow}>
            <h1 className={styles.pageTitle}>Run{hasRun && <> <span className="num">{s.runId}</span></>}</h1>
            {s && <span className={`${styles.state} ${styles.stateLg} ${pillTone}`} title={STATE_TITLES[s.state]}>{pillText}</span>}
          </div>
          {hasRun ? (
            <div className={styles.runMeta}>
              {tester && <span>Tester <b>{tester.name}</b></span>}
              <span>Profile <b>{s.profileName || '—'}</b></span>
              <span>Engine <b>{s.dataplane.engine || '—'}</b></span>
              <span>Started <b className="num">{s.startedAt ? new Date(s.startedAt).toLocaleString() : '—'}</b></span>
              <span>Elapsed <b className="num">{elapsed === null ? '—' : formatClock(elapsed)}</b></span>
            </div>
          ) : (
            <p className={styles.pageSub}>
              {tester && <>Tester <b>{tester.name}</b>. </>}No run has been started yet. <Link to="/tester">Set one up</Link>.
            </p>
          )}
        </div>
        <div className={styles.pageActions}>
          {s?.stopReason === 'maxDuration' && <span className={styles.state}>Stopped at max run time</span>}
          {!isConnected && <span className={`${styles.state} ${styles.stateBad}`}>Live updates disconnected</span>}
          <Button variant="danger" onClick={handleStop} disabled={!(isActive || isCleaningUp) || isStopping}>
            <StopIcon />{isStopping ? 'Stopping…' : isCleaningUp ? 'Skip cleanup' : 'Stop run'}
          </Button>
        </div>
      </section>

      {s?.error && <p className={styles.runError}>{s.error}</p>}

      {hasRun && <RunBody s={s} gnbPage={gnbPage} gnbStates={gnbStates} onGnbs={showGnbs} />}
    </AppLayout>
  )
}

// RunBody is everything under the run's header.
interface RunBodyProps {
  s: TesterRunSnapshot
  gnbPage: number // 1-based
  gnbStates: string // gnbState filter, '' for all
  onGnbs: (page: number, states: string) => void
}

function RunBody({ s, gnbPage, gnbStates, onGnbs: showGnbs }: RunBodyProps) {
  const cleaning = s.state === 'stopping' || s.state === 'stopped'
  const dp = s.dataplane

  // the gNB table: the page the tester sent, with its traffic in the same
  // order. A tester older than paging sends every gNB (no gnbTotal) and
  // the page is cut here.
  const paged = typeof s.gnbTotal === 'number'
  const wanted = gnbStates ? gnbStates.split(',') : null
  const matching = paged ? s.gnbs : s.gnbs.filter((g) => !wanted || wanted.includes(g.state))
  const gnbTotal = paged ? s.gnbTotal : matching.length
  const pages = Math.max(1, Math.ceil(gnbTotal / GNB_PAGE))
  const current = Math.min(gnbPage, pages)
  const start = paged ? s.gnbOffset : (current - 1) * GNB_PAGE
  const gnbRows = (paged ? matching : matching.slice(start, start + GNB_PAGE))
    .map((g, i) => ({ g, traffic: paged ? dp.gnbs[i] : dp.gnbs[g.index - 1] }))
  const pager: Page<unknown> = { items: gnbRows, page: current, pages, from: gnbRows.length ? start + 1 : 0, to: start + gnbRows.length }
  const counts: Record<string, number> = paged ? s.gnbCounts : s.gnbs.reduce<Record<string, number>>((m, g) => ({ ...m, [g.state]: (m[g.state] ?? 0) + 1 }), {})
  const allGnbs = Object.values(counts).reduce((a, b) => a + b, 0)
  const gnbCountText = [`${allGnbs.toLocaleString()} gNB${allGnbs === 1 ? '' : 's'}`,
    ...['up', 'connecting', 'pending', 'failed', 'lost', 'closed'].filter((st) => counts[st]).map((st) => `${counts[st].toLocaleString()} ${st}`)].join(' · ')

  // a filter can leave fewer pages than the one shown: go to the last
  useEffect(() => {
    if (paged && gnbPage > pages) showGnbs(pages, gnbStates)
  }, [paged, gnbPage, pages, gnbStates, showGnbs])
  const cpOn = ['waiting', 'running', 'stopped'].includes(s.cpLoop.state)

  // stage progress strip
  const stages: { name: string, st: TesterStageSnapshot, note: string }[] = [
    { name: 'Attach gNBs', st: s.n2, note: `${s.n2.accepted} / ${s.n2.expected}` },
    { name: 'Register UEs', st: s.registration, note: `${s.registration.accepted.toLocaleString()} / ${s.registration.expected.toLocaleString()}` },
    { name: 'PDU sessions', st: s.pdu, note: s.pdu.accepted.toLocaleString() },
    { name: 'Deregister', st: s.deregistration, note: s.deregistration.attempted === 0 && !cleaning ? 'on stop' : `${s.deregistration.accepted.toLocaleString()} / ${s.deregistration.expected.toLocaleString()}` },
  ]

  // key figures
  const reg = s.registration
  const pct = (n: number, of: number) => (of > 0 ? `${((n / of) * 100).toFixed(1)}%` : '—')
  const regRate = reg.totalTimeMs > 0 ? reg.accepted / (reg.totalTimeMs / 1000) : null
  const throughputStages = [s.n2, s.registration, s.pdu, s.deregistration, s.n2Release]
  const failures = throughputStages.reduce((a, st) => a + failuresOf(st), 0)
  const attempts = throughputStages.reduce((a, st) => a + st.attempted, 0)
  const kpis = [
    { label: 'Registered UEs', value: reg.accepted.toLocaleString(), unit: `/ ${reg.expected.toLocaleString()}`, sub: `${pct(reg.accepted, reg.expected)} of target` },
    { label: 'Registrations/s', value: regRate === null ? '—' : Math.round(regRate).toLocaleString(), unit: '/s', sub: regRate === null ? 'not started' : `average over ${formatClock(reg.totalTimeMs / 1000)}` },
    { label: 'PDU sessions', value: s.ues.established.toLocaleString(), unit: 'active', sub: `${pct(s.ues.established, reg.accepted)} of registered` },
    { label: 'Failures', value: failures.toLocaleString(), unit: 'total', sub: `${pct(failures, attempts)} of attempts`, bad: failures > 0 },
  ]

  const procRows: [string, TesterStageSnapshot][] = [
    ['N2 setup', s.n2],
    ['Registration', s.registration],
    ['PDU session establishment', s.pdu],
    [cpOn ? 'Deregistration (throughput UEs)' : 'Deregistration', s.deregistration],
  ]
  if (cleaning && cpOn) procRows.push(['Deregistration (control-plane UEs)', s.cpLoop.cleanup])
  if (cleaning) procRows.push(['gNB SCTP close', s.n2Release])

  const causes = mergeCauses(throughputStages)
  const causeTotal = causes.reduce((a, c) => a + c.count, 0)
  const step = sampleStep(dp.series)

  return (
    <>
      <section aria-label="Run progress" className={`${styles.panel} ${styles.stageStrip}`}>
        <div className={styles.stageGrid}>
          {stages.map(({ name, st, note }) => {
            const ratio = st.expected > 0 ? Math.min(1, st.accepted / st.expected) : 0
            const pending = st.attempted === 0 && !st.done
            const bar = ratio >= 1 ? 'var(--accent)'
              : ratio <= 0 ? 'var(--border)'
                : `linear-gradient(90deg, var(--accent) ${(ratio * 100).toFixed(1)}%, var(--border) ${(ratio * 100).toFixed(1)}%)`
            return (
              <div key={name} className={styles.stageItem}>
                <div className={styles.stageBar} style={{ background: bar }} />
                <div className={`${styles.stageLabel} ${pending ? styles.stagePending : ''}`}>
                  <span>{name}</span><span className="num">{note}</span>
                </div>
              </div>
            )
          })}
        </div>
      </section>

      <section aria-label="Key figures" className={`${styles.kpis} ${styles.kpisRun}`}>
        {kpis.map((k) => (
          <div key={k.label} className={styles.kpi}>
            <span className={styles.kpiLabel}>{k.label}</span>
            <span className={styles.kpiLine}><span className={`num ${styles.kpiBig}`}>{k.value}</span><span className={styles.kpiUnit}>{k.unit}</span></span>
            <span className={`num ${styles.kpiSub} ${k.bad ? styles.kpiSubBad : ''}`}>{k.sub}</span>
          </div>
        ))}
      </section>

      <section className={styles.split}>
        <div className={`${styles.panel} ${styles.span2}`}>
          <div className={styles.panelHead}>
            <h2 className={styles.panelTitle}>Data plane throughput</h2>
            <span className={styles.legendKey}><i className={styles.keyTx} />Sent</span>
            <span className={styles.legendKey}><i className={styles.keyUl} /><i className={styles.keyDl} />Received</span>
            <span className={styles.panelNote}>bps{step ? ` · ${step} s samples` : ''}</span>
          </div>
          <h3 className={styles.chartTitle}>Uplink <span className="num">{formatBps(dp.ul.rxBps)}</span></h3>
          <ThroughputChart series={dp.series} dir="ul" />
          <h3 className={styles.chartTitle}>Downlink <span className="num">{formatBps(dp.dl.rxBps)}</span></h3>
          <ThroughputChart series={dp.series} dir="dl" />
        </div>

        <div className={`${styles.panel} ${styles.panelKv}`}>
          <h2 className={styles.panelTitle}>Data plane</h2>
          <div className={styles.kvRow}><span>UL sent</span><span className="num">{formatBps(dp.ul.txBps)} · {formatPps(dp.ul.txPps)}</span></div>
          <div className={styles.kvRow}><span>UL received</span><span className="num">{formatBps(dp.ul.rxBps)} · {formatPps(dp.ul.rxPps)}</span></div>
          <div className={styles.kvRow}><span>DL sent</span><span className="num">{formatBps(dp.dl.txBps)} · {formatPps(dp.dl.txPps)}</span></div>
          <div className={styles.kvRow}><span>DL received</span><span className="num">{formatBps(dp.dl.rxBps)} · {formatPps(dp.dl.rxPps)}</span></div>
          <div className={styles.kvRow}><span>Loss UL / DL</span><span className="num">{formatLoss(dp.ul.lossRate)} / {formatLoss(dp.dl.lossRate)}</span></div>
          <div className={styles.kvRow}><span>DP CPUs</span><span className="num">{s.cpus.dpCpus || 'not pinned'}</span></div>
          <div className={styles.kvRow}><span>CP CPUs</span><span className="num">{s.cpus.cpCpus || 'not pinned'}</span></div>
        </div>
      </section>

      <section className={styles.split}>
        <div className={`${styles.panel} ${styles.panelFlush} ${styles.span2}`}>
          <div className={styles.panelHead}><h2 className={styles.panelTitle}>Control plane procedures</h2></div>
          <ProcTable rows={procRows} />
        </div>

        <div className={`${styles.panel} ${styles.panelKv}`}>
          <div className={styles.panelHead}>
            <h2 className={styles.panelTitle}>Failure causes</h2>
            <span className={`num ${styles.panelNote}`}>{causeTotal.toLocaleString()} total</span>
          </div>
          {causes.length === 0 ? <p className={styles.panelText}>No failures.</p> : <CauseBars causes={causes} />}
        </div>
      </section>

      <section className={`${styles.panel} ${styles.panelFlush}`}>
        <div className={styles.panelHead}>
          <h2 className={styles.panelTitle}>Data plane details</h2>
          <span className={`${styles.state} ${dp.activeUes ? styles.stateOk : ''}`}>{dp.activeUes.toLocaleString()} UEs sending</span>
        </div>
        <div className={styles.tableScroll}>
          <table className={styles.procTable}>
            <thead><tr><th /><th>Uplink · UE → N3 → UPF → N6</th><th>Downlink · N6 → UPF → N3 → UE</th></tr></thead>
            <tbody>
              {DIRECTION_ROWS.map(([label, f]) => (
                <tr key={label}><td>{label}</td><td>{f(dp.ul)}</td><td>{f(dp.dl)}</td></tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      {cpOn && <CpLoopPanel cp={s.cpLoop} />}

      <section className={`${styles.panel} ${styles.panelFlush}`}>
        <div className={styles.panelHead}><h2 className={styles.panelTitle}>UEs</h2></div>
        <div className={styles.panelBody}>
          <div className={styles.chips}>
            {UE_STATES.map(([key, label, tone]) => (
              <span key={key} className={`${styles.state} ${TONES[tone]}`}>{label} <span className="num">{s.ues[key].toLocaleString()}</span></span>
            ))}
          </div>
        </div>
        {s.failedUes.length > 0 && (
          <>
            <div className={styles.panelHead} style={{ borderTop: '1px solid var(--border-soft)' }}>
              <h3 className={styles.panelTitle}>
                Failed UEs{s.ues.failed > s.failedUes.length && ` (first ${s.failedUes.length} of ${s.ues.failed})`}
              </h3>
            </div>
            <div className={styles.tableScroll}>
              <table className={styles.dataTable}>
                <thead><tr><th>SUPI</th><th>gNB</th><th>Stage</th><th>Cause</th><th className={styles.r}>Attempts</th></tr></thead>
                <tbody>
                  {s.failedUes.map((f) => (
                    <tr key={f.supi}>
                      <td className="num">{f.supi}</td>
                      <td>{f.gnb}</td>
                      <td>{f.stage === 'pdu' ? 'PDU session' : 'Registration'}</td>
                      <td className="num">{f.cause}</td>
                      <td className={`num ${styles.r}`}>{f.attempts}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </>
        )}
      </section>

      <section className={`${styles.panel} ${styles.panelFlush}`}>
        <div className={styles.panelHead}>
          <h2 className={styles.panelTitle}>gNBs</h2>
          <span className={styles.gnbCounts}>{gnbCountText}</span>
          <label className={styles.gnbFilter}>
            Show
            <select value={gnbStates} onChange={(e) => showGnbs(1, e.target.value)}>
              {GNB_FILTERS.map((f) => <option key={f.label} value={f.states}>{f.label}</option>)}
            </select>
          </label>
        </div>
        <div className={styles.tableScroll}>
          <table className={styles.dataTable}>
            <thead>
              <tr>
                <th className={styles.r}>#</th><th>Name</th><th>N2 IP</th><th>State</th><th className={styles.r}>Attempts</th>
                <th className={styles.r}>Setup time</th><th className={styles.r}>Registered</th><th className={styles.r}>PDU sessions</th>
                <th className={styles.r}>UL received</th><th className={styles.r}>DL received</th><th>Last error</th>
              </tr>
            </thead>
            <tbody>
              {gnbRows.length === 0 && (
                <tr><td colSpan={11} className={styles.emptyCell}>{gnbStates ? 'No gNBs in these states.' : 'No gNBs.'}</td></tr>
              )}
              {gnbRows.map(({ g, traffic }) => {
                const tone = g.state === 'up' ? 'ok' : g.state === 'failed' || g.state === 'lost' ? 'bad' : g.state === 'connecting' ? 'accent' : 'idle'
                return (
                  <tr key={g.index}>
                    <td className={`num ${styles.r}`}>{g.index}</td>
                    <td>{g.name}</td>
                    <td className="num">{g.n2Ip}</td>
                    <td><span className={`${styles.state} ${TONES[tone]}`}>{g.state}</span></td>
                    <td className={`num ${styles.r}`}>{g.attempts}</td>
                    <td className={`num ${styles.r}`}>{g.state === 'up' || g.state === 'closed' ? formatMs(g.latencyMs) : '—'}</td>
                    <td className={`num ${styles.r}`}>{g.registered.toLocaleString()} / {g.ueCount.toLocaleString()}</td>
                    <td className={`num ${styles.r}`}>{g.established.toLocaleString()} / {g.ueCount.toLocaleString()}</td>
                    <td className={`num ${styles.r}`}>{gnbTraffic(traffic, 'ul')}</td>
                    <td className={`num ${styles.r}`}>{gnbTraffic(traffic, 'dl')}</td>
                    <td className={styles.tdWrap}>{g.cause || '—'}</td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
        <div className={styles.gnbPager}>
          <Pager page={pager} total={gnbTotal} onChange={(p) => showGnbs(p, gnbStates)} />
        </div>
      </section>
    </>
  )
}
