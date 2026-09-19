// A browser WebSocket can't set an Authorization header on its handshake
// request, so the ue terminal endpoint takes the JWT as a query param
// instead (validated server-side the same way, see backend's
// handleUeTerminal) - this mirrors apiClient.ts's own base path resolution.
export function buildUeTerminalUrl(instance: string): string {
  const httpBase = import.meta.env.VITE_API_BASE_URL || `${window.location.protocol}//${window.location.hostname}:8888`
  const wsBase = httpBase.replace(/^http/, 'ws')
  const token = localStorage.getItem('token') ?? ''
  return `${wsBase}/api/deploy/ue/${encodeURIComponent(instance)}/terminal?token=${encodeURIComponent(token)}`
}
