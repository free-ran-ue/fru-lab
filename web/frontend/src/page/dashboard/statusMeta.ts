import type { NodeStatus } from './types'

export interface StatusMeta {
  text: string
  color: string
  bg: string
  border: string
  dot: string
}

const STATUS_META: Record<NodeStatus, StatusMeta> = {
  running: { text: 'Running', color: '#15803d', bg: '#dcfce7', border: '#86efac', dot: '#16a34a' },
  unhealthy: { text: 'Unhealthy', color: '#b91c1c', bg: '#fee2e2', border: '#fca5a5', dot: '#dc2626' },
  deploying: { text: 'Deploying', color: '#b45309', bg: '#fef3c7', border: '#fcd34d', dot: '#d97706' },
  stopped: { text: 'Stopped', color: '#475569', bg: '#f1f5f9', border: '#cbd5e1', dot: '#94a3b8' },
}

export function getStatusMeta(status: NodeStatus): StatusMeta {
  return STATUS_META[status]
}
