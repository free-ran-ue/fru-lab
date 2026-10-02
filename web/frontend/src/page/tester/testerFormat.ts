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
export function buildTesterStreamUrl(): string {
  const wsBase = apiBaseUrl.replace(/^http/, 'ws')
  const token = localStorage.getItem('token') ?? ''
  return `${wsBase}/api/tester/run/stream?token=${encodeURIComponent(token)}`
}
