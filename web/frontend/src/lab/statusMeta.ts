import type { NodeStatus } from './types'

export interface StatusMeta {
  text: string
  color: string
  bg: string
  border: string
  dot: string
}

// values are CSS custom properties from index.css, so they work in inline
// `style` objects (including SVG `fill`/`stroke` set via style), not in raw
// SVG presentation attributes.
const STATUS_META: Record<NodeStatus, StatusMeta> = {
  running: { text: 'Running', color: 'var(--ok-ink)', bg: 'var(--ok-bg)', border: 'var(--ok-bg)', dot: 'var(--ok)' },
  unhealthy: { text: 'Unhealthy', color: 'var(--bad-ink)', bg: 'var(--bad-bg)', border: 'var(--bad-bg)', dot: 'var(--bad)' },
  deploying: { text: 'Deploying', color: 'var(--warn-ink)', bg: 'var(--warn-bg)', border: 'var(--warn-bg)', dot: 'var(--warn)' },
  stopped: { text: 'Stopped', color: 'var(--idle-ink)', bg: 'var(--idle-bg)', border: 'var(--idle-bg)', dot: 'var(--chrome-muted)' },
}

export function getStatusMeta(status: NodeStatus): StatusMeta {
  return STATUS_META[status]
}
