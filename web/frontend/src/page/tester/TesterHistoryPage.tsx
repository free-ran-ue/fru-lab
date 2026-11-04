import { useEffect, useMemo, useRef, useState } from 'react'
import AppLayout from '../../components/layout/AppLayout'
import NotificationContainer from '../../components/notifications/NotificationContainer'
import { useNotifications } from '../../hooks/useNotifications'
import { api, extractErrorMessage } from '../../apiClient'
import type { TesterHistorySummary } from '../../api'
import { formatBps, formatBits, formatLoss, formatMs } from './testerFormat'
import { useTesters } from './useTesters'
import styles from './history.module.css'
import { pageOf } from '../../components/pager/paging'

// HISTORY_PAGE is how many runs one page of the history shows.
const HISTORY_PAGE = 20

// TESTER_COLORS tell the testers apart in the table: accent, then violet.
const TESTER_COLORS = ['var(--accent)', 'var(--violet)', 'var(--ok)', 'var(--warn)', 'var(--accent-ink)', 'var(--ink-2)']

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

type Result = 'max' | 'failed' | 'stopped'

function resultOf(r: TesterHistorySummary): Result {
  if (r.state === 'failed') return 'failed'
  return r.stopReason === 'maxDuration' ? 'max' : 'stopped'
}

const RESULTS: Record<Result, [string, string]> = {
  max: ['Max run time', ''],
  failed: ['Failed', styles.tBad],
  stopped: ['Stopped', ''],
}

// COMPARE_COLORS mark the two compared runs: accent, then violet.
const COMPARE_COLORS = ['var(--accent)', 'var(--violet)']

// started is the start time of the table, e.g. 2026-11-03 05:54:10.
function started(r: TesterHistorySummary): string {
  if (!r.startedAt) return '—'
  const d = new Date(r.startedAt)
  const p = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
}

