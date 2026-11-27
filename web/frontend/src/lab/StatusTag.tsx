import { getStatusMeta } from './statusMeta'
import type { NodeStatus } from './types'
import styles from './status-tag.module.css'

const TONE: Record<NodeStatus, string> = {
  running: styles.ok,
  deploying: styles.warn,
  unhealthy: styles.bad,
  stopped: styles.idle,
}

// The v2 status tag: bordered, uppercase mono, with a round swatch.
export function StatusTag({ status, text }: { status: NodeStatus; text?: string }) {
  return (
    <span className={`${styles.tag} ${TONE[status]}`}>
      <i aria-hidden="true" />
      {(text ?? getStatusMeta(status).text).toUpperCase()}
    </span>
  )
}

// A bare round swatch in a status colour (legend, blocked-reason note).
export function StatusSwatch({ status }: { status: NodeStatus }) {
  return <span aria-hidden="true" className={`${styles.swatch} ${TONE[status]}`} />
}
