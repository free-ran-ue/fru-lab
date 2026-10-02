// Shared number formatting for the tester pages.
export function formatMs(ms: number): string {
  if (!ms) return '—'
  if (ms >= 1000) return `${(ms / 1000).toFixed(2)} s`
  return `${ms.toFixed(ms < 10 ? 2 : 0)} ms`
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
