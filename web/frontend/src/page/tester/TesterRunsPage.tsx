import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import AppLayout from '../../components/layout/AppLayout'
import { api, extractErrorMessage } from '../../apiClient'
import type { TesterRunSnapshot, TesterStageSnapshot } from '../../api'
import { formatBps, formatClock } from './testerFormat'
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

function StageMeter({ label, s }: { label: string, s: TesterStageSnapshot }) {
  const ratio = s.expected > 0 ? s.accepted / s.expected : 0
  return (
    <div className={styles.meterRow}>
      <div className={styles.meterTop}><span>{label}</span><span className="num">{s.accepted.toLocaleString()} / {s.expected.toLocaleString()}</span></div>
      <div className={styles.track}><i style={{ width: `${Math.min(100, ratio * 100)}%` }} /></div>
    </div>
  )
}

const OpenArrow = () => (
  <svg viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="1.6" aria-hidden="true"><path d="M4 2l4 4-4 4" /></svg>
)

// RunCard is one tester's current or last run; clicking it opens the run.
function RunCard({ tester, state }: { tester: TesterOption, state: RunState }) {
  const run = state.kind === 'ok' ? state.run : null
  const active = run !== null && ACTIVE.includes(run.state)
  const to = `/tester/run/${tester.id}`
  const elapsed = run?.startedAt
    ? ((run.stoppedAt ? new Date(run.stoppedAt).getTime() : Date.now()) - new Date(run.startedAt).getTime()) / 1000
    : null

  let pill = null
  if (state.kind === 'down') pill = <span className={`${styles.state} ${styles.stateBad}`}>Unreachable</span>
  else if (run) {
    const tone = run.state === 'failed' ? styles.stateBad : run.state === 'stopping' ? styles.stateWarn : active ? styles.stateOk : ''
    pill = <span className={`${styles.state} ${tone}`}>{STATE_LABELS[run.state]}</span>
  }

  return (
    <div className={styles.tcard}>
      <Link to={to} className={styles.tcardLink}>
        <div className={styles.tcardHead}>
          <div className={styles.tcardName}>
            <span className={styles.tcardTitle}>{run?.profileName || tester.name}</span>
            {run && <span className={`num ${styles.tcardUrl}`}>{run.runId}</span>}
          </div>
          {pill}
        </div>
        {state.kind === 'down' && (
          <div className={styles.tcardDown}>
            <span>No answer</span>
            <span>{state.message}</span>
          </div>
        )}
        {run && (
          <div className={styles.tcardBody}>
            <StageMeter label="Registered" s={run.registration} />
            <StageMeter label="PDU sessions" s={run.pdu} />
            <dl className={styles.facts}>
              <div><dt>Started</dt><dd>{run.startedAt ? new Date(run.startedAt).toLocaleString() : '—'}</dd></div>
              <div><dt>Elapsed</dt><dd>{elapsed === null ? '—' : formatClock(elapsed)}</dd></div>
              <div><dt>Downlink</dt><dd>{active ? formatBps(run.dataplane.dl.rxBps) : '—'}</dd></div>
              <div><dt>Uplink</dt><dd>{active ? formatBps(run.dataplane.ul.rxBps) : '—'}</dd></div>
            </dl>
            {run.error && <p className={styles.runError}>{run.error}</p>}
          </div>
        )}
      </Link>
      <div className={styles.tcardFoot}>
        <span>Tester {tester.name}{run && run.ues.failed > 0 ? ` · ${run.ues.failed.toLocaleString()} UEs failed` : ''}</span>
        <Link to={to} className={styles.openLink} tabIndex={-1}>Open<OpenArrow /></Link>
      </div>
    </div>
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
    <AppLayout crumbs={[{ label: 'Runs' }]}>
      <section className={styles.pageHead}>
        <div className={styles.pageHeadText}>
          <h1 className={styles.pageTitle}>Runs</h1>
          <p className={styles.pageSub}>
            The runs going on now, one per tester. Open a run to follow it live and stop it; stopped runs are in <Link to="/tester/history">History</Link>. Start a run from <Link to="/tester">Setup</Link>.
          </p>
        </div>
      </section>
      {loading && shown.length === 0 && <p className={styles.emptyState}>Loading runs…</p>}
      {!loading && shown.length === 0 && (
        <p className={styles.emptyState}>No run in progress. <Link to="/tester">Start one</Link>, or see past runs in <Link to="/tester/history">History</Link>.</p>
      )}
      <section aria-label="Runs" className={styles.cardGrid}>
        {shown.map((t) => <RunCard key={t.id} tester={t} state={runs[t.id]} />)}
      </section>
    </AppLayout>
  )
}