// METRICS are the numbers compared side by side.
const METRICS: [string, (r: TesterHistorySummary) => string][] = [
  ['Duration', duration],
  ['gNBs / UEs', (r) => `${r.gnbCount} / ${r.ueCount}`],
  ['Registration', (r) => percent(r.registered, r.ueCount)],
  ['Registration p95', (r) => formatMs(r.registrationP95Ms)],
  ['PDU session', (r) => percent(r.established, r.ueCount)],
  ['Deregistration', (r) => percent(r.deregistered, r.registered)],
  ['UL received', (r) => `${formatBits(r.ulRxBytes)} · ${avgRate(r.ulRxBytes, r)}`],
  ['DL received', (r) => `${formatBits(r.dlRxBytes)} · ${avgRate(r.dlRxBytes, r)}`],
  ['Loss UL / DL', (r) => `${formatLoss(r.ulLossRate)} / ${formatLoss(r.dlLossRate)}`],
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
  const [comparing, setComparing] = useState(false)
  const [query, setQuery] = useState('')
  const [testerFilter, setTesterFilter] = useState('')
  const [resultFilter, setResultFilter] = useState<'' | Result>('')
  const { testers } = useTesters()
  const compareRef = useRef<HTMLElement>(null)
  // the tester's name when the run was stored, else its name now
  const testerName = (r: TesterHistorySummary) => r.testerName || testers?.find((t) => t.id === r.testerId)?.name || r.testerId
  const [page, setPage] = useState(1)

  useEffect(() => {
    api.testerHistoryList()
      .then((response) => setRuns(response.data))
      .catch((error) => addError(extractErrorMessage(error, 'Failed to load the run history')))
  }, [addError])

  // toggle keeps at most two runs selected: picking a third drops the oldest pick.
  const toggle = (id: string) => setSelected((cur) => (cur.includes(id) ? cur.filter((x) => x !== id) : [...cur, id].slice(-2)))

  // report fetches a run's HTML report (the API needs the JWT header, so
  // a plain link cannot) and opens it in a new tab or saves it.
  const report = async (r: TesterHistorySummary, how: 'view' | 'download') => {
    // opened before the request, while the click still counts as the
    // user's, so pop-up blockers let it through
    const tab = how === 'view' ? window.open('', '_blank') : null
    try {
      const response = await api.testerHistoryReportHtml(r.id, { responseType: 'blob' })
      const blob = new Blob([response.data as unknown as Blob], { type: 'text/html' })
      if (tab) {
        const url = URL.createObjectURL(blob)
        tab.location.href = url
        setTimeout(() => URL.revokeObjectURL(url), 60_000)
      } else {
        save(blob, `tester-${r.runId}.html`)
      }
    } catch (error) {
      tab?.close()
      addError(extractErrorMessage(error, `Failed to load the report of run ${r.runId}`))
    }
  }

  // every tester in the history gets one colour, in order of first use
  const colorOf = useMemo(() => {
    const ids: string[] = []
    for (const r of runs ?? []) if (!ids.includes(r.testerId)) ids.push(r.testerId)
    return (id: string) => TESTER_COLORS[ids.indexOf(id) % TESTER_COLORS.length]
  }, [runs])

  const testerIds = [...new Set((runs ?? []).map((r) => r.testerId))]
  const q = query.trim().toLowerCase()
  const filtered = (runs ?? []).filter((r) =>
    (q === '' || r.runId.toLowerCase().includes(q) || r.profileName.toLowerCase().includes(q))
    && (testerFilter === '' || r.testerId === testerFilter)
    && (resultFilter === '' || resultOf(r) === resultFilter))

  useEffect(() => { setPage(1) }, [query, testerFilter, resultFilter])

  const compared = (runs ?? []).filter((r) => selected.includes(r.id))
  const showCompare = comparing && compared.length === 2
  const shown = pageOf(filtered, page, HISTORY_PAGE)

  useEffect(() => { if (selected.length !== 2) setComparing(false) }, [selected])
  useEffect(() => { if (showCompare) compareRef.current?.scrollIntoView({ behavior: 'smooth', block: 'start' }) }, [showCompare])

  return (
    <AppLayout crumbs={[{ label: 'History' }]}
      overlay={<NotificationContainer errors={errors} successes={successes} onClose={removeNotification} />}>
      <section className={styles.head}>
        <h1 className={styles.title}>History</h1>
        <span className={styles.sub}>The last 50 finished runs from every tester. Tick two to compare them; View opens a run&apos;s report, Download saves it as one offline HTML file.</span>
        <div className={styles.headActions}>
          <button type="button" className={`${styles.btn} ${styles.btnP}`} onClick={() => setComparing((c) => !c)} disabled={selected.length !== 2}>
            {showCompare ? 'Hide comparison' : 'Compare 2 runs'}
          </button>
        </div>
      </section>

      <section className={styles.panel}>
        <div className={styles.ph}>
          <h2>Runs</h2>
          <span className={`${styles.aux} ${styles.mono}`}>{runs === null ? 'loading…' : `${runs.length} runs · ${selected.length} selected`}</span>
        </div>
        <div className={styles.toolbar}>
          <input className={`${styles.inp} ${styles.search}`} aria-label="Search runs" placeholder="Search run ID or profile"
            value={query} onChange={(e) => setQuery(e.target.value)} />
          <select className={styles.inp} aria-label="Tester" value={testerFilter} onChange={(e) => setTesterFilter(e.target.value)}>
            <option value="">All testers</option>
            {testerIds.map((id) => {
              const r = runs!.find((x) => x.testerId === id)!
              return <option key={id} value={id}>{testerName(r)}</option>
            })}
          </select>
          <select className={styles.inp} aria-label="Result" value={resultFilter} onChange={(e) => setResultFilter(e.target.value as '' | Result)}>
            <option value="">All results</option>
            <option value="max">Max run time</option>
            <option value="failed">Failed</option>
            <option value="stopped">Stopped</option>
          </select>
          <span className={`${styles.mono} ${styles.dim}`}>{filtered.length} of {runs?.length ?? 0} shown</span>
        </div>
        {runs === null && <p className={styles.empty}>Loading…</p>}
        {runs?.length === 0 && <p className={styles.empty}>No finished runs yet. Runs are saved here when they stop.</p>}
        {runs && runs.length > 0 && filtered.length === 0 && <p className={styles.empty}>No run matches these filters.</p>}
        {filtered.length > 0 && (
          <div className={styles.scroll}>
            <table className={`${styles.tbl} ${styles.runsTbl}`}>
              <thead>
                <tr>
                  <th className={styles.c}><span className={styles.srOnly}>Compare</span></th>
                  <th>Started</th><th>Run</th><th>Profile</th><th>Tester</th><th>Result</th><th>Report</th>
                  {METRICS.map(([label]) => <th key={label} className={styles.r}>{label}</th>)}
                </tr>
              </thead>
              <tbody>
                {shown.items.map((r) => {
                  const [label, tone] = RESULTS[resultOf(r)]
                  const sel = selected.includes(r.id)
                  return (
                    <tr key={r.id} className={sel ? styles.sel : undefined}>
                      <td className={styles.c}>
                        <input type="checkbox" className={styles.ck} aria-label={`Compare run ${r.runId}`}
                          checked={sel} onChange={() => toggle(r.id)} />
                      </td>
                      <td className={`${styles.mono} ${styles.dim}`}>{started(r)}</td>
                      <td className={styles.mono}>{r.runId}</td>
                      <td>{r.profileName}</td>
                      <td><span className={styles.swatch}><i style={{ background: colorOf(r.testerId) }} />{testerName(r)}</span></td>
                      <td><span className={`${styles.tag} ${tone}`} title={r.error || undefined}><i />{label}</span></td>
                      <td>
                        <span className={styles.lnks}>
                          <button type="button" className={styles.lnk} onClick={() => report(r, 'view')}>View</button>
                          <button type="button" className={styles.lnk} onClick={() => report(r, 'download')}>Download</button>
                        </span>
                      </td>
                      {METRICS.map(([m, f]) => <td key={m} className={styles.r}>{f(r)}</td>)}
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
        {filtered.length > 0 && (
          <div className={styles.pager}>
            <span className={styles.mono}>{shown.from}–{shown.to} of {filtered.length}</span>
            {shown.pages > 1 && (
              <span className={styles.pagerBtns}>
                <span className={styles.mono}>Page {shown.page} / {shown.pages}</span>
                <button type="button" className={styles.btn} onClick={() => setPage(shown.page - 1)} disabled={shown.page === 1}>Previous</button>
                <button type="button" className={styles.btn} onClick={() => setPage(shown.page + 1)} disabled={shown.page === shown.pages}>Next</button>
              </span>
            )}
          </div>
        )}
      </section>

      {showCompare && (
        <section ref={compareRef} className={styles.panel}>
          <div className={styles.ph}><h2>Comparison</h2><span className={styles.aux}>2 of 2 runs ticked</span></div>
          <div className={styles.scroll}>
            <table className={`${styles.tbl} ${styles.cmpTbl}`}>
              <thead>
                <tr>
                  <th className={styles.metricCol}>Metric</th>
                  {compared.map((r, i) => (
                    <th key={r.id} className={styles.r}>
                      <span className={styles.legend}><i style={{ background: COMPARE_COLORS[i] }} /><span className={styles.mono}>{started(r)}</span></span>
                      <br />
                      <span className={styles.colSub}><span className={styles.mono}>{r.runId}</span> · {r.profileName} · {testerName(r)}</span>
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {METRICS.map(([label, f]) => (
                  <tr key={label}>
                    <td>{label}</td>
                    {compared.map((r) => <td key={r.id} className={styles.r}>{f(r)}</td>)}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      )}
    </AppLayout>
  )
}
