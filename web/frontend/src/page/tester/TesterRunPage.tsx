import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import Sidebar from '../../components/sidebar/Sidebar'
import Button from '../../components/button/button'
import NotificationContainer from '../../components/notifications/NotificationContainer'
import { useNotifications } from '../../hooks/useNotifications'
import { api, extractErrorMessage } from '../../apiClient'
import type { TesterRunSnapshot, TesterStageSnapshot, TesterUeSummary } from '../../api'
import { buildTesterStreamUrl, formatMs } from './testerFormat'
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
                    <tr><th>#</th><th>Name</th><th>N2 IP</th><th>State</th><th>Attempts</th><th>Setup time</th><th>Registered</th><th>PDU sessions</th><th>Last error</th></tr>
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
