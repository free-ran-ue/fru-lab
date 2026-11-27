import { useEffect, useRef, useState, type ReactNode } from 'react'
import { extractErrorMessage } from '../../apiClient'
import styles from './profiles.module.css'

export { styles as ps }

// A labelled text input; `pattern` only colours the box red while the value
// does not match (the backend's checks are the real ones).
export function Field({ label, value, onChange, hint, disabled, pattern, wide, type = 'text', min, max }: {
  label: string
  value: string | number
  onChange: (v: string) => void
  hint?: ReactNode
  disabled?: boolean
  pattern?: RegExp
  wide?: boolean
  type?: 'text' | 'number'
  min?: number
  max?: number
}) {
  const id = `f-${label.replace(/\W+/g, '-').toLowerCase()}`
  const badValue = pattern && !disabled && !pattern.test(String(value))
  return (
    <div className={`${styles.field} ${wide ? styles.wide : ''}`}>
      <label htmlFor={id}>{label}</label>
      <input
        id={id}
        className={`${styles.input} ${badValue ? styles.inputBad : ''}`}
        value={value}
        type={type}
        min={min}
        max={max}
        spellCheck={false}
        autoComplete="off"
        disabled={disabled}
        onChange={(e) => onChange(e.target.value)}
      />
      {hint && <span className={styles.fieldHint}>{hint}</span>}
    </div>
  )
}

export function Switch({ checked, onChange, disabled, label }: { checked: boolean; onChange: (v: boolean) => void; disabled?: boolean; label: string }) {
  return <button type="button" role="switch" aria-checked={checked} aria-label={label} className={styles.switch} disabled={disabled} onClick={() => onChange(!checked)} />
}

// useConfigPreview asks the backend to render `draft` a moment after it
// stops changing; an invalid draft comes back as the backend's message.
export function useConfigPreview<T>(draft: T, render: (d: T) => Promise<string>) {
  const [yaml, setYaml] = useState('')
  const [error, setError] = useState<string | null>(null)
  const key = JSON.stringify(draft)
  const renderRef = useRef(render)
  renderRef.current = render
  useEffect(() => {
    let cancelled = false
    const timer = window.setTimeout(() => {
      renderRef.current(JSON.parse(key) as T)
        .then((y) => { if (!cancelled) { setYaml(y); setError(null) } })
        .catch((e) => { if (!cancelled) setError(extractErrorMessage(e, 'Cannot render this config')) })
    }, 250)
    return () => { cancelled = true; window.clearTimeout(timer) }
  }, [key])
  return { yaml, error }
}

// Yaml shows a config file with keys, strings and comments coloured, and
// highlights the lines that changed since the last render.
export function Yaml({ text, error }: { text: string; error: string | null }) {
  const prev = useRef<string[]>([])
  const lines = text.split('\n')
  const changed = new Set(prev.current.length ? lines.map((l, i) => (l !== prev.current[i] ? i : -1)).filter((i) => i >= 0) : [])
  useEffect(() => { prev.current = lines })
  if (error) return <p className={styles.previewErr}>⚠ {error}</p>
  return (
    <pre className={styles.preview} aria-label="Config preview">
      {lines.map((line, i) => {
        const m = line.match(/^(\s*)([\w-]+)(:)(.*)$/)
        let body: ReactNode = line
        if (m) {
          const [, ind, k, colon, rest] = m
          const [val, cmt] = rest.split(/(?= #)/)
          body = <>{ind}<span className={styles.yKey}>{k}</span>{colon}<span className={styles.yStr}>{val}</span>{cmt && <span className={styles.yCmt}>{cmt}</span>}</>
        }
        return <div key={i} className={changed.has(i) ? styles.yChanged : undefined}>{body || ' '}</div>
      })}
    </pre>
  )
}
