import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import Sidebar from '../../components/sidebar/Sidebar'
import Button from '../../components/button/button'
import NotificationContainer from '../../components/notifications/NotificationContainer'
import { useNotifications } from '../../hooks/useNotifications'
import { api, extractErrorMessage } from '../../apiClient'
import type {
  TesterDataplaneSnapshot, TesterGnbTraffic, TesterRunSnapshot, TesterStageSnapshot, TesterTrafficDirection,
  TesterTrafficPoint, TesterUeSummary,
} from '../../api'
import { buildTesterStreamUrl, formatBps, formatBytes, formatLoss, formatMs } from './testerFormat'
import styles from './tester.module.css'

const RECONNECT_MS = 2000

const STATE_LABELS: Record<TesterRunSnapshot['state'], string> = {
  idle: 'No run yet',
  configuring: 'Configuring gNB IPs',
  n2: 'N2 setup in progress',
  running: 'Running',
  stopping: 'Stopping',
  stopped: 'Stopped',
  failed: 'Failed',
}

// UE_STATES orders the UE summary chips: key, label, pill style.
const UE_STATES: [keyof TesterUeSummary, string, 'pillOk' | 'pillBad' | 'pillActive' | 'pillMuted'][] = [
  ['established', 'Established', 'pillOk'],
  ['establishing', 'Establishing', 'pillActive'],
  ['registered', 'Registered', 'pillActive'],
  ['registering', 'Registering', 'pillActive'],
  ['pending', 'Pending', 'pillMuted'],
  ['failed', 'Failed', 'pillBad'],
  ['skipped', 'gNB down', 'pillMuted'],
  ['cancelled', 'Cancelled', 'pillMuted'],
]

// gnbTraffic renders one gNB's received bytes and loss for a direction.
function gnbTraffic(t: TesterGnbTraffic | undefined, dir: 'ul' | 'dl'): string {
  if (!t) return '—'
  const tx = dir === 'ul' ? t.ulTxBytes : t.dlTxBytes
  const rx = dir === 'ul' ? t.ulRxBytes : t.dlRxBytes
  if (!tx) return '—'
  return `${formatBytes(rx)} (${formatLoss(rx < tx ? 1 - rx / tx : 0)} lost)`
}

// niceStep rounds a grid step up to 1, 2, 2.5 or 5 x 10^n so the grid
// lines land on round rates.
function niceStep(v: number): number {
  const p = 10 ** Math.floor(Math.log10(v))
  return ([1, 2, 2.5, 5, 10].find((m) => m * p >= v) ?? 10) * p
}

// axisBps is a short rate for the chart axis, e.g. 500M or 1.5G.
function axisBps(bps: number): string {
  if (!bps) return '0'
  const [v, u] = bps >= 1e9 ? [bps / 1e9, 'G'] : bps >= 1e6 ? [bps / 1e6, 'M'] : [bps / 1e3, 'k']
  return `${Number(v.toFixed(2))}${u}`
}

// useWidth tracks an element's rendered width, so the chart is drawn in
// real pixels and its text stays small however wide the card is.
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

type Dir = 'ul' | 'dl'

