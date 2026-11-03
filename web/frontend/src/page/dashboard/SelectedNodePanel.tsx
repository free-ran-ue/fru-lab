import type { ReactNode } from 'react'
import { StatusSwatch, StatusTag } from './StatusTag'
import type { NetworkFunction, NodeStatus } from './types'
import styles from './dashboard-page.module.css'

export interface DetailRow {
  k: string
  v: ReactNode
  mono?: boolean
}

interface SelectedNodePanelProps {
  name: string
  status: NodeStatus
  // overrides the tag's text (e.g. "2 / 3" for the UE fleet)
  statusText?: string
  rows: DetailRow[]
  // the node's services (core NFs, gNB slices); a click selects one when
  // onSelectService is given
  services?: NetworkFunction[]
  selectedService?: string | null
  onSelectService?: (name: string) => void
  // why the primary action is disabled
  hint?: string
  actions: ReactNode
}

// The v2 "Selected node" aside: name + tag, key/value list, a Services
// table, then the blocked-reason note and the action buttons.
export default function SelectedNodePanel({ name, status, statusText, rows, services, selectedService, onSelectService, hint, actions }: SelectedNodePanelProps) {
  const healthy = services?.filter((nf) => nf.status === 'running').length ?? 0
  return (
    <aside className={`${styles.panel} ${styles.aside}`} aria-label="Selected node">
      <div className={styles.ph}>
        <h2 className={styles.asideName}>{name}</h2>
        <span className={styles.tagRight}><StatusTag status={status} text={statusText} /></span>
      </div>
      <dl className={styles.kv}>
        {rows.map((row) => (
          <div key={row.k} className={styles.kvRow}>
            <dt>{row.k}</dt>
            <dd className={row.mono ? 'num' : undefined}>{row.v}</dd>
          </div>
        ))}
      </dl>
      {services && services.length > 0 && (
        <>
          <div className={`${styles.ph} ${styles.phFlat}`}>
            <h2>Services</h2>
            <span className={`num ${styles.aux}`}>{healthy} / {services.length} healthy</span>
          </div>
          <table className={styles.tbl}>
            <thead><tr><th>NF</th><th>Status</th></tr></thead>
            <tbody>
              {services.map((nf) => {
                const on = nf.name === selectedService
                return (
                  <tr key={nf.name} className={on ? styles.rowOn : undefined}>
                    <td>
                      {onSelectService ? (
                        <button type="button" className={`num ${styles.cellButton}`} aria-pressed={on} onClick={() => onSelectService(nf.name)}>
                          {nf.name}
                        </button>
                      ) : <span className="num">{nf.name}</span>}
                    </td>
                    <td><StatusTag status={nf.status} /></td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </>
      )}
      <div className={styles.acts}>
        {hint && <p><StatusSwatch status="deploying" />{hint}</p>}
        <div>{actions}</div>
      </div>
    </aside>
  )
}
