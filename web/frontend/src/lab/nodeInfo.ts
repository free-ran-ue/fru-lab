import type { ReactNode } from 'react'
import { ID } from './canvas/graph'
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
  slice?: 'slice1' | 'slice2'
}

// nodeInfo describes a canvas node (by its graph id) for the floating card.
export function nodeInfo(lab: Lab, id: string): NodeInfo | null {
  const core = lab.nodes.core
  if (id === ID.core) {
    const nfs = lab.free5gc.networkFunctions
    if (lab.gnbExternal && !lab.coreUp) {
      const d = lab.gnb.gnbDeployment
      return {
        kind: 'core', icon: 'core', name: 'External core', sub: 'a 5G core outside the lab', page: '/free-ran',
        rows: [{ k: 'AMF', v: d?.amfIp ?? '—' }, { k: 'gNB profile', v: d?.profileName ?? '—' }, { k: 'gNB host IP', v: d?.hostIp ?? '—' }],
      }
    }
    return {
      kind: 'core', icon: 'core', name: 'free5GC core', sub: '5G core network', status: core.status, page: '/core-network',
      rows: [
        { k: 'Template', v: lab.runningTemplate ? templateLabel(lab.runningTemplate) : '—' },
        { k: 'Network functions', v: nfs.length ? `${lab.healthyNfs} / ${nfs.length} healthy` : '—' },
        { k: 'Subscribers', v: lab.ueCounts.total },
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
  if (id === ID.gnb || id === ID.slice('slice1') || id === ID.slice('slice2')) {
    const slice = id === ID.gnb ? undefined : (id === ID.slice('slice1') ? 'slice1' : 'slice2') as 'slice1' | 'slice2'
    const target = slice === 'slice1' ? lab.gnbSlice1 : slice === 'slice2' ? lab.gnbSlice2 : null
    const node = target ? target.node : lab.nodes.gnb
    return {
      kind: 'gnb', icon: 'gnb', name: slice ? `gNB ${slice}` : 'gNB', sub: 'free-ran-ue base station', status: node.status, slice, page: '/free-ran',
      rows: [
        ...(slice
          ? [{ k: 'Template', v: 'ULCL (Two Slice)' }]
          : [
            { k: 'Profile', v: node.status !== 'stopped' ? lab.gnb.gnbDeployment?.profileName ?? '—' : '—' },
            { k: 'Core', v: lab.gnbExternal ? `external · AMF ${lab.gnb.gnbDeployment?.amfIp}` : 'lab free5GC' },
          ]),
        { k: 'UEs attached', v: lab.upUes.length },
        { k: 'Last deployed', v: node.lastDeployed },
      ],
      services: target ? target.networkFunctions : lab.gnbNetworkFunctions,
    }
  }
  if (id === ID.fleet) {
    const c = lab.ueCounts
    return {
      kind: 'fleet', icon: 'fleet', name: 'UE fleet', sub: 'one free-ran-ue container per subscriber', status: lab.nodes.ue.status, statusText: `${c.running} / ${c.total}`, page: '/free-ue',
      rows: [
        { k: 'Running', v: c.running },
        { k: 'Starting', v: c.starting },
        { k: 'Unhealthy', v: c.unhealthy },
        { k: 'Subscribers', v: c.total },
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
        { k: 'Source', v: row.source === 'profile' ? `profile · ${row.profileName}` : row.source === 'subscriber' ? 'free5GC subscriber' : 'deployed earlier' },
        { k: 'PLMN', v: row.plmnId },
        ...(row.gpsi ? [{ k: 'GPSI', v: row.gpsi }] : []),
        ...(row.sd ? [{ k: 'Slice SD', v: row.sd }] : []),
        { k: 'Last deployed', v: row.lastDeployed },
      ],
    }
  }
  if (id === ID.dn) {
    return { kind: 'dn', icon: 'cloud', name: 'Data network', sub: 'where UE traffic leaves the core over N6', rows: [] }
  }
  return null
}
