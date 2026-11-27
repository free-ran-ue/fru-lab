import type { ReactNode } from 'react'
import styles from './workbench.module.css'

export { logSourceFor } from './logSources'

interface Props {
  // the toolbar: view name and status on the left, actions on the right
  title: ReactNode
  status?: ReactNode
  actions?: ReactNode
  // either a topology with an inspector beside it, or one body (a list
  // and its detail, a log view) across the full width
  canvas?: ReactNode
  inspector?: ReactNode
  body?: ReactNode
}

// Workbench lays out a lab view: the toolbar, then a topology with the
// inspector on its right, or one full-width body.
export default function Workbench({ title, status, actions, canvas, inspector, body }: Props) {
  return (
    <div className={`${styles.wb} ${body ? styles.wbBody : ''}`}>
      <div className={styles.toolbar}>
        <h1 className={styles.title}>{title}</h1>
        {status && <div className={styles.status}>{status}</div>}
        {actions && <div className={styles.actions}>{actions}</div>}
      </div>
      {body
        ? <div className={styles.body}>{body}</div>
        : (
          <>
            <div className={styles.canvas}>{canvas}</div>
            <aside className={styles.inspector} aria-label="Inspector">{inspector}</aside>
          </>
        )}
    </div>
  )
}

export { Workbench }
