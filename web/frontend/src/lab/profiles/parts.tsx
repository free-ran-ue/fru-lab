import { useEffect, useRef, useState, type ReactNode } from 'react'
import type { ProfileConflict } from '../../api'
import { extractErrorMessage } from '../../apiClient'
import { Button, ui } from '../LabUi'
import NumberInput from '../NumberInput'
import styles from './profiles.module.css'

export { styles as ps }

// ConflictNote shows a field's clash with another saved profile under it,
// with the backend's next free value as a one-click fix.
export function ConflictNote({ conflict, onUse }: { conflict?: ProfileConflict; onUse?: (v: string) => void }) {
  if (!conflict) return null
  return (
    <span className={styles.conflict} role="alert">
      {conflict.message}
      {conflict.suggestion && onUse && (
        <button type="button" className={styles.useBtn} onClick={() => onUse(conflict.suggestion!)}>use {conflict.suggestion}</button>
      )}
    </span>
  )
}

// A labelled text input; `pattern` only colours the box red while the value
// does not match (the backend's checks are the real ones). `conflict` is
// the backend's check result for this field.
export function Field({ label, value, onChange, hint, disabled, pattern, wide, type = 'text', min, max, conflict }: {
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
  conflict?: ProfileConflict
}) {
  const id = `f-${label.replace(/\W+/g, '-').toLowerCase()}`
  const badValue = (pattern && !disabled && !pattern.test(String(value))) || Boolean(conflict)
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
      <ConflictNote conflict={conflict} onUse={onChange} />
    </div>
  )
}

// useProfileCheck asks the backend, ~300 ms after `draft` stops changing,
// which of its unique fields clash with another saved profile. A malformed
// draft (400) has no conflicts to show; the config preview shows why.
export function useProfileCheck<T>(draft: T, check: (d: T) => Promise<ProfileConflict[]>) {
  const [conflicts, setConflicts] = useState<ProfileConflict[]>([])
  const [checking, setChecking] = useState(false)
  const key = JSON.stringify(draft)
  const checkRef = useRef(check)
  checkRef.current = check
  useEffect(() => {
    let cancelled = false
    setChecking(true)
    const timer = window.setTimeout(() => {
      checkRef.current(JSON.parse(key) as T)
        .then((c) => { if (!cancelled) setConflicts(c) })
        .catch(() => { if (!cancelled) setConflicts([]) })
        .finally(() => { if (!cancelled) setChecking(false) })
    }, 300)
    return () => { cancelled = true; window.clearTimeout(timer) }
  }, [key])
  const byField = (field: string) => conflicts.find((c) => c.field === field)
  return { conflicts, checking, byField }
}

// BulkCreate copies the selected saved profile 1-500 times.
export function BulkCreate({ disabled, title, onCreate }: { disabled?: boolean; title?: string; onCreate: (count: number) => Promise<void> }) {
  const [count, setCount] = useState(10)
  const [busy, setBusy] = useState(false)
  const valid = Number.isInteger(count) && count >= 1 && count <= 500
  async function create() {
    setBusy(true)
    try { await onCreate(count) } finally { setBusy(false) }
  }
  return (
    <span className={styles.bulk}>
      <NumberInput
        className={ui.select}
        integer
        min={1}
        max={500}
        value={count}
        disabled={disabled}
        onChange={setCount}
        aria-label="How many copies"
        title="How many copies (1-500)"
      />
      <Button small busy={busy} disabled={disabled || !valid} title={title ?? 'Copy the selected profile; unique fields count up to the next free value'} onClick={create}>Bulk create</Button>
    </span>
  )
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
