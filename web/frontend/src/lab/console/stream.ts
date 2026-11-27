import { apiBaseUrl } from '../../apiClient'

export interface LogLine {
  t: string
  src: string
  line: string
  err?: boolean
}

// a gNB streams as its instance name, gnb-<profileId>
export type StreamTarget = 'free5gc' | 'ue' | `gnb-${string}`

export interface LogSource {
  target: StreamTarget
  instance?: string
  service?: string
}

export const sourceKey = (s: LogSource) => [s.target, s.instance ?? '', s.service ?? ''].join(':')

// wsUrl builds a websocket URL on fru-lab's API; the JWT goes as ?token=
// because a browser cannot set headers on the handshake.
export function wsUrl(path: string, params: Record<string, string | number | undefined>): string {
  const q = new URLSearchParams({ token: localStorage.getItem('token') ?? '' })
  for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== '') q.set(k, String(v))
  return `${apiBaseUrl.replace(/^http/, 'ws')}/api${path}?${q}`
}

// openLogStream follows a source's logs (the last `tail` lines first) and
// reconnects after a drop until it is closed. It returns the close
// function.
export function openLogStream(src: LogSource, tail: number, onLine: (l: LogLine) => void, onState?: (s: 'open' | 'closed' | 'error', detail?: string) => void): () => void {
  let socket: WebSocket | null = null
  let disposed = false
  let retry: number | undefined
  const connect = (first: boolean) => {
    socket = new WebSocket(wsUrl('/stream/logs', { target: src.target, instance: src.instance, service: src.service, tail: first ? tail : 0 }))
    socket.onopen = () => onState?.('open')
    socket.onmessage = (e) => {
      try {
        const msg = JSON.parse(e.data as string)
        if (msg.error) onState?.('error', msg.error)
        else onLine(msg as LogLine)
      } catch { /* not ours */ }
    }
    socket.onclose = () => {
      if (disposed) return
      onState?.('closed')
      retry = window.setTimeout(() => connect(false), 2500)
    }
  }
  connect(true)
  return () => {
    disposed = true
    window.clearTimeout(retry)
    socket?.close()
  }
}
