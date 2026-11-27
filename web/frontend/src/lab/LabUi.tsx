import type { ReactNode } from 'react'
import { StatusTag } from './StatusTag'
import type { NodeStatus } from './types'
import { NodeIcon, type IconName } from './canvas/icons'
import styles from './ui.module.css'

export { styles as ui }

export function Dot({ status }: { status: NodeStatus }) {
  return <span aria-hidden="true" className={`${styles.dot} ${styles[`dot_${status}`]}`} />
}

interface ButtonProps {
  kind?: 'primary' | 'danger' | 'plain'
  small?: boolean
  busy?: boolean
  disabled?: boolean
  title?: string
  onClick: () => void
  children: ReactNode
}

export function Button({ kind = 'plain', small, busy, disabled, title, onClick, children }: ButtonProps) {
  const cls = [styles.btn, kind === 'primary' ? styles.btnPrimary : kind === 'danger' ? styles.btnDanger : '', small ? styles.btnSm : ''].join(' ')
  return (
    <button type="button" className={cls} onClick={onClick} disabled={disabled || busy} title={title}>
      {busy && <span className={styles.spinner} aria-hidden="true" />}
      {children}
    </button>
  )
}

export function Segmented<T extends string>({ value, options, onChange, disabled }: { value: T; options: { value: T; label: string }[]; onChange: (v: T) => void; disabled?: boolean }) {
  return (
    <div className={styles.seg} role="group">
      {options.map((o) => (
        <button key={o.value} type="button" aria-pressed={o.value === value} onClick={() => onChange(o.value)} disabled={disabled}>{o.label}</button>
      ))}
    </div>
  )
}

export function Section({ title, aside, children }: { title?: ReactNode; aside?: ReactNode; children: ReactNode }) {
  return (
    <section className={styles.section}>
      {title && <h3 className={styles.sectionTitle}><span>{title}</span>{aside}</h3>}
      {children}
    </section>
  )
}

// The inspector's header: icon, name, sub line and status.
export function InspectorHead({ icon, name, sub, status, statusText }: { icon: IconName; name: string; sub?: string; status?: NodeStatus; statusText?: string }) {
  return (
    <Section>
      <div className={styles.head}>
        <span className={styles.headIcon}><svg width="18" height="18" viewBox="0 0 24 24" aria-hidden="true"><NodeIcon name={icon} x={0} y={0} size={24} /></svg></span>
        <div>
          <h2 className={styles.headName}>{name}</h2>
          {sub && <div className={styles.headSub}>{sub}</div>}
        </div>
        {status && <span className={styles.headStatus}><StatusTag status={status} text={statusText} /></span>}
      </div>
    </Section>
  )
}

export function KV({ rows }: { rows: { k: string; v: ReactNode }[] }) {
  return (
    <dl className={styles.kv}>
      {rows.map((r) => <div key={r.k} style={{ display: 'contents' }}><dt>{r.k}</dt><dd>{r.v}</dd></div>)}
    </dl>
  )
}

export function Hint({ children }: { children: ReactNode }) {
  return <p className={styles.hint}>{children}</p>
}

export function Actions({ children }: { children: ReactNode }) {
  return <div className={styles.acts}>{children}</div>
}

export function Tabs<T extends string>({ value, tabs, onChange }: { value: T; tabs: { value: T; label: ReactNode }[]; onChange: (v: T) => void }) {
  return (
    <div className={styles.tabs} role="tablist">
      {tabs.map((t) => <button key={t.value} type="button" role="tab" aria-selected={t.value === value} onClick={() => onChange(t.value)}>{t.label}</button>)}
    </div>
  )
}
