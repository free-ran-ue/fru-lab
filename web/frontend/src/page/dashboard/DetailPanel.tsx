import type { ReactNode } from 'react'
import { getStatusMeta } from './statusMeta'
import type { NodeStatus } from './types'
import styles from './detail-panel.module.css'

export interface DetailRow {
  k: string
  v: ReactNode
}

interface DetailPanelProps {
  name: string
  status: NodeStatus
  // overrides the tag's text (e.g. "2 / 3" for the UE fleet)
  statusText?: string
  rows: DetailRow[]
  // extra body content after the key/value rows (the UE instance list)
  children?: ReactNode
  // why the primary action is disabled, shown above the footer
  hint?: string
  footer: ReactNode
}

// The prototype's "Selected node" aside: eyebrow, name + status tag,
// key/value rows and a footer row of buttons.
export default function DetailPanel({ name, status, statusText, rows, children, hint, footer }: DetailPanelProps) {
  const meta = getStatusMeta(status)
  return (
    <aside aria-label="Selected node" className={styles.aside}>
      <div className={styles.head}>
        <span className={styles.eyebrow}>SELECTED NODE</span>
        <div className={styles.nameRow}>
          <span className={styles.name}>{name}</span>
          <span className={styles.tag} style={{ color: meta.color, background: meta.bg }}>
            {(statusText ?? meta.text).toUpperCase()}
          </span>
        </div>
      </div>
      <div className={styles.body}>
        {rows.map((row) => (
          <div key={row.k} className={styles.row}>
            <span className={styles.rowKey}>{row.k}</span>
            <span className={`num ${styles.rowValue}`}>{row.v}</span>
          </div>
        ))}
        {children}
      </div>
      {hint && <p className={styles.hint}>{hint}</p>}
      <div className={styles.footer}>{footer}</div>
    </aside>
  )
}
