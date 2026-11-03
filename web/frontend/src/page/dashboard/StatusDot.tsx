import { getStatusMeta } from './statusMeta'
import type { NodeStatus } from './types'

interface StatusDotProps {
  status: NodeStatus
  // diameter in px of the prototype's svg (8 in the topology, 10 in the KPI strip)
  size?: 8 | 10
}

// The prototype's status dot: a filled circle per status, and a hollow
// ring for a target that isn't deployed. Sized in em-free rem so it
// scales with the fluid root font size.
export default function StatusDot({ status, size = 8 }: StatusDotProps) {
  const rem = `${size / 16}rem`
  const c = size / 2
  return (
    <svg width={rem} height={rem} viewBox={`0 0 ${size} ${size}`} aria-hidden="true" style={{ flexShrink: 0 }}>
      {status === 'stopped'
        ? <circle cx={c} cy={c} r={c - 1} fill="none" style={{ stroke: 'var(--chrome-muted)' }} strokeWidth={1.2} />
        : <circle cx={c} cy={c} r={c - 0.5} style={{ fill: getStatusMeta(status).dot }} />}
    </svg>
  )
}