// RateChart plots one direction's sent (Tx) and received (Rx) rate.
function RateChart({ series, dir }: { series: TesterTrafficPoint[], dir: Dir }) {
  const [ref, W] = useWidth<HTMLDivElement>()
  const [hover, setHover] = useState<number | null>(null)
  const H = 150
  const pad = { l: 40, r: 8, t: 8, b: 20 }
  const tx = (p: TesterTrafficPoint) => (dir === 'ul' ? p.ulTxBps : p.dlTxBps)
  const rx = (p: TesterTrafficPoint) => (dir === 'ul' ? p.ulRxBps : p.dlRxBps)
  if (series.length < 2) {
    return <div ref={ref} className={styles.chartEmpty}>The chart starts after two seconds of traffic.</div>
  }
  const t0 = series[0].t
  const t1 = series[series.length - 1].t
  const peak = Math.max(1000, ...series.flatMap((p) => [tx(p), rx(p)]))
  const gridStep = niceStep(peak / 4)
  const top = Math.ceil(peak / gridStep) * gridStep
  const plotW = Math.max(1, W - pad.l - pad.r)
  const x = (t: number) => pad.l + ((t - t0) / Math.max(1, t1 - t0)) * plotW
  const y = (v: number) => H - pad.b - (v / top) * (H - pad.t - pad.b)
  const line = (f: (p: TesterTrafficPoint) => number) => series.map((p) => `${x(p.t).toFixed(1)},${y(f(p)).toFixed(1)}`).join(' ')
  const span = Math.max(1, t1 - t0)
  const step = [1, 2, 5, 10, 15, 30, 60, 120].find((s) => span / s <= Math.max(2, plotW / 90)) ?? 60
  const ticks: number[] = []
  for (let t = Math.ceil(t0 / step) * step; t <= t1; t += step) ticks.push(t)
  const h = hover === null ? null : series[hover]
  const onMove = (ev: React.MouseEvent<SVGSVGElement>) => {
    const px = ev.clientX - ev.currentTarget.getBoundingClientRect().left
    const t = t0 + ((px - pad.l) / plotW) * span
    let best = 0
    series.forEach((p, k) => { if (Math.abs(p.t - t) < Math.abs(series[best].t - t)) best = k })
    setHover(best)
  }
  return (
    <div ref={ref} className={styles.chartBox}>
      {W > 0 && (
        <svg width={W} height={H} className={styles.chart} role="img" aria-label={`${dir === 'ul' ? 'Uplink' : 'Downlink'} rate over time`}
          onMouseMove={onMove} onMouseLeave={() => setHover(null)}>
          {Array.from({ length: Math.round(top / gridStep) + 1 }, (_, k) => k * gridStep).map((v) => (
            <g key={v}>
              <line x1={pad.l} x2={W - pad.r} y1={y(v)} y2={y(v)} className={styles.chartGrid} />
              <text x={pad.l - 6} y={y(v) + 3.5} textAnchor="end" className={styles.chartAxis}>{axisBps(v)}</text>
            </g>
          ))}
          {ticks.map((t) => (
            <text key={t} x={x(t)} y={H - 5} textAnchor="middle" className={styles.chartAxis}>{t}s</text>
          ))}
          <polyline points={line(rx)} className={styles.lineRx} />
          <polyline points={line(tx)} className={styles.lineTx} />
          {h && (
            <g>
              <line x1={x(h.t)} x2={x(h.t)} y1={pad.t} y2={H - pad.b} className={styles.chartCursor} />
              <circle cx={x(h.t)} cy={y(rx(h))} r={3} className={styles.dotRx} />
              <circle cx={x(h.t)} cy={y(tx(h))} r={3} className={styles.dotTx} />
            </g>
          )}
        </svg>
      )}
      {h && W > 0 && (
        <div className={styles.chartTip} style={{ left: Math.min(x(h.t) + 10, W - 150) }}>
          <b>{Math.round(h.t)}s</b>
          <span><i className={styles.swatchTx} />Tx {formatBps(tx(h))}</span>
          <span><i className={styles.swatchRx} />Rx {formatBps(rx(h))}</span>
        </div>
      )}
    </div>
  )
}

function Stat({ label, value }: { label: string, value: React.ReactNode }) {
  return <div className={styles.stat}><span>{label}</span><b>{value}</b></div>
}

// DirectionPanel is one direction: live Tx/Rx, its chart and its numbers.
function DirectionPanel({ dir, d, series }: { dir: Dir, d: TesterTrafficDirection, series: TesterTrafficPoint[] }) {
  const lossy = d.lossRate > 0.001
  return (
    <div className={styles.dirPanel}>
      <div className={styles.dirHead}>
        <div>
          <h4 className={styles.dirTitle}>{dir === 'ul' ? 'Uplink' : 'Downlink'}</h4>
          <span className={styles.dirPath}>{dir === 'ul' ? 'UE → N3 → UPF → N6' : 'N6 → UPF → N3 → UE'}</span>
        </div>
        <span className={`${styles.pill} ${lossy ? styles.pillBad : styles.pillOk}`}>loss {formatLoss(d.lossRate)}</span>
      </div>
      <div className={styles.legend}>
        <span><i className={styles.swatchTx} />Tx <b>{formatBps(d.txBps)}</b></span>
        <span><i className={styles.swatchRx} />Rx <b>{formatBps(d.rxBps)}</b></span>
      </div>
      <RateChart series={series} dir={dir} />
      <div className={styles.statGrid}>
        <Stat label="Packets/s Tx / Rx" value={`${Math.round(d.txPps).toLocaleString()} / ${Math.round(d.rxPps).toLocaleString()}`} />
        <Stat label="Total Tx / Rx" value={`${formatBytes(d.txBytes)} / ${formatBytes(d.rxBytes)}`} />
        <Stat label="Latency p50 / p99" value={`${formatMs(d.latency.p50Ms)} / ${formatMs(d.latency.p99Ms)}`} />
        <Stat label="Out of order" value={d.outOfOrder.toLocaleString()} />
        <Stat label="Send errors" value={d.sendErrors.toLocaleString()} />
        {d.misrouted > 0 && <Stat label="Misrouted" value={d.misrouted.toLocaleString()} />}
      </div>
    </div>
  )
}

