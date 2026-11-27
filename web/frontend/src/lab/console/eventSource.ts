import type { Lab } from '../useLab'
import type { LabEvent } from './events'
import type { NodeStatus } from '../types'

// eventSource names what an event is about, for lists and tables: the UE
// (its IMSI), the gNB (its profile), the core (free5GC's AMF or a core
// network's name).
export function eventSource(lab: Lab, e: LabEvent): string {
  if (e.ue) return e.ue.id.replace(/^imsi-/, 'UE ')
  if (e.node === 'core') return 'free5GC · AMF'
  if (e.node.startsWith('core:')) return `${lab.coreNetworks.networks.find((n) => `core:${n.id}` === e.node)?.name ?? e.node.slice(5)} · AMF`
  if (e.node.startsWith('gnb-')) return `gNB ${lab.gnbs.find((g) => g.id === e.node)?.profileName ?? lab.gnbProfiles.profiles.find((p) => `gnb-${p.id}` === e.node)?.name ?? e.node}`
  if (e.node.startsWith('ue:')) return e.node.slice(3).replace(/^imsi-/, 'UE ')
  return e.node
}

// nodeLabel names a node id the way eventSource names an event's
export const nodeLabel = (lab: Lab, node: string) => eventSource(lab, { id: '', t: 0, node, level: 'info', text: '' })

// the node to filter or select by: the UE's for a UE event, else the node
export const eventNode = (e: LabEvent) => (e.ue ? `ue:${e.ue.id}` : e.node)

// an event level drawn as a status dot
export const levelStatus = (level: LabEvent['level']): NodeStatus => (level === 'error' ? 'unhealthy' : level === 'warn' ? 'deploying' : level === 'ok' ? 'running' : 'stopped')

export const clockOf = (t: number) => {
  const d = new Date(t)
  return `${d.toISOString().slice(0, 10)} ${d.toISOString().slice(11, 23)}`
}
