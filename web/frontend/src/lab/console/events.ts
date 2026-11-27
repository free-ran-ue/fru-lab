import type { LogLine, LogSource } from './stream'

export type EventLevel = 'ok' | 'info' | 'warn' | 'error'
export type UeStage = 'connecting' | 'connected' | 'registered' | 'pdu' | 'ready' | 'failed'

export interface LabEvent {
  id: string
  t: number
  // the node it is about, as a canvas id ("gnb-…", "core", "core:<id>" for
  // a free5GC core network, "ue:imsi-…")
  node: string
  level: EventLevel
  text: string
  // what it tells about a UE
  ue?: { id: string; stage?: UeStage; ip?: string; tun?: string }
}

// free-ran-ue and free5GC prefix lines with their own time and tags:
// "2026/11/27 06:59:27 [INFO][UE][PDU] PDU session UE IP: 10.60.0.1"
const RE_PREFIX = /^(?:\d{4}\/\d\d\/\d\d \d\d:\d\d:\d\d(?:\.\d+)? )?(?:\d{4}-\d\d-\d\dT[\d:.]+Z )?\[(\w+)\]((?:\[[^\]]*\])*)\s*(.*)$/

interface Rule {
  re: RegExp
  level: EventLevel
  text: (m: RegExpMatchArray) => string
  ue?: (m: RegExpMatchArray, src: LogSource) => LabEvent['ue']
}

const ueOf = (src: LogSource) => src.instance ?? ''

const GNB: Rule[] = [
  { re: /Connected to AMF: (\S+)/, level: 'info', text: (m) => `SCTP connected to AMF ${m[1]}` },
  { re: /N2 setup complete/, level: 'ok', text: () => 'NG Setup complete' },
  { re: /Error connecting to AMF/, level: 'error', text: () => 'Cannot reach the AMF (N2)' },
  { re: /Connected to UPF: (\S+)/, level: 'ok', text: (m) => `N3 tunnel to UPF ${m[1]}` },
  { re: /UE (imsi-\d+) initialized/, level: 'info', text: (m) => `${m[1]} attached`, ue: (m) => ({ id: m[1] }) },
  { re: /UE (imsi-\d+) PDU session establishment completed/, level: 'ok', text: (m) => `${m[1]} PDU session resources set up`, ue: (m) => ({ id: m[1] }) },
]

const UE: Rule[] = [
  { re: /Connected to RAN control plane: (\S+)/, level: 'info', text: (m) => `Connected to gNB ${m[1]}`, ue: (_m, s) => ({ id: ueOf(s), stage: 'connected' }) },
  { re: /UE Registration finished/, level: 'ok', text: () => 'Registered', ue: (_m, s) => ({ id: ueOf(s), stage: 'registered' }) },
  { re: /PDU session UE IP: (\S+)/, level: 'ok', text: (m) => `PDU session up, IP ${m[1]}`, ue: (m, s) => ({ id: ueOf(s), stage: 'pdu', ip: m[1] }) },
  { re: /UE tunnel device setup as (\S+)/, level: 'info', text: (m) => `Tunnel ${m[1]} up`, ue: (m, s) => ({ id: ueOf(s), tun: m[1] }) },
  { re: /^UE started$/, level: 'ok', text: () => 'Ready for traffic', ue: (_m, s) => ({ id: ueOf(s), stage: 'ready' }) },
  { re: /Starting UE/, level: 'info', text: () => 'Starting', ue: (_m, s) => ({ id: ueOf(s), stage: 'connecting' }) },
]

const AMF: Rule[] = [
  { re: /SCTP Accept from: (\S+)/, level: 'info', text: (m) => `AMF accepted SCTP from ${m[1]}` },
  { re: /Send NG-Setup response/, level: 'ok', text: () => 'AMF answered NG Setup' },
  { re: /\[supi:SUPI:(imsi-\d+)\] Send Registration Accept/, level: 'ok', text: (m) => `AMF accepted registration of ${m[1]}`, ue: (m) => ({ id: m[1] }) },
  { re: /\[supi:SUPI:(imsi-\d+)\].*(Registration Reject|Authentication Reject|Authentication Failure)/i, level: 'error', text: (m) => `AMF: ${m[2]} for ${m[1]}`, ue: (m) => ({ id: m[1], stage: 'failed' }) },
  { re: /\[supi:SUPI:(imsi-\d+)\] create smContext\[pduSessionID: (\d+)\] Success/, level: 'info', text: (m) => `AMF created PDU session ${m[2]} for ${m[1]}`, ue: (m) => ({ id: m[1] }) },
]

// parseEvent turns one log line into a lab event, or null for the noise.
export function parseEvent(src: LogSource, l: LogLine): LabEvent | null {
  const m = l.line.match(RE_PREFIX)
  const level = m?.[1]?.toUpperCase() ?? ''
  const tags = m?.[2] ?? ''
  const msg = (m?.[3] ?? l.line).trim()
  const t = Date.parse(l.t) || Date.now()
  const id = `${src.target}:${src.instance ?? ''}:${l.t}:${l.line.length}:${msg.slice(0, 40)}`

  let rules: Rule[] = []
  // the lab free5GC is 'core'; a free5GC core network core:<id>
  let node = src.target.startsWith('core-') ? `core:${src.target.slice(5)}` : 'core'
  if (src.target === 'ue') { rules = UE; node = `ue:${ueOf(src)}` }
  else if (src.target.startsWith('gnb')) { rules = GNB; node = src.target }
  else if (src.service === 'amf' || tags.includes('[AMF]')) rules = AMF

  for (const r of rules) {
    const mm = msg.match(r.re) ?? (tags + msg).match(r.re)
    if (!mm) continue
    const ue = r.ue?.(mm, src)
    return { id, t, node, level: r.level, text: r.text(mm), ue }
  }
  // any error line from free-ran-ue is worth seeing
  if ((level === 'EROR' || level === 'ERRO' || level === 'ERROR' || level === 'FATA') && (src.target === 'ue' || src.target.startsWith('gnb'))) {
    return { id, t, node, level: 'error', text: msg, ue: src.target === 'ue' ? { id: ueOf(src), stage: 'failed' } : undefined }
  }
  return null
}

export const STAGE_LABEL: Record<UeStage, string> = {
  connecting: 'connecting',
  connected: 'at gNB',
  registered: 'registered',
  pdu: 'PDU up',
  ready: 'ready',
  failed: 'failed',
}
