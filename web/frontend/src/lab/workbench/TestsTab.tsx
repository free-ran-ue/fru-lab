import { useEffect, useRef, useState } from 'react'
import { wsUrl } from '../console/stream'
import styles from './console.module.css'

type Action = 'ping' | 'curl' | 'iperf3'

interface Run {
  id: number
  ue: string
  action: Action
  target: string
  output: string
  done: boolean
  summary?: string
  pass?: boolean
}

const DEFAULT_TARGET: Record<Action, string> = { ping: '8.8.8.8', curl: 'http://example.com', iperf3: '' }

// summarize reads the verdict out of a finished command's output.
function summarize(action: Action, out: string): { summary: string; pass: boolean } {
  if (action === 'ping') {
    const loss = out.match(/(\d+(?:\.\d+)?)% packet loss/)
    const rtt = out.match(/= [\d.]+\/([\d.]+)\//)
    if (loss) return { summary: `${loss[1]}% loss${rtt ? ` · avg ${rtt[1]} ms` : ''}`, pass: Number(loss[1]) < 100 }
  }
  if (action === 'curl') {
    const m = out.match(/HTTP (\d{3}).*total ([\d.]+)s/)
    if (m) return { summary: `HTTP ${m[1]} · ${(Number(m[2]) * 1000).toFixed(0)} ms`, pass: m[1] !== '000' }
  }
  if (action === 'iperf3') {
    const m = out.match(/([\d.]+ [KMG]bits\/sec)\s+\S*\s*receiver/)
    if (m) return { summary: m[1], pass: true }
  }
  return { summary: /error|unreachable|failed|could not|refused/i.test(out) ? 'failed' : 'finished', pass: false }
}

// TestsTab runs ping, curl or iperf3 from the selected UE over its 5G
// tunnel and streams the output; past runs stay in the list on the right.
export default function TestsTab({ instance, tun, ready }: { instance: string | null; tun?: string; ready: boolean }) {
  const [action, setAction] = useState<Action>('ping')
  const [target, setTarget] = useState(DEFAULT_TARGET.ping)
  const [count, setCount] = useState(4)
  const [runs, setRuns] = useState<Run[]>([])
  const [viewing, setViewing] = useState<number | null>(null)
  const socket = useRef<WebSocket | null>(null)
  const out = useRef<HTMLPreElement>(null)
  const running = runs.some((r) => !r.done)
  const shown = runs.find((r) => r.id === viewing) ?? runs[0]

  useEffect(() => () => socket.current?.close(), [])
  useEffect(() => { if (out.current) out.current.scrollTop = out.current.scrollHeight }, [shown?.output])

  function run() {
    if (!instance) return
    const id = Date.now()
    const decoder = new TextDecoder()
    setRuns((r) => [{ id, ue: instance, action, target, output: '', done: false }, ...r].slice(0, 30))
    setViewing(id)
    const ws = new WebSocket(wsUrl('/stream/ue-test', {
      instance, action, target, dev: tun,
      count: action === 'ping' ? count : undefined,
      seconds: action === 'iperf3' ? count : undefined,
    }))
    ws.binaryType = 'arraybuffer'
    socket.current = ws
    const append = (text: string) => setRuns((rs) => rs.map((r) => (r.id === id ? { ...r, output: r.output + text.replace(/\r/g, '') } : r)))
    ws.onmessage = (e) => append(e.data instanceof ArrayBuffer ? decoder.decode(e.data) : String(e.data))
    ws.onclose = () => setRuns((rs) => rs.map((r) => (r.id === id ? { ...r, done: true, ...summarize(r.action, r.output) } : r)))
  }

  if (!instance) return <p className={styles.empty}>Select a running UE to test its data path: ping, curl or iperf3 over the 5G tunnel.</p>

  return (
    <>
      <div className={styles.tools}>
        <select className={styles.ctl} value={action} onChange={(e) => { const a = e.target.value as Action; setAction(a); setTarget(DEFAULT_TARGET[a]); setCount(a === 'iperf3' ? 5 : 4) }} aria-label="Test">
          <option value="ping">ping</option>
          <option value="curl">curl</option>
          <option value="iperf3">iperf3</option>
        </select>
        <input className={styles.ctl} style={{ width: '16rem' }} value={target} onChange={(e) => setTarget(e.target.value)} placeholder={action === 'iperf3' ? 'iperf3 server host' : action === 'curl' ? 'http://…' : 'host or IP'} aria-label="Target" />
        {action !== 'curl' && (
          <label className={styles.check}>{action === 'ping' ? 'count' : 'seconds'}
            <input className={styles.ctl} style={{ width: '3.5rem' }} type="number" min={1} max={action === 'ping' ? 50 : 60} value={count} onChange={(e) => setCount(Number(e.target.value) || 1)} />
          </label>
        )}
        <button type="button" className={styles.ctlPrimary} disabled={running || !target || !ready} onClick={run} title={ready ? undefined : 'The UE has no PDU session yet'}>
          {running ? 'running…' : 'run'}
        </button>
        {running && <button type="button" className={styles.ctlBtn} onClick={() => socket.current?.close()}>stop</button>}
        <span className={styles.status}>from {instance} via {tun ?? 'ueTun0'}{ready ? '' : ' · no PDU session yet'}</span>
      </div>
      <div className={styles.split}>
        <pre className={styles.out} ref={out}>{shown ? shown.output || 'starting…' : 'Run a test to see its output here.'}</pre>
        <div className={styles.history} aria-label="Test history">
          {runs.map((r) => (
            <button key={r.id} type="button" className={styles.hist} onClick={() => setViewing(r.id)} aria-pressed={r.id === shown?.id}>
              <b>{r.action} {r.target}</b>
              <span>{r.ue.replace(/^imsi-/, '')} · </span>
              {r.done ? <span className={r.pass ? styles.pass : styles.fail}>{r.summary}</span> : <span>running…</span>}
            </button>
          ))}
        </div>
      </div>
    </>
  )
}
