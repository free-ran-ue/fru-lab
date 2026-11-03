import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import Sidebar from '../../components/sidebar/Sidebar'
import { api, extractErrorMessage } from '../../apiClient'
import type { TesterRunSnapshot } from '../../api'
import { formatBps } from './testerFormat'
import { useTesters, type TesterOption } from './useTesters'
import styles from './tester.module.css'

// how often the list refreshes each tester's run
const POLL_MS = 3000

const ACTIVE = ['configuring', 'n2', 'running', 'stopping']

const STATE_LABELS: Record<TesterRunSnapshot['state'], string> = {
  idle: 'No run yet',
  configuring: 'Configuring',
  n2: 'N2 setup',
  running: 'Running',
  stopping: 'Cleaning up',
  stopped: 'Stopped',
  failed: 'Failed',
}

// RunState is what the list knows of one tester's run.
type RunState = { kind: 'loading' } | { kind: 'ok', run: TesterRunSnapshot } | { kind: 'down', message: string }

// RunCard is one tester's current or last run; clicking it opens the run.
function RunCard({ tester, state }: { tester: TesterOption, state: RunState }) {
  const run = state.kind === 'ok' ? state.run : null
  const active = run !== null && ACTIVE.includes(run.state)
  return (
    <Link to={`/tester/run/${tester.id}`} className={`${styles.card} ${styles.testerCard} ${styles.testerCardLink}`}>
      <div className={styles.testerCardHead}>
        <div>
          <h3>{run?.profileName || tester.name}</h3>
          {run && <span className={styles.testerCardUrl}>{run.runId}</span>}
        </div>
        {state.kind === 'down'
          ? <span className={`${styles.pill} ${styles.pillBad}`}>Unreachable</span>
          : run && <span className={`${styles.pill} ${active ? styles.pillActive : run.state === 'failed' ? styles.pillBad : styles.pillMuted}`}>{STATE_LABELS[run.state]}</span>}
      </div>
      <div className={styles.testerCardBody}>
        <dl className={styles.facts}>
          <div><dt>Tester</dt><dd>{tester.name}</dd></div>
          {run && (
            <>
              <div><dt>Started</dt><dd>{run.startedAt ? new Date(run.startedAt).toLocaleString() : '—'}</dd></div>
              <div className={styles.factWide}><dt>UEs</dt><dd>{run.ues.established} established{run.ues.failed > 0 ? ` · ${run.ues.failed} failed` : ''}</dd></div>
              {active && (
                <>
                  <div><dt>Uplink</dt><dd>{formatBps(run.dataplane.ul.rxBps)} received</dd></div>
                  <div><dt>Downlink</dt><dd>{formatBps(run.dataplane.dl.rxBps)} received</dd></div>
                </>
              )}
            </>
          )}
        </dl>
        {state.kind === 'down' && <p className={styles.runError}>{state.message}</p>}
        {run?.error && <p className={styles.runError}>{run.error}</p>}
      </div>
    </Link>
  )
}

// TesterRunsPage lists the runs going on now, one per tester (a failed
// one stays until the next run). Each card opens that run.
export default function TesterRunsPage() {
  const { testers } = useTesters()
  const [runs, setRuns] = useState<Record<string, RunState>>({})

  useEffect(() => {
    if (!testers) return
    let cancelled = false
    const read = () => {
      if (document.hidden) return
      for (const t of testers) {
        api.testerByIdRun(t.id)
          .then((r) => { if (!cancelled) setRuns((cur) => ({ ...cur, [t.id]: { kind: 'ok', run: r.data } })) })
          .catch((error) => { if (!cancelled) setRuns((cur) => ({ ...cur, [t.id]: { kind: 'down', message: extractErrorMessage(error, 'fru-tester did not answer') } })) })
      }
    }
    read()
    const timer = window.setInterval(read, POLL_MS)
    return () => { cancelled = true; window.clearInterval(timer) }
  }, [testers])

  // a tester with no run or a stopped one has no card; one that cannot be
  // asked does. Stopped runs are in History.
  const shown = (testers ?? []).filter((t) => {
    const s = runs[t.id]
    return s?.kind === 'down' || (s?.kind === 'ok' && s.run.state !== 'idle' && s.run.state !== 'stopped')
  })
  const loading = testers === null || testers.some((t) => !runs[t.id])

  return (
    <div className={styles.layout}>
      <Sidebar />
      <main className={styles.content}>
        <header className={styles.header}>
          <div>
            <h2 className={styles.title}>Runs</h2>
            <p className={styles.subtitle}>
              The runs going on now, one per tester. Open a run to follow it live and stop it; stopped runs are in History. Start a run from the <Link to="/tester" className={styles.linkButton}>Setup</Link> page.
            </p>
          </div>
        </header>
        {loading && shown.length === 0 && <p className={styles.emptyState}>Loading runs…</p>}
        {!loading && shown.length === 0 && (
          <p className={styles.emptyState}>No run in progress. <Link to="/tester">Start one</Link>, or see past runs in <Link to="/tester/history">History</Link>.</p>
        )}
        <div className={styles.testerGrid}>
          {shown.map((t) => <RunCard key={t.id} tester={t} state={runs[t.id]} />)}
        </div>
      </main>
    </div>
  )
}
