import { apiBaseUrl } from '../../apiClient'

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

// formatBits renders a byte count as bits in decimal units, e.g. 9.6 Gb:
// every amount of data on the tester pages is in bits, like the rates.
export function formatBits(bytes: number): string {
  const units = ['b', 'kb', 'Mb', 'Gb', 'Tb']
  let v = bytes * 8
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
export function buildTesterStreamUrl(testerId: string): string {
  const wsBase = apiBaseUrl.replace(/^http/, 'ws')
  const token = localStorage.getItem('token') ?? ''
  return `${wsBase}/api/testers/${encodeURIComponent(testerId)}/run/stream?token=${encodeURIComponent(token)}`
}

// formatBytes renders a memory size in binary units, e.g. 3.2 GiB.
export function formatBytes(bytes: number): string {
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB']
  let v = bytes
  let i = 0
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i += 1
  }
  return `${v.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

// formatDuration renders seconds as the two largest units, e.g. 3d 4h.
export function formatDuration(sec: number): string {
  const d = Math.floor(sec / 86400)
  const h = Math.floor((sec % 86400) / 3600)
  const m = Math.floor((sec % 3600) / 60)
  if (d > 0) return `${d}d ${h}h`
  if (h > 0) return `${h}h ${m}m`
  if (m > 0) return `${m}m ${sec % 60}s`
  return `${sec}s`
}

// formatBytesPair renders used of total in the total's binary unit,
// e.g. 11.2 / 64 GiB.
export function formatBytesPair(used: number, total: number): string {
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB']
  let div = 1
  let i = 0
  while (total / div >= 1024 && i < units.length - 1) {
    div *= 1024
    i += 1
  }
  const t = total / div
  return `${(used / div).toFixed(1)} / ${Number.isInteger(t) ? t : t.toFixed(1)} ${units[i]}`
}

// formatClock renders seconds as hh:mm:ss, e.g. 00:04:52.
export function formatClock(sec: number): string {
  const s = Math.max(0, Math.floor(sec))
  const p = (n: number) => String(n).padStart(2, '0')
  return `${p(Math.floor(s / 3600))}:${p(Math.floor(s / 60) % 60)}:${p(s % 60)}`
}
