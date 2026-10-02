import { useCallback, useEffect, useState } from 'react'
import Sidebar from '../../components/sidebar/Sidebar'
import Button from '../../components/button/button'
import NotificationContainer from '../../components/notifications/NotificationContainer'
import { useNotifications } from '../../hooks/useNotifications'
import { api, extractErrorMessage } from '../../apiClient'
import type { TesterBenchResult, TesterBenchStep } from '../../api'
import { formatBps } from './testerFormat'
import styles from './tester.module.css'

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

export default function TesterBenchPage() {
  const { errors, successes, addError, addSuccess, removeNotification } = useNotifications()
  const [result, setResult] = useState<TesterBenchResult | null>(null)
  const [packetSize, setPacketSize] = useState(1400)
  const [stepSeconds, setStepSeconds] = useState(3)
  const [isStarting, setIsStarting] = useState(false)

  const load = useCallback(async () => {
    try {
      const response = await api.testerBenchGet()
      setResult(response.data)
    } catch (error) {
      addError(extractErrorMessage(error, 'Failed to load the bench'))
    }
  }, [addError])

  useEffect(() => { load() }, [load])

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
      const response = await api.testerBenchStart({ packetSize, stepSeconds })
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
  const maxBps = Math.max(1, ...steps.map((s) => s.bps))
  const planned = result?.plannedSenders ?? []
  const pending = planned.slice(steps.length)
  const estimateSec = (result?.cpus ? planned.length || 1 : 1) * (stepSeconds + 0.5)
  const pillTone = result?.state === 'failed' ? styles.pillBad : result?.state === 'done' ? styles.pillOk : isRunning ? styles.pillActive : styles.pillMuted

  return (
    <div className={styles.layout}>
      <NotificationContainer errors={errors} successes={successes} onClose={removeNotification} />
      <Sidebar />

      <main className={styles.content}>
        <header className={styles.header}>
          <div>
            <h2 className={styles.title}>Throughput Tester · Bench</h2>
            <p className={styles.subtitle}>
              How fast can this machine&apos;s fru-tester send? Uplink packets go over loopback to a socket that never reads them,
              with 1, 2, 4 … senders up to one per CPU. No core network or UPF is involved, so this is the tester&apos;s own ceiling.
            </p>
          </div>
          <div className={styles.headerActions}>
            {result && <span className={`${styles.pill} ${pillTone}`}>{STATE_LABELS[result.state]}</span>}
          </div>
        </header>

        <section className={styles.card}>
          <h3 className={styles.cardTitle}>Settings</h3>
          <div className={styles.fieldGrid}>
            <div className={styles.field}>
              <label htmlFor="bench-size">Packet size (bytes, inner IP)</label>
              <input id="bench-size" className={styles.input} type="number" min={64} max={1400}
                value={packetSize} onChange={(e) => setPacketSize(e.target.valueAsNumber)} disabled={isRunning} />
            </div>
            <div className={styles.field}>
              <label htmlFor="bench-seconds">Seconds per step</label>
              <input id="bench-seconds" className={styles.input} type="number" min={1} max={30}
                value={stepSeconds} onChange={(e) => setStepSeconds(e.target.valueAsNumber)} disabled={isRunning} />
            </div>
          </div>
          <p className={styles.hint}>
            {result ? <>This host: <b>{result.cpus} CPUs</b>{result.kernel && <>, kernel <span className={styles.mono}>{result.kernel}</span></>}. </> : null}
            A bench takes about {Math.ceil(estimateSec)} s and cannot run while a test run is active (they would compete for the CPUs).
          </p>
          <div style={{ marginTop: '1rem' }}>
            <Button onClick={handleStart} disabled={isRunning || isStarting}>
              {isRunning ? 'Measuring…' : isStarting ? 'Starting…' : 'Start bench'}
            </Button>
          </div>
        </section>

        {result?.error && <p className={styles.runError}>{result.error}</p>}

        {(steps.length > 0 || isRunning) && (
          <section className={styles.card}>
            <h3 className={styles.cardTitle}>Result</h3>
            {best && first && (
              <p className={styles.subtitle} style={{ maxWidth: 'none', marginBottom: '1rem' }}>
                One sender (about one CPU): <b>{formatBps(first.bps)}</b> ({kpps(first.pps)} packets/s).
                {' '}Best: <b>{formatBps(best.bps)}</b> with {best.senders} senders, {efficiency(best, first)} of linear scaling.
                {' '}Packets per second, not bytes, are the limit: bigger packets give more Gbps.
              </p>
            )}
            <div className={styles.tableWrap}>
              <table className={styles.table}>
                <thead>
                  <tr><th>Senders</th><th>Packets/s</th><th>Throughput</th><th style={{ width: '40%' }} /><th>Per sender</th><th>Scaling</th><th>Send errors</th></tr>
                </thead>
                <tbody>
                  {steps.map((s) => (
                    <tr key={s.senders}>
                      <td>{s.senders}</td>
                      <td className={styles.mono}>{kpps(s.pps)}</td>
                      <td className={styles.mono}>{formatBps(s.bps)}</td>
                      <td>
                        <div style={{ height: 10, borderRadius: 5, background: '#eef2ff' }}>
                          <div style={{ width: `${(s.bps / maxBps) * 100}%`, height: '100%', borderRadius: 5, background: '#4f46e5' }} />
                        </div>
                      </td>
                      <td className={styles.mono}>{formatBps(s.bps / s.senders)}</td>
                      <td className={styles.mono}>{efficiency(s, first)}</td>
                      <td className={styles.mono}>{s.sendErrors}</td>
                    </tr>
                  ))}
                  {isRunning && pending.map((n, i) => (
                    <tr key={`pending-${n}`}>
                      <td>{n}</td>
                      <td colSpan={6} className={styles.hint} style={{ margin: 0 }}>{i === 0 ? 'Measuring…' : 'Waiting'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </section>
        )}
      </main>
    </div>
  )
}
