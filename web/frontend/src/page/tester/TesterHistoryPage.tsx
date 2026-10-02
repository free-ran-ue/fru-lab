import { useEffect, useState } from 'react'
import Sidebar from '../../components/sidebar/Sidebar'
import NotificationContainer from '../../components/notifications/NotificationContainer'
import { useNotifications } from '../../hooks/useNotifications'
import { api, extractErrorMessage } from '../../apiClient'
import type { TesterHistorySummary } from '../../api'
import { formatBps, formatBits, formatLoss, formatMs } from './testerFormat'
import styles from './tester.module.css'
import Pager from '../../components/pager/Pager'
import { pageOf } from '../../components/pager/paging'

// HISTORY_PAGE is how many runs one page of the history shows.
const HISTORY_PAGE = 20

function seconds(r: TesterHistorySummary): number {
  if (!r.startedAt || !r.stoppedAt) return 0
  return (new Date(r.stoppedAt).getTime() - new Date(r.startedAt).getTime()) / 1000
}

function percent(n: number, of: number): string {
  return of ? `${((n / of) * 100).toFixed(1)} %` : '—'
}

function duration(r: TesterHistorySummary): string {
  const s = Math.round(seconds(r))
  return `${Math.floor(s / 3600)}:${String(Math.floor(s / 60) % 60).padStart(2, '0')}:${String(s % 60).padStart(2, '0')}`
}

// avgRate is the average received rate over the whole run.
function avgRate(bytes: number, r: TesterHistorySummary): string {
  const s = seconds(r)
  return s > 0 ? formatBps((bytes * 8) / s) : '—'
}

function result(r: TesterHistorySummary): string {
  if (r.state === 'failed') return 'Failed'
  return r.stopReason === 'maxDuration' ? 'Max run time' : 'Stopped'
}

// METRICS are the numbers shown per run, in the table and side by side.
const METRICS: [string, (r: TesterHistorySummary) => string][] = [
  ['Duration', duration],
  ['gNBs / UEs', (r) => `${r.gnbCount} / ${r.ueCount}`],
  ['Registration', (r) => percent(r.registered, r.ueCount)],
  ['Registration p95', (r) => formatMs(r.registrationP95Ms)],
  ['PDU session', (r) => percent(r.established, r.ueCount)],
  ['Deregistration', (r) => percent(r.deregistered, r.registered)],
  ['DL received', (r) => `${formatBits(r.dlRxBytes)} · ${avgRate(r.dlRxBytes, r)}`],
  ['UL received', (r) => `${formatBits(r.ulRxBytes)} · ${avgRate(r.ulRxBytes, r)}`],
  ['Loss DL / UL', (r) => `${formatLoss(r.dlLossRate)} / ${formatLoss(r.ulLossRate)}`],
]

// save downloads blob as a file named name.
function save(blob: Blob, name: string) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = name
  a.click()
  URL.revokeObjectURL(url)
}

export default function TesterHistoryPage() {
  const { errors, successes, addError, removeNotification } = useNotifications()
  const [runs, setRuns] = useState<TesterHistorySummary[] | null>(null)
  const [selected, setSelected] = useState<string[]>([])
  const [page, setPage] = useState(1)

  useEffect(() => {
    api.testerHistoryList()
      .then((response) => setRuns(response.data))
      .catch((error) => addError(extractErrorMessage(error, 'Failed to load the run history')))
  }, [addError])

  // toggle keeps at most two runs selected: picking a third drops the oldest pick.
  const toggle = (id: string) => setSelected((cur) => (cur.includes(id) ? cur.filter((x) => x !== id) : [...cur, id].slice(-2)))

  const download = async (r: TesterHistorySummary, kind: 'json' | 'csv') => {
    try {
      const response = kind === 'json'
        ? await api.testerHistoryGet(r.runId, { responseType: 'blob' })
        : await api.testerHistorySeriesCsv(r.runId, { responseType: 'blob' })
      save(response.data as unknown as Blob, `tester-${r.runId}.${kind}`)
    } catch (error) {
      addError(extractErrorMessage(error, `Failed to export run ${r.runId}`))
    }
  }

  const compared = (runs ?? []).filter((r) => selected.includes(r.runId))
  const shown = pageOf(runs ?? [], page, HISTORY_PAGE)

  return (
    <div className={styles.layout}>
      <NotificationContainer errors={errors} successes={successes} onClose={removeNotification} />
      <Sidebar />

      <main className={styles.content}>
        <header className={styles.header}>
          <div>
            <h2 className={styles.title}>Throughput Tester · History</h2>
            <p className={styles.subtitle}>The last 50 finished runs. Tick two to compare them side by side.</p>
          </div>
        </header>

        {compared.length === 2 && (
          <section className={styles.card}>
            <h3 className={styles.cardTitle}>Comparison</h3>
            <div className={styles.tableWrap}>
              <table className={styles.table}>
                <thead>
                  <tr>
                    <th />
                    {compared.map((r) => (
                      <th key={r.runId}>{r.startedAt ? new Date(r.startedAt).toLocaleString() : r.runId} · {r.profileName}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {METRICS.map(([label, f]) => (
                    <tr key={label}>
                      <td>{label}</td>
                      {compared.map((r) => <td key={r.runId} className={styles.mono}>{f(r)}</td>)}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </section>
        )}

        <section className={styles.card}>
          <h3 className={styles.cardTitle}>Runs</h3>
          {runs === null && <p className={styles.hint}>Loading…</p>}
          {runs?.length === 0 && <p className={styles.hint}>No finished runs yet. Runs are saved here when they stop.</p>}
          {runs && runs.length > 0 && (
            <div className={styles.tableWrap}>
              <table className={styles.table}>
                <thead>
                  <tr>
                    <th />
                    <th>Started</th>
                    <th>Profile</th>
                    <th>Result</th>
                    <th>Export</th>
                    {METRICS.map(([label]) => <th key={label}>{label}</th>)}
                  </tr>
                </thead>
                <tbody>
                  {shown.items.map((r) => (
                    <tr key={r.runId}>
                      <td>
                        <input type="checkbox" aria-label={`Compare run ${r.runId}`}
                          checked={selected.includes(r.runId)} onChange={() => toggle(r.runId)} />
                      </td>
                      <td>{r.startedAt ? new Date(r.startedAt).toLocaleString() : '—'}</td>
                      <td>{r.profileName}</td>
                      <td>
                        <span className={`${styles.pill} ${r.state === 'failed' ? styles.pillBad : styles.pillMuted}`} title={r.error || undefined}>
                          {result(r)}
                        </span>
                      </td>
                      <td>
                        <button type="button" className={styles.linkButton} onClick={() => download(r, 'json')}>JSON</button>
                        <button type="button" className={styles.linkButton} onClick={() => download(r, 'csv')}>CSV</button>
                      </td>
                      {METRICS.map(([label, f]) => <td key={label} className={styles.mono}>{f(r)}</td>)}
                    </tr>
                  ))}
                </tbody>
              </table>
              <Pager page={shown} total={runs.length} onChange={setPage} />
            </div>
          )}
        </section>
      </main>
    </div>
  )
}
