import { useCallback, useEffect, useState } from 'react'
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

// splitUnit splits "61.8 Gbps" into its number and unit for a KPI tile.
function splitUnit(text: string): [string, string] {
  const i = text.lastIndexOf(' ')
  return i < 0 ? [text, ''] : [text.slice(0, i), text.slice(i + 1)]
}

// The bench runs on the tester in fru-lab's config.yaml (/api/tester/bench).
export default function TesterBenchPage() {
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

  useEffect(() => {
    api.testerByIdSystem(DEFAULT_TESTER_ID)
      .then((response) => setSystem(response.data))
      .catch(() => setSystem(null))
  }, [])

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
      const response = await api.testerBenchStart({ packetSize, stepSeconds, gso })
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
  const maxBps = Math.max(1, ...steps.map((s) => s.bps))
  const planned = result?.plannedSenders ?? []
  const pending = isRunning ? planned.slice(steps.length) : []
  const estimateSec = (result?.cpus ? planned.length || 1 : 1) * (stepSeconds + 0.5)
  const stateTone = result?.state === 'failed' ? styles.stateBad : result?.state === 'done' ? styles.stateOk : isRunning ? styles.stateActive : ''
  const testerName = testerLabel(testers, DEFAULT_TESTER_ID)
  const [peakValue, peakUnit] = splitUnit(best ? formatBps(best.bps) : '')
  const nics = system?.topology.interfaces ?? []

  return (
    <AppLayout crumbs={[{ label: 'Bench' }]}
      overlay={<NotificationContainer errors={errors} successes={successes} onClose={removeNotification} />}>
      <section className={styles.head}>
        <div className={styles.headText}>
          <h1 className={styles.title}>Host data-plane bench</h1>
          <p className={styles.subtitle}>
            How much this host can send with no core attached: the ceiling for any run on <b>{testerName}</b>.
          </p>
        </div>
        <div className={styles.headActions}>
          {result && <span className={`${styles.state} ${stateTone}`}>{STATE_LABELS[result.state]}</span>}
          <button type="button" className={styles.btn} onClick={handleStart} disabled={isRunning || isStarting}>
            <svg viewBox="0 0 12 12" aria-hidden="true"><path d="M3 1.5v9l7-4.5z" fill="currentColor" /></svg>
            {isRunning ? 'Measuring…' : isStarting ? 'Starting…' : 'Run bench'}
          </button>
        </div>
      </section>

      {result?.error && <p className={styles.error}>{result.error}</p>}

      <section className={styles.body}>
        <div className={`${styles.panel} ${styles.side}`}>
          <div className={styles.panelH}><h2>Settings</h2></div>
          <div className={styles.panelB}>
            <div className={styles.fld}>
              <span className={styles.fieldLabel}>Senders</span>
              <span className={styles.value}>{planned.length > 0 ? planned.join(', ') : '1, 2, 4 … one per CPU'}</span>
              <span className={styles.hint}>one step per count, each on its own CPU</span>
            </div>
            <div className={styles.fld}>
              <label htmlFor="bench-size">Packet size (bytes, inner IP)</label>
              <input id="bench-size" className={styles.in} type="number" min={64} max={9000}
                value={packetSize} onChange={(e) => setPacketSize(e.target.valueAsNumber)} disabled={isRunning} />
            </div>
            <div className={styles.fld}>
              <label htmlFor="bench-seconds">Seconds per step</label>
              <input id="bench-seconds" className={styles.in} type="number" min={1} max={30}
                value={stepSeconds} onChange={(e) => setStepSeconds(e.target.valueAsNumber)} disabled={isRunning} />
              <span className={styles.hint}>a bench takes about {Math.ceil(estimateSec)} s</span>
            </div>
            <div className={styles.fld}>
              <label className={styles.check}>
                <input type="checkbox" checked={gso} onChange={(e) => setGso(e.target.checked)} disabled={isRunning} />
                UDP GSO
              </label>
              <span className={styles.hint}>
                Hands the kernel up to 64 packets per send, as runs do; turn it off to see plain sendmmsg, as on a kernel older than 4.18.
              </span>
            </div>
            {(result || system) && (
              <div className={styles.host}>
                <span>This host</span>
                <span className="num">
                  {[
                    result?.cpus ? `${result.cpus} CPUs` : '',
                    system && system.topology.nodes.length > 0 ? `${system.topology.nodes.length} NUMA` : '',
                    nics.map((n) => (n.driver ? `${n.name} (${n.driver})` : n.name)).join(', '),
                  ].filter(Boolean).join(' · ')}
                </span>
                {result?.kernel && <span className="num">kernel {result.kernel}</span>}
              </div>
            )}
            <span className={styles.hint}>
              Uplink packets go over loopback to a socket that never reads them, so no core network or UPF is involved.
              A bench cannot run while a test run is active (they would compete for the CPUs).
            </span>
          </div>
        </div>

        <div className={styles.main}>
          <div className={styles.kpis}>
            <div className={styles.kpi}>
              <span className={styles.kpiLabel}>Peak throughput</span>
              <span className={styles.kpiValue}><span className="num">{best ? peakValue : '—'}</span><span className={styles.kpiUnit}>{best ? peakUnit : ''}</span></span>
            </div>
            <div className={styles.kpi}>
              <span className={styles.kpiLabel}>Peak packets/s</span>
              <span className={styles.kpiValue}><span className="num">{peakPps ? (peakPps / 1e6).toFixed(2) : '—'}</span><span className={styles.kpiUnit}>{peakPps ? 'Mpps' : ''}</span></span>
            </div>
            <div className={styles.kpi}>
              <span className={styles.kpiLabel}>Send errors</span>
              <span className={styles.kpiValue}><span className="num">{steps.length ? sendErrors.toLocaleString() : '—'}</span></span>
            </div>
          </div>

          <div className={`${styles.panel} ${styles.scale}`}>
            <div className={styles.scaleH}><h2>Scaling</h2><span>Gbps by number of senders</span></div>
            {steps.length === 0 && pending.length === 0 && (
              <span className={styles.hint}>{result?.state === 'idle' || !result ? 'Run the bench to measure this host.' : 'No steps measured.'}</span>
            )}
            {steps.map((s) => (
              <div key={s.senders} className={styles.bar}>
                <span className={`num ${styles.barN}`}>{s.senders} ×</span>
                <div className={styles.track}><div className={styles.fill} style={{ width: `${(s.bps / maxBps) * 100}%` }} /></div>
                <span className={`num ${styles.barV}`}>{formatBps(s.bps)}</span>
              </div>
            ))}
            {pending.map((n, i) => (
              <div key={`pending-${n}`} className={styles.bar}>
                <span className={`num ${styles.barN}`}>{n} ×</span>
                <div className={styles.track} />
                <span className={styles.barPending}>{i === 0 ? 'Measuring…' : 'Waiting'}</span>
              </div>
            ))}
          </div>

          {(steps.length > 0 || pending.length > 0) && (
            <div className={`${styles.panel} ${styles.tablePanel}`}>
              <div className={styles.tableWrap}>
                <table className={styles.table}>
                  <thead>
                    <tr>
                      <th className={`${styles.th} ${styles.left}`}>Senders</th>
                      <th className={styles.th}>Throughput</th>
                      <th className={styles.th}>Packets/s</th>
                      <th className={styles.th}>Per sender</th>
                      <th className={styles.th}>Scaling</th>
                      <th className={styles.th}>Send errors</th>
                    </tr>
                  </thead>
                  <tbody>
                    {steps.map((s) => (
                      <tr key={s.senders}>
                        <td className={`${styles.td} ${styles.left}`}>{s.senders}</td>
                        <td className={styles.td}>{formatBps(s.bps)}</td>
                        <td className={styles.td}>{(s.pps / 1e6).toFixed(2)} M</td>
                        <td className={styles.td}>{formatBps(s.bps / s.senders)}</td>
                        <td className={styles.td}>{efficiency(s, first)}</td>
                        <td className={styles.td}>{s.sendErrors.toLocaleString()}</td>
                      </tr>
                    ))}
                    {pending.map((n, i) => (
                      <tr key={`pending-${n}`}>
                        <td className={`${styles.td} ${styles.left}`}>{n}</td>
                        <td colSpan={5} className={`${styles.td} ${styles.left} ${styles.dim}`}>{i === 0 ? 'Measuring…' : 'Waiting'}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              {best && first && (
                <p className={styles.note}>
                  One sender (about one CPU): <b>{formatBps(first.bps)}</b> ({kpps(first.pps)} packets/s).
                  {' '}Best: <b>{formatBps(best.bps)}</b> with {best.senders} senders, {efficiency(best, first)} of linear scaling.
                  {' '}UDP GSO was <b>{result?.settings.gso ? (steps.every((s) => s.gso) ? 'on' : 'asked for but refused by the kernel') : 'off'}</b>.
                  {' '}Packets per second, not bytes, are the limit: bigger packets give more Gbps.
                </p>
              )}
            </div>
          )}
        </div>
      </section>
    </AppLayout>
  )
}
