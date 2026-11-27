import { useEffect, useMemo, useRef, useState } from 'react'
import { openLogStream, sourceKey, type LogLine, type LogSource } from '../console/stream'
import styles from './console.module.css'

const MAX_LINES = 4000

export interface SourceOption {
  label: string
  source: LogSource
}

function levelClass(line: string): string {
  if (/\[(EROR|ERRO|ERROR|FATA|FATAL)\]/.test(line)) return styles.lvErr
  if (/\[(WARN|WARNING)\]/.test(line)) return styles.lvWarn
  if (/\[(DEBU|DEBUG|TRAC|TRACE)\]/.test(line)) return styles.lvDebug
  return styles.lvInfo
}

const clock = (t: string) => {
  const d = new Date(t)
  return Number.isNaN(d.getTime()) ? '' : `${d.toISOString().slice(11, 23)}`
}

// LogsTab follows one source live: the last 500 lines, then every new one.
// The filter narrows and highlights; Follow keeps the newest line in view.
export default function LogsTab({ options, source, onSource }: { options: SourceOption[]; source: LogSource | null; onSource: (s: LogSource) => void }) {
  const [lines, setLines] = useState<LogLine[]>([])
  const [state, setState] = useState<'open' | 'closed' | 'error' | 'idle'>('idle')
  const [problem, setProblem] = useState('')
  const [filter, setFilter] = useState('')
  const [follow, setFollow] = useState(true)
  const [paused, setPaused] = useState(false)
  const buffer = useRef<LogLine[]>([])
  const pausedRef = useRef(paused)
  pausedRef.current = paused
  const box = useRef<HTMLDivElement>(null)
  const key = source ? sourceKey(source) : ''

  useEffect(() => {
    setLines([])
    buffer.current = []
    if (!source) { setState('idle'); return }
    const close = openLogStream(source, 500, (l) => buffer.current.push(l), (s, detail) => { setState(s); setProblem(detail ?? '') })
    const timer = window.setInterval(() => {
      if (pausedRef.current || buffer.current.length === 0) return
      const batch = buffer.current
      buffer.current = []
      setLines((cur) => [...cur, ...batch].slice(-MAX_LINES))
    }, 200)
    return () => { close(); window.clearInterval(timer) }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key])

  const shown = useMemo(() => {
    if (!filter) return lines
    const f = filter.toLowerCase()
    return lines.filter((l) => l.line.toLowerCase().includes(f) || l.src.toLowerCase().includes(f))
  }, [lines, filter])

  useEffect(() => {
    if (follow && box.current) box.current.scrollTop = box.current.scrollHeight
  }, [shown, follow])

  const highlight = (text: string) => {
    if (!filter) return text
    const i = text.toLowerCase().indexOf(filter.toLowerCase())
    if (i < 0) return text
    return <>{text.slice(0, i)}<mark className={styles.mark}>{text.slice(i, i + filter.length)}</mark>{text.slice(i + filter.length)}</>
  }

  return (
    <>
      <div className={styles.tools}>
        <select className={styles.ctl} value={key} onChange={(e) => { const o = options.find((x) => sourceKey(x.source) === e.target.value); if (o) onSource(o.source) }} aria-label="Log source">
          {!source && <option value="">pick a source</option>}
          {options.map((o) => <option key={sourceKey(o.source)} value={sourceKey(o.source)}>{o.label}</option>)}
        </select>
        <input className={styles.ctl} placeholder="filter" value={filter} onChange={(e) => setFilter(e.target.value)} aria-label="Filter lines" />
        <label className={styles.check}><input type="checkbox" checked={follow} onChange={(e) => setFollow(e.target.checked)} />follow</label>
        <button type="button" className={styles.ctlBtn} onClick={() => setPaused((p) => !p)}>{paused ? 'resume' : 'pause'}</button>
        <button type="button" className={styles.ctlBtn} onClick={() => setLines([])}>clear</button>
        <span className={`${styles.status} ${state === 'open' ? styles.ok : state === 'error' ? styles.bad : ''}`}>
          {state === 'open' ? `● live · ${shown.length} lines` : state === 'error' ? problem || 'error' : state === 'closed' ? 'reconnecting…' : ''}
        </span>
      </div>
      <div className={styles.lines} ref={box} onWheel={() => { if (box.current && box.current.scrollTop + box.current.clientHeight < box.current.scrollHeight - 20) setFollow(false) }}>
        {!source && <p className={styles.empty}>Select a node on the topology, or pick a source.</p>}
        {source && shown.length === 0 && <p className={styles.empty}>{state === 'open' ? 'No output yet.' : 'Connecting…'}</p>}
        {shown.map((l, i) => (
          <div key={i} className={styles.line}>
            <span className={styles.ts}>{clock(l.t)}</span>
            <span className={styles.src} title={l.src}>{l.src}</span>
            <span className={levelClass(l.line)}>{highlight(l.line)}</span>
          </div>
        ))}
      </div>
    </>
  )
}
