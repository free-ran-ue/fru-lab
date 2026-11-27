import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { useLab } from '../LabContext'
import { parseEvent, type LabEvent, type UeStage } from './events'
import { openLogStream, sourceKey, type LogSource } from './stream'

const MAX_EVENTS = 3000
// the most UE log streams followed at once
const MAX_UE_STREAMS = 60

export interface UeLive {
  stage?: UeStage
  ip?: string
  tun?: string
  lastError?: string
  t?: number
}

interface Events {
  events: LabEvent[]
  ueLive: Record<string, UeLive>
  clear: () => void
}

const EventsContext = createContext<Events>({ events: [], ueLive: {}, clear: () => {} })

// EventsProvider follows the logs of everything that runs (the AMF, every
// gNB, every UE, the AMF of every free5GC core network) for as long as the user is signed in, and turns them into
// lab events: NG Setup, registration, PDU sessions, failures. The console's
// Events tab lists them; the canvas marks each UE with its last stage.
export function EventsProvider({ children }: { children: ReactNode }) {
  const lab = useLab()
  const [events, setEvents] = useState<LabEvent[]>([])
  const seen = useRef(new Set<string>())
  const pending = useRef<LabEvent[]>([])
  const streams = useRef(new Map<string, () => void>())

  const gnbIds = lab.gnbs.map((g) => g.id).join('|')
  // every free5GC core network that is up: its AMF's events are core:<id>'s
  const coreIds = lab.upCores.map((c) => c.id).join('|')
  const sources = useMemo(() => {
    const out: LogSource[] = []
    if (lab.coreUp) out.push({ target: 'free5gc', service: 'amf' })
    for (const id of coreIds.split('|').filter(Boolean)) out.push({ target: `core-${id}`, service: 'amf' })
    // every running gNB (gnb-<profileId>)
    for (const id of gnbIds.split('|').filter(Boolean)) out.push({ target: id as `gnb-${string}` })
    for (const r of lab.upUes.slice(0, MAX_UE_STREAMS)) out.push({ target: 'ue', instance: r.ueId })
    return out
  }, [lab.coreUp, coreIds, gnbIds, lab.upUes])
  const keys = sources.map(sourceKey).join('|')

  useEffect(() => {
    const want = new Map(sources.map((s) => [sourceKey(s), s]))
    for (const [key, close] of streams.current) {
      if (!want.has(key)) { close(); streams.current.delete(key) }
    }
    for (const [key, src] of want) {
      if (streams.current.has(key)) continue
      streams.current.set(key, openLogStream(src, 300, (line) => {
        const ev = parseEvent(src, line)
        if (!ev || seen.current.has(ev.id)) return
        seen.current.add(ev.id)
        pending.current.push(ev)
      }))
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [keys])

  useEffect(() => () => { for (const close of streams.current.values()) close() }, [])

  // batch incoming events into one render every 300 ms
  useEffect(() => {
    const timer = window.setInterval(() => {
      if (pending.current.length === 0) return
      const batch = pending.current
      pending.current = []
      setEvents((cur) => [...cur, ...batch].sort((a, b) => a.t - b.t).slice(-MAX_EVENTS))
    }, 300)
    return () => window.clearInterval(timer)
  }, [])

  const ueLive = useMemo(() => {
    const out: Record<string, UeLive> = {}
    for (const e of events) {
      if (!e.ue?.id) continue
      const cur = (out[e.ue.id] ??= {})
      if (e.ue.stage) {
        // a fresh start wipes the last run's state
        if (e.ue.stage === 'connecting') { delete cur.ip; delete cur.lastError }
        cur.stage = e.ue.stage
        cur.t = e.t
      }
      if (e.ue.ip) cur.ip = e.ue.ip
      if (e.ue.tun) cur.tun = e.ue.tun
      if (e.level === 'error') cur.lastError = e.text
    }
    return out
  }, [events])

  const clear = useCallback(() => setEvents([]), [])

  return <EventsContext.Provider value={{ events, ueLive, clear }}>{children}</EventsContext.Provider>
}

export const useEvents = () => useContext(EventsContext)
