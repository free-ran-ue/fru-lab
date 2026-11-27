import { apiBaseUrl } from '../apiClient'

// A browser WebSocket can't set an Authorization header on its handshake
// request, so the ue terminal endpoint takes the JWT as a query param
// instead (validated server-side the same way, see backend's
// handleUeTerminal) - this mirrors apiClient.ts's own base path resolution.
export function buildUeTerminalUrl(instance: string): string {
  const wsBase = apiBaseUrl.replace(/^http/, 'ws')
  const token = localStorage.getItem('token') ?? ''
  return `${wsBase}/api/deploy/ue/${encodeURIComponent(instance)}/terminal?token=${encodeURIComponent(token)}`
}

// buildTerminalUrl is the shell of a topology node: a UE (ue:<imsi>) or a
// profile gNB (gnb-<profileId>).
export function buildTerminalUrl(node: string): string {
  if (/^gnb-[0-9a-f]{12}$/.test(node)) {
    const wsBase = apiBaseUrl.replace(/^http/, 'ws')
    const token = localStorage.getItem('token') ?? ''
    return `${wsBase}/api/deploy/gnbs/${encodeURIComponent(node.slice(4))}/terminal?token=${encodeURIComponent(token)}`
  }
  return buildUeTerminalUrl(node.replace(/^ue:/, ''))
}