function DataplaneCard({ dp }: { dp: TesterDataplaneSnapshot }) {
  return (
    <section className={styles.card}>
      <div className={styles.stageTop}>
        <h3 className={styles.cardTitle}>Data plane</h3>
        <span className={`${styles.pill} ${dp.activeUes ? styles.pillActive : styles.pillMuted}`}>{dp.activeUes} UEs sending</span>
      </div>
      <div className={styles.dirRow}>
        <DirectionPanel dir="ul" d={dp.ul} series={dp.series} />
        <DirectionPanel dir="dl" d={dp.dl} series={dp.series} />
      </div>
    </section>
  )
}

const ACTIVE_STATES: TesterRunSnapshot['state'][] = ['configuring', 'n2', 'running']

function StageCard({ title, stage }: { title: string, stage: TesterStageSnapshot }) {
  const pct = (n: number) => (stage.expected ? `${(n / stage.expected) * 100}%` : '0%')
  return (
    <section className={styles.card}>
      <div className={styles.stageTop}>
        <h3 className={styles.cardTitle}>{title}</h3>
        <span className={`${styles.pill} ${stage.done ? styles.pillOk : styles.pillActive}`}>
          {stage.done ? 'Done' : stage.attempted === 0 ? 'Waiting' : `${stage.inFlight} in flight`}
        </span>
      </div>
      <div className={styles.bigNumber}>
        {stage.accepted}<small> / {stage.expected} accepted</small>
      </div>
      <div className={styles.bar} aria-hidden="true">
        <span className={styles.barOk} style={{ width: pct(stage.accepted) }} />
        <span className={styles.barBad} style={{ width: pct(stage.rejected + stage.timedOut + stage.failed) }} />
        <span className={styles.barActive} style={{ width: pct(stage.inFlight) }} />
      </div>
      <dl className={styles.kv}>
        <dt>Rejected</dt><dd>{stage.rejected}</dd>
        <dt>Timed out</dt><dd>{stage.timedOut}</dd>
        <dt>Connection errors</dt><dd>{stage.failed}</dd>
        <dt>Retries</dt><dd>{stage.retries}</dd>
        <dt>Not started</dt><dd>{stage.skipped}</dd>
        <dt>Total time</dt><dd>{formatMs(stage.totalTimeMs)}</dd>
        <dt>Average</dt><dd>{formatMs(stage.avgMs)}</dd>
        <dt>p50 / p95 / p99</dt><dd>{formatMs(stage.p50Ms)} / {formatMs(stage.p95Ms)} / {formatMs(stage.p99Ms)}</dd>
        <dt>Max</dt><dd>{formatMs(stage.maxMs)}</dd>
      </dl>
      {stage.causes.length > 0 && (
        <div className={styles.causes}>
          <h4>Failure causes</h4>
          <ul>
            {stage.causes.map((c) => <li key={c.cause}><span className={styles.mono}>{c.cause}</span> × {c.count}</li>)}
          </ul>
        </div>
      )}
    </section>
  )
}

