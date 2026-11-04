import { useState } from 'react'
import { api, extractErrorMessage } from '../../apiClient'
import type { TesterFieldError, TesterPingResult, TesterPingRequestPlaneEnum, TesterProfile } from '../../api'
import { forTester } from './testerDefaults'
import styles from './setup.module.css'

interface PingTestProps {
  plane: TesterPingRequestPlaneEnum
  label: string
  profile: TesterProfile
  testerId: string
  // told every outcome, for Setup's pre-flight checks
  onOutcome?: (plane: TesterPingRequestPlaneEnum, outcome: PingOutcomeSummary) => void
}

// PingOutcomeSummary is a ping test's verdict in one line.
export interface PingOutcomeSummary {
  ok: boolean
  title: string
  detail: string
}

type PingState =
  | { kind: 'idle' }
  | { kind: 'busy' }
  | { kind: 'done', result: TesterPingResult }
  | { kind: 'failed', message: string, fields: TesterFieldError[] }

// fieldErrorsOf pulls the field list out of a 400 answer, if it has one.
function fieldErrorsOf(error: unknown): TesterFieldError[] {
  const data = (error as { response?: { data?: { errors?: TesterFieldError[] } } })?.response?.data
  return Array.isArray(data?.errors) ? data.errors : []
}

// summarize is a finished ping as one pre-flight line.
function summarize(r: TesterPingResult): PingOutcomeSummary {
  const ok = r.sent > 0 && r.received === r.sent
  const verdict = r.sent === 0 ? 'not tested' : ok ? 'reachable' : r.received > 0 ? 'some replies lost' : 'no reply'
  const detail = r.sent > 0
    ? `ping ${r.target} · ${r.received}/${r.sent}${r.received > 0 ? ` · ${avgMs(r.rttMs)} ms` : ''}`
    : r.error || `ping ${r.target}`
  return { ok, title: verdict, detail }
}

function avgMs(rtts: number[]): string {
  if (rtts.length === 0) return ''
  const avg = rtts.reduce((a, b) => a + b, 0) / rtts.length
  return avg < 1 ? avg.toFixed(2) : avg.toFixed(1)
}

// PingTest checks one network the way a run would use it: fru-tester puts
// the run's address on the interface, pings the core's address and removes
// it again.
export default function PingTest({ plane, label, profile, testerId, onOutcome }: PingTestProps) {
  const [state, setState] = useState<PingState>({ kind: 'idle' })

  async function handlePing() {
    setState({ kind: 'busy' })
    try {
      const response = await api.testerByIdNetworkPing(testerId, { plane, profile: forTester(profile) })
      setState({ kind: 'done', result: response.data })
      onOutcome?.(plane, summarize(response.data))
    } catch (error) {
      const message = extractErrorMessage(error, 'Ping test failed')
      setState({ kind: 'failed', message, fields: fieldErrorsOf(error) })
      onOutcome?.(plane, { ok: false, title: 'Not tested', detail: message })
    }
  }

  return (
    <div className={styles.pingTest}>
      <button type="button" className={styles.btn} onClick={handlePing} disabled={state.kind === 'busy'}>
        {state.kind === 'busy' ? 'Pinging…' : label}
      </button>
      <div className={styles.pingResult} aria-live="polite">
        {state.kind === 'idle' && (
          <span className={styles.pingNote}>Adds the address a run would use, pings, and removes it again.</span>
        )}
        {state.kind === 'busy' && <span className={styles.pingNote}>Sending 3 pings…</span>}
        {state.kind === 'failed' && (
          <>
            <span className={`${styles.pill} ${styles.pillBad}`}>Not tested</span>
            <span>{state.message}</span>
            {state.fields.map((f) => <span key={f.field} className={styles.mono}>{f.field}: {f.message}</span>)}
          </>
        )}
        {state.kind === 'done' && <PingOutcome result={state.result} />}
      </div>
    </div>
  )
}

function PingOutcome({ result }: { result: TesterPingResult }) {
  const { sent, received, rttMs, target, source } = result
  const tone = sent > 0 && received === sent ? styles.pillOk : received > 0 ? styles.pillWarn : styles.pillBad
  const verdict = sent === 0 ? 'Not tested' : received === sent ? 'Reachable' : received > 0 ? 'Some replies lost' : 'No reply'
  return (
    <>
      <span className={`${styles.pill} ${tone}`}>{verdict}</span>
      {sent > 0 && (
        <span>
          {received}/{sent} replies from <span className={styles.mono}>{target}</span>
          {received > 0 && <>, avg {avgMs(rttMs)} ms</>}
        </span>
      )}
      <span className={styles.pingNote}>
        from <span className={styles.mono}>{source}</span> on <span className={styles.mono}>{result.interface}</span>
        {result.added ? ' (added for the test and removed)' : ' (already on the host)'}
      </span>
      {result.error && <span className={styles.pingError}>{result.error}</span>}
      {sent > 0 && received === 0 && !result.error && (
        <span className={styles.pingNote}>
          Check the cable and the switch port, that {target} is in the same subnet as {source}, a VLAN on the core side, and
          that the peer answers ping.
        </span>
      )}
    </>
  )
}
