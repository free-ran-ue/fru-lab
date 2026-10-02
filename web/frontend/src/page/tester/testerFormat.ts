// Shared number formatting for the tester pages.
export function formatMs(ms: number): string {
  if (!ms) return '—'
  if (ms >= 1000) return `${(ms / 1000).toFixed(2)} s`
  return `${ms.toFixed(ms < 10 ? 2 : 0)} ms`
}

// formatBps renders a bit rate, e.g. 874.8 Mbps.
export function formatBps(bps: number): string {
  if (!bps) return '0 bps'
  const units = ['bps', 'kbps', 'Mbps', 'Gbps']
  let v = bps
  let i = 0
  while (v >= 1000 && i < units.length - 1) {
    v /= 1000
    i += 1
  }
  return `${v.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

// formatBytes renders a byte count in decimal units, e.g. 1.2 GB.
export function formatBytes(bytes: number): string {
  const units = ['B', 'kB', 'MB', 'GB', 'TB']
  let v = bytes
  let i = 0
  while (v >= 1000 && i < units.length - 1) {
    v /= 1000
    i += 1
  }
  return `${v.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

// formatLoss renders a loss ratio as a percentage, e.g. 0.49 %.
export function formatLoss(rate: number): string {
  return `${(rate * 100).toFixed(rate > 0 && rate < 0.0001 ? 4 : 2)} %`
}

// A browser WebSocket can't set an Authorization header, so the stream
// takes the JWT as ?token= (checked by fru-lab's handleTesterStream),
// mirroring dashboard/terminalSocket.ts.
export function buildTesterStreamUrl(): string {
  const httpBase = import.meta.env.VITE_API_BASE_URL || `${window.location.protocol}//${window.location.hostname}:8888`
  const wsBase = httpBase.replace(/^http/, 'ws')
  const token = localStorage.getItem('token') ?? ''
  return `${wsBase}/api/tester/run/stream?token=${encodeURIComponent(token)}`
}