export default function TesterRunPage() {
  const { errors, successes, addError, addSuccess, removeNotification } = useNotifications()
  const [snapshot, setSnapshot] = useState<TesterRunSnapshot | null>(null)
  const [isConnected, setIsConnected] = useState(false)
  const [isStopping, setIsStopping] = useState(false)

  useEffect(() => {
    api.testerRunGet()
      .then((response) => setSnapshot(response.data))
      .catch((error) => addError(extractErrorMessage(error, 'Failed to load the run')))
  }, [addError])

  // Keep one stream open while the page is mounted; reconnect after drops
  // (fru-tester restart, network blip) so the numbers never silently freeze.
  useEffect(() => {
    let socket: WebSocket | null = null
    let retryTimer: number | undefined
    let closedByPage = false

    const connect = () => {
      socket = new WebSocket(buildTesterStreamUrl())
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
    }
  }, [])

  const handleStop = useCallback(async () => {
    setIsStopping(true)
    try {
      await api.testerRunStop()
      addSuccess('Stopping: closing N2 and removing gNB IPs')
    } catch (error) {
      addError(extractErrorMessage(error, 'Failed to stop the run'))
    } finally {
      setIsStopping(false)
    }
  }, [addError, addSuccess])

  const isActive = snapshot ? ACTIVE_STATES.includes(snapshot.state) : false

  return (
    <div className={styles.layout}>
      <NotificationContainer errors={errors} successes={successes} onClose={removeNotification} />
      <Sidebar />

      <main className={styles.content}>
        <header className={styles.header}>
          <div>
            <h2 className={styles.title}>Throughput Tester · Run</h2>
            <p className={styles.subtitle}>
              {snapshot && snapshot.state !== 'idle'
                ? <>Run <span className={styles.mono}>{snapshot.runId}</span> · {snapshot.profileName}{snapshot.startedAt && ` · started ${new Date(snapshot.startedAt).toLocaleString()}`}</>
                : <>No run has been started yet. <Link to="/tester">Set one up</Link>.</>}
            </p>
          </div>
          <div className={styles.headerActions}>
            {snapshot && <span className={`${styles.pill} ${isActive ? styles.pillActive : snapshot.state === 'failed' ? styles.pillBad : styles.pillMuted}`}>{STATE_LABELS[snapshot.state]}</span>}
            {!isConnected && <span className={`${styles.pill} ${styles.pillBad}`}>Live updates disconnected</span>}
            <Button variant="danger" onClick={handleStop} disabled={!isActive || isStopping}>
              {isStopping ? 'Stopping…' : 'Stop'}
            </Button>
          </div>
        </header>

        {snapshot?.error && <p className={styles.runError}>{snapshot.error}</p>}

        {snapshot && snapshot.state !== 'idle' && (
          <>
            <div className={styles.stageRow}>
              <StageCard title="N2 setup" stage={snapshot.n2} />
              <StageCard title="Registration" stage={snapshot.registration} />
              <StageCard title="PDU session" stage={snapshot.pdu} />
            </div>

            <DataplaneCard dp={snapshot.dataplane} />

            <section className={styles.card}>
              <h3 className={styles.cardTitle}>UEs</h3>
              <div className={styles.ueChips}>
                {UE_STATES.map(([key, label, tone]) => (
                  <span key={key} className={`${styles.pill} ${styles[tone]}`}>{label} {snapshot.ues[key]}</span>
                ))}
              </div>
              {snapshot.failedUes.length > 0 && (
                <div className={styles.tableWrap}>
                  <h4 className={styles.subTitle}>
                    Failed UEs{snapshot.ues.failed > snapshot.failedUes.length && ` (first ${snapshot.failedUes.length} of ${snapshot.ues.failed})`}
                  </h4>
                  <table className={styles.table}>
                    <thead>
                      <tr><th>SUPI</th><th>gNB</th><th>Stage</th><th>Cause</th><th>Attempts</th></tr>
                    </thead>
                    <tbody>
                      {snapshot.failedUes.map((f) => (
                        <tr key={f.supi}>
                          <td className={styles.mono}>{f.supi}</td>
                          <td>{f.gnb}</td>
                          <td>{f.stage === 'pdu' ? 'PDU session' : 'Registration'}</td>
                          <td className={styles.mono}>{f.cause}</td>
                          <td>{f.attempts}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </section>

            <section className={styles.card}>
              <h3 className={styles.cardTitle}>gNBs</h3>
              <div className={styles.tableWrap}>
                <table className={styles.table}>
                  <thead>
                    <tr><th>#</th><th>Name</th><th>N2 IP</th><th>State</th><th>Attempts</th><th>Setup time</th><th>Registered</th><th>PDU sessions</th><th>UL received</th><th>DL received</th><th>Last error</th></tr>
                  </thead>
                  <tbody>
                    {snapshot.gnbs.map((g) => (
                      <tr key={g.index}>
                        <td>{g.index}</td>
                        <td>{g.name}</td>
                        <td className={styles.mono}>{g.n2Ip}</td>
                        <td><span className={`${styles.pill} ${g.state === 'up' ? styles.pillOk : g.state === 'failed' || g.state === 'lost' ? styles.pillBad : g.state === 'connecting' ? styles.pillActive : styles.pillMuted}`}>{g.state}</span></td>
                        <td>{g.attempts}</td>
                        <td>{g.state === 'up' || g.state === 'closed' ? formatMs(g.latencyMs) : '—'}</td>
                        <td>{g.registered} / {g.ueCount}</td>
                        <td>{g.established} / {g.ueCount}</td>
                        <td>{gnbTraffic(snapshot.dataplane.gnbs[g.index - 1], 'ul')}</td>
                        <td>{gnbTraffic(snapshot.dataplane.gnbs[g.index - 1], 'dl')}</td>
                        <td className={styles.causeCell}>{g.cause || '—'}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </section>
          </>
        )}
      </main>
    </div>
  )
}
