import type { ReactNode } from 'react'
import { ID, ueGnbId } from './canvas/graph'
import type { IconName } from './canvas/icons'
import { templateLabel, type Lab } from './useLab'
import type { NodeStatus } from './types'

export interface NodeInfo {
  kind: 'core' | 'nf' | 'gnb' | 'fleet' | 'ue' | 'dn'
  icon: IconName
  name: string
  sub?: string
  status?: NodeStatus
  statusText?: string
  rows: { k: string; v: ReactNode }[]
  services?: { name: string; status: NodeStatus }[]
  // the page where this node is deployed
  page?: string
  // for kind 'ue' / 'nf' / 'gnb'
  ueId?: string
  nf?: string
}

// nodeInfo describes a canvas node (by its graph id) for the floating card.
export function nodeInfo(lab: Lab, id: string): NodeInfo | null {
  const core = lab.nodes.core
  if (id === ID.externalCore || (id === ID.core && lab.gnbExternal && !lab.coreUp)) {
    const ext = lab.externalGnbs
    return {
      kind: 'core', icon: 'core', name: 'External core', sub: 'a 5G core outside the lab', page: '/free-ran',
      rows: [
        { k: 'AMF', v: [...new Set(ext.map((g) => g.deployment?.amfIp).filter(Boolean))].join(', ') || '—' },
        { k: 'gNBs on it', v: ext.map((g) => g.profileName).join(', ') || '—' },
        { k: 'gNB host IP', v: ext.map((g) => g.deployment?.hostIp).filter(Boolean).join(', ') || '—' },
      ],
    }
  }
  if (id === ID.core) {
    const nfs = lab.free5gc.networkFunctions
    return {
      kind: 'core', icon: 'core', name: 'free5GC core', sub: '5G core network', status: core.status, page: '/core-network',
      rows: [
        { k: 'Template', v: lab.runningTemplate ? templateLabel(lab.runningTemplate) : '—' },
        { k: 'Network functions', v: nfs.length ? `${lab.healthyNfs} / ${nfs.length} healthy` : '—' },
        { k: 'gNBs on it', v: lab.gnbs.filter((g) => g.core === 'lab').length },
        { k: 'Last deployed', v: core.lastDeployed },
      ],
    }
  }
  if (id.startsWith('nf:')) {
    const name = id.slice(3)
    const nf = lab.free5gc.networkFunctions.find((n) => n.name === name)
    return {
      kind: 'nf', icon: 'nf', name, sub: 'free5GC network function', status: nf?.status ?? 'stopped', nf: name, page: '/core-network',
      rows: [
        { k: 'Service', v: name.toLowerCase() },
        { k: 'Core', v: core.status },
        { k: 'Last deployed', v: core.lastDeployed },
      ],
    }
  }
  if (id === ID.gnbNone) {
    return { kind: 'gnb', icon: 'gnb', name: 'gNB', sub: 'none is running', status: 'stopped', page: '/free-ran', rows: [{ k: 'Deploy', v: 'pick a gNB profile on free-ran' }] }
  }
  const gnb = lab.gnbs.find((g) => g.id === id)
  if (gnb) {
    const d = gnb.deployment
    return {
      kind: 'gnb', icon: 'gnb', name: gnb.profileName, sub: gnb.name, status: gnb.status, page: '/free-ran',
      rows: [
        { k: 'Profile', v: gnb.profileName },
        { k: 'Core', v: gnb.core === 'external' ? `external · AMF ${d?.amfIp ?? ''}` : 'lab free5GC' },
        ...(d?.n2Ip ? [{ k: 'N2 IP', v: d.n2Ip }] : []),
        ...(d?.ranIp ? [{ k: 'RAN IP (auto)', v: d.ranIp }] : []),
        ...(gnb.sd ? [{ k: 'SD', v: gnb.sd }] : []),
        { k: 'UEs attached', v: lab.upUes.filter((r) => ueGnbId(lab, r) === gnb.id).length },
        { k: 'Last deployed', v: gnb.lastDeployed },
      ],
      services: gnb.networkFunctions,
    }
  }
  if (id === ID.fleet) {
    const c = lab.ueCounts
    return {
      kind: 'fleet', icon: 'fleet', name: 'UE fleet', sub: 'one free-ran-ue container per UE profile', status: lab.nodes.ue.status, statusText: `${c.running} / ${c.total}`, page: '/free-ue',
      rows: [
        { k: 'Running', v: c.running },
        { k: 'Starting', v: c.starting },
        { k: 'Unhealthy', v: c.unhealthy },
        { k: 'UE profiles', v: c.total },
      ],
    }
  }
  if (id.startsWith('ue:')) {
    const ueId = id.slice(3)
    const row = lab.ue.rows.find((r) => r.ueId === ueId)
    if (!row) return null
    return {
      kind: 'ue', icon: 'ue', name: row.ueId, sub: 'UE instance', status: row.status, ueId, page: '/free-ue',
      rows: [
        { k: 'Source', v: row.source === 'profile' ? `profile · ${row.profileName}` : 'deployed earlier' },
        { k: 'gNB', v: row.status !== 'stopped' ? ueGnbId(lab, row) || '—' : '—' },
        { k: 'PLMN', v: row.plmnId },
        ...(row.sd ? [{ k: 'SD', v: row.sd }] : []),
        { k: 'Last deployed', v: row.lastDeployed },
      ],
    }
  }
  if (id === ID.dn) {
    return { kind: 'dn', icon: 'cloud', name: 'Data network', sub: 'where UE traffic leaves the core over N6', rows: [] }
  }
  return null
}
