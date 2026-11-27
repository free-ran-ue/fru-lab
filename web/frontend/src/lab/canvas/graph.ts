import type { Free5gcTemplate } from '../free5gcTemplate'
import type { NodeStatus } from '../types'
import type { Lab, LabGnb } from '../useLab'
import { isFree5gcCore, templateLabel } from '../useLab'
import type { UeRow } from '../useUeInstances'
import type { CanvasFrame, CanvasLink, CanvasNode } from './LabCanvas'
import type { UeLive } from '../console/EventsContext'
import { STAGE_LABEL } from '../console/events'

// Which page the map is drawn for. Every page shows the same lab; the focus
// picks what is expanded and what fades into the background:
//   all  - Dashboard: core NFs expanded, every gNB, the UE fleet as one node
//   core - Core Network: same, with the gNBs and UEs faded
//   ran  - free-ran: the core as one node, every gNB
//   ue   - free-ue: one node per UE, gNBs and core faded
export type Focus = 'all' | 'core' | 'ran' | 'ue'

export interface Graph {
  nodes: CanvasNode[]
  links: CanvasLink[]
  frames: CanvasFrame[]
  notes: { x: number; y: number; text: string; anchor?: 'start' | 'middle' | 'end' }[]
}

// node ids, shared with the pages' cards
export const ID = {
  core: 'core',
  // the core outside the lab that external-core gNBs dial
  externalCore: 'core-external',
  dn: 'dn',
  fleet: 'ue-fleet',
  // the ghost gNB drawn while none runs (not a real target)
  gnbNone: 'gnb-placeholder',
  // a gNB's node id is its target name, gnb-<profileId>
  gnb: (instance: string) => instance,
  nf: (name: string) => `nf:${name}`,
  ue: (ueId: string) => `ue:${ueId}`,
}

const TOP_NFS = ['NRF', 'AUSF', 'UDM', 'UDR', 'PCF', 'NSSF', 'CHF']
const PLANE_NFS = new Set(['AMF', 'SMF', 'UPF', 'I-UPF', 'PSA-UPF'])

const BIG = { w: 210, h: 80 }
const NF = { w: 120, h: 42 }
// control-plane NFs per row above the SBI bus; rows are staggered by half
// a step so the upper row's bus lines pass between the lower row's NFs
const NF_PER_ROW = 4
const NF_STEP = NF.w + 24
const UE = { w: 270, h: 34 }
const UE_ROWS = 14
// the most UE nodes drawn; the rest fold into one "+N more" node
const MAX_UE_NODES = 48
const UE_GAP = 12

interface Box { x: number; y: number; w: number; h: number }
const cx = (b: Box) => b.x + b.w / 2
const cy = (b: Box) => b.y + b.h / 2

// a smooth horizontal link from a's right edge to b's left edge
function hCurve(a: Box, b: Box): string {
  const x1 = a.x + a.w
  const y1 = cy(a)
  const x2 = b.x
  const y2 = cy(b)
  const dx = Math.max(30, (x2 - x1) / 2)
  return `M${x1} ${y1}C${x1 + dx} ${y1} ${x2 - dx} ${y2} ${x2} ${y2}`
}

// a smooth vertical link from a's bottom edge to b's top edge
function vCurve(a: Box, b: Box): string {
  const x1 = cx(a)
  const y1 = a.y + a.h
  const x2 = cx(b)
  const y2 = b.y
  const dy = Math.max(20, (y2 - y1) / 2)
  return `M${x1} ${y1}C${x1} ${y1 + dy} ${x2} ${y2 - dy} ${x2} ${y2}`
}

const mid = (a: Box, b: Box, horizontal: boolean, text: string) => (horizontal
  ? { x: (a.x + a.w + b.x) / 2, y: (cy(a) + cy(b)) / 2 - 8, text }
  : { x: (cx(a) + cx(b)) / 2 + 16, y: (a.y + a.h + b.y) / 2 + 4, text })

const up = (s: NodeStatus | undefined) => s === 'running'

export function buildGraph(lab: Lab, focus: Focus, previewTemplate: Free5gcTemplate, ueLive: Record<string, UeLive> = {}): Graph {
  const nodes: CanvasNode[] = []
  const links: CanvasLink[] = []
  const frames: CanvasFrame[] = []
  const notes: Graph['notes'] = []

  const template = lab.runningTemplate ?? previewTemplate
  const coreStatus = lab.nodes.core.status
  const nfStatus = new Map(lab.free5gc.networkFunctions.map((nf) => [nf.name, nf.status]))
  // the gNB dials a core outside the lab while free5GC is off: draw that
  // core as one node instead of free5GC's NFs
  const externalCore = lab.gnbExternal && !lab.coreUp && focus !== 'core'
  const expandCore = (focus === 'all' || focus === 'core') && !externalCore
  const ueEach = focus === 'ue'

  // ----- RAN side: every running gNB (or a ghost one while none runs) -----
  const gnbX = 300
  const GNB_STEP = 150
  const shownGnbs = lab.gnbs.length ? lab.gnbs : [null]
  const gnbs: { id: string; box: Box; status: NodeStatus; label: string; sub: string; tag: string; sd: string }[] = shownGnbs.map((g, i) => {
    const box = { x: gnbX, y: 260 + (i - (shownGnbs.length - 1) / 2) * GNB_STEP, ...BIG }
    if (!g) return { id: ID.gnbNone, box, status: 'stopped' as NodeStatus, label: 'gNB', sub: 'not deployed', tag: '0 UE attached', sd: '' }
    return {
      id: g.id,
      box,
      status: g.status,
      label: g.profileName,
      sub: g.name,
      tag: `${lab.upUes.filter((r) => ueGnbId(lab, r) === g.id).length} UE attached`,
      sd: g.sd,
    }
  })
  const ranGhost = focus === 'core' || focus === 'ue'
  for (const g of gnbs) {
    nodes.push({ id: g.id, ...g.box, icon: 'gnb', label: g.label, sub: g.sub, tag: g.tag, status: g.status, ghost: ranGhost || g.id === ID.gnbNone })
  }
  const gnbFor = (row: UeRow) => gnbs.find((g) => g.id === ueGnbId(lab, row)) ?? gnbs[0]
  const gnbCenterY = (gnbs[0].box.y + gnbs[gnbs.length - 1].box.y) / 2 + BIG.h / 2

  // ----- UE side -----
  if (ueEach) {
    // running UEs first, then the ones deployed before, then the rest
    const rank = (r: (typeof lab.ue.rows)[number]) => (r.status !== 'stopped' ? 0 : r.hasBeenDeployed ? 1 : 2)
    const sorted = [...lab.ue.rows].sort((a, b) => rank(a) - rank(b))
    const hidden = Math.max(0, sorted.length - MAX_UE_NODES)
    const rows = hidden ? sorted.slice(0, MAX_UE_NODES - 1) : sorted
    const drawn = rows.length + (hidden ? 1 : 0)
    const cols = Math.max(1, Math.ceil(drawn / UE_ROWS))
    const perCol = Math.min(UE_ROWS, Math.max(1, drawn))
    const centerY = gnbCenterY
    const step = UE.h + UE_GAP
    const top = centerY - (perCol * step - UE_GAP) / 2
    rows.forEach((row, i) => {
      const c = Math.floor(i / UE_ROWS)
      const r = i % UE_ROWS
      // every other column shifts half a row, so the outer columns' links
      // pass through the gaps of the inner ones
      const shift = (cols - 1 - c) % 2 ? step / 2 : 0
      const box = { x: gnbX - 140 - UE.w - (cols - 1 - c) * (UE.w + 40), y: top + r * step + shift, ...UE }
      const id = ID.ue(row.ueId)
      const live = row.status === 'stopped' ? undefined : ueLive[row.ueId]
      const tag = live?.stage ? [STAGE_LABEL[live.stage], live.stage !== 'failed' ? live.ip : undefined].filter(Boolean).join(' · ') : undefined
      nodes.push({ id, ...box, icon: 'ue', label: row.ueId.replace(/^imsi-/, ''), status: live?.stage === 'failed' && row.status === 'running' ? 'unhealthy' : row.status, tag, compact: true })
      const g = gnbFor(row)
      links.push({ id: `uu:${row.ueId}`, from: id, to: g.id, d: hCurve(box, g.box), live: up(row.status) && up(g.status) })
    })
    if (hidden) {
      const i = rows.length
      const c = Math.floor(i / UE_ROWS)
      const r = i % UE_ROWS
      const shift = (cols - 1 - c) % 2 ? step / 2 : 0
      const box = { x: gnbX - 140 - UE.w - (cols - 1 - c) * (UE.w + 40), y: top + r * step + shift, ...UE }
      nodes.push({ id: ID.fleet, ...box, icon: 'fleet', label: `+${hidden + 1} more UEs`, tag: 'see the list', compact: true })
    }
    if (rows.length === 0) {
      const box = { x: gnbX - 110 - BIG.w, y: 260, ...BIG }
      nodes.push({ id: ID.fleet, ...box, icon: 'fleet', label: 'No UEs yet', sub: 'create a UE profile' })
      links.push({ id: 'uu:empty', from: ID.fleet, to: gnbs[0].id, d: hCurve(box, gnbs[0].box), dashed: true })
    }
  } else {
    const box = { x: 0, y: gnbCenterY - BIG.h / 2, ...BIG }
    const counts = lab.ueCounts
    nodes.push({
      id: ID.fleet,
      ...box,
      icon: 'fleet',
      label: 'UE fleet',
      sub: `${counts.running} / ${counts.total} running`,
      status: lab.nodes.ue.status,
      dots: lab.ue.rows.map((r) => ({ id: r.ueId, status: r.status })),
      ghost: focus === 'core',
    })
    for (const g of gnbs) {
      links.push({ id: `uu:${g.id}`, from: ID.fleet, to: g.id, d: hCurve(box, g.box), live: lab.ueUp && up(g.status), label: mid(box, g.box, true, 'Uu') })
    }
  }

  // ----- core -----
  const coreX = gnbX + BIG.w + 110
  let dnAnchors: Box[] = []
  let coreRight: number
  if (expandCore) {
    const fx = coreX
    const fy = 40
    const x0 = fx + 34
    const extras = lab.free5gc.networkFunctions.map((n) => n.name).filter((n) => !TOP_NFS.includes(n) && !PLANE_NFS.has(n))
    const place = (name: string, x: number, y: number): Box => {
      const box = { x, y, ...NF }
      nodes.push({ id: ID.nf(name), ...box, icon: 'nf', label: name, status: nfStatus.get(name) ?? 'stopped', compact: true })
      return box
    }
    // control plane on the SBI bus, in staggered rows above it
    const control = [...TOP_NFS, ...extras]
    const rowCount = Math.ceil(control.length / NF_PER_ROW)
    const top = control.map((name, i) => {
      const row = Math.floor(i / NF_PER_ROW)
      const col = i % NF_PER_ROW
      return place(name, x0 + (col + (row % 2) * 0.5) * NF_STEP, fy + 60 + row * 58)
    })
    const busY = fy + 60 + rowCount * 58 + 10
    const amf = place('AMF', x0, busY + 34)
    const smfs: Box[] = [place('SMF', x0 + NF_STEP, busY + 34)]
    const busEnd = Math.max(...top.map((b) => b.x + b.w), ...smfs.map((b) => b.x + b.w))
    for (const b of [...top, amf, ...smfs]) {
      const name = nodes.find((n) => n.x === b.x && n.y === b.y)!.id
      const y = b.y < busY ? b.y + b.h : b.y
      links.push({ id: `sbi:${name}`, from: name, to: 'sbi', d: `M${cx(b)} ${y}V${busY}`, live: false })
    }
    links.push({ id: 'sbi-bus', from: 'sbi', to: 'sbi', d: `M${x0} ${busY}H${busEnd}`, live: false })
    notes.push({ x: busEnd + 10, y: busY + 4, text: 'SBI', anchor: 'start' })

    // user plane lanes
    const laneY = busY + 140
    const lanes: { first: Box; last: Box; smf: Box }[] = []
    if (template === 'basic') {
      const upf = place('UPF', smfs[0].x, laneY)
      lanes.push({ first: upf, last: upf, smf: smfs[0] })
    } else {
      const i = place('I-UPF', smfs[0].x, laneY)
      const p = place('PSA-UPF', smfs[0].x + 2 * NF_STEP, laneY)
      lanes.push({ first: i, last: p, smf: smfs[0] })
    }
    for (const [i, lane] of lanes.entries()) {
      const smfId = 'nf:SMF'
      const firstId = nodes.find((n) => n.x === lane.first.x && n.y === lane.first.y)!.id
      const lastId = nodes.find((n) => n.x === lane.last.x && n.y === lane.last.y)!.id
      links.push({ id: `n4:${firstId}`, from: smfId, to: firstId, d: vCurve(lane.smf, lane.first), live: up(coreStatus), label: mid(lane.smf, lane.first, false, 'N4') })
      if (lane.last !== lane.first) {
        links.push({ id: `n4:${lastId}`, from: smfId, to: lastId, d: vCurve(lane.smf, lane.last), dashed: true, live: false })
        links.push({ id: `n9:${lastId}`, from: firstId, to: lastId, d: hCurve(lane.first, lane.last), live: up(coreStatus), label: mid(lane.first, lane.last, true, 'N9') })
      }
      // N3 from every gNB to the lane's first UPF
      for (const [j, g] of gnbs.entries()) {
        links.push({ id: `n3:${firstId}:${g.id}`, from: g.id, to: firstId, d: hCurve(g.box, lane.first), live: up(g.status) && up(coreStatus), label: i === 0 && j === 0 ? mid(g.box, lane.first, true, 'N3') : undefined })
      }
    }
    for (const g of gnbs) {
      links.push({ id: `n2:${g.id}`, from: g.id, to: ID.nf('AMF'), d: hCurve(g.box, amf), live: up(g.status) && up(coreStatus), label: mid(g.box, amf, true, 'N2') })
    }
    dnAnchors = lanes.map((l) => l.last)

    const right = Math.max(...nodes.filter((n) => n.id.startsWith('nf:')).map((n) => n.x + n.w)) + 64
    const bottom = Math.max(...nodes.filter((n) => n.id.startsWith('nf:')).map((n) => n.y + n.h)) + 34
    const nfs = lab.free5gc.networkFunctions
    frames.unshift({
      id: ID.core,
      x: fx,
      y: fy,
      w: right - fx,
      h: bottom - fy,
      label: 'free5GC core',
      sub: `${templateLabel(template).toUpperCase()} · ${nfs.length ? `${lab.healthyNfs}/${nfs.length} NF` : lab.runningTemplate ? '' : 'preview'}`,
      status: coreStatus,
    })
    coreRight = right
  } else {
    const box = { x: coreX, y: gnbCenterY - BIG.h / 2, ...BIG }
    const nfs = lab.free5gc.networkFunctions
    const ext = lab.externalGnbs[0]?.deployment
    nodes.push(externalCore
      ? { id: ID.core, ...box, icon: 'core', label: 'External core', sub: `AMF ${ext?.amfIp ?? ''}`, tag: 'outside the lab', ghost: focus !== 'all' }
      : {
        id: ID.core,
        ...box,
        icon: 'core',
        label: 'free5GC core',
        sub: lab.runningTemplate ? templateLabel(template) : 'not deployed',
        tag: nfs.length ? `${lab.healthyNfs} / ${nfs.length} NF healthy` : undefined,
        status: coreStatus,
        ghost: focus !== 'all',
      })
    for (const g of gnbs) {
      links.push({ id: `n2n3:${g.id}`, from: g.id, to: ID.core, d: hCurve(g.box, box), live: up(g.status) && (externalCore || up(coreStatus)), dashed: externalCore, label: mid(g.box, box, true, 'N2 · N3') })
    }
    dnAnchors = [box]
    coreRight = box.x + box.w
  }

  // ----- data network -----
  const first = dnAnchors[0]
  const last = dnAnchors[dnAnchors.length - 1]
  const dn = { x: coreRight + 80, y: (cy(first) + cy(last)) / 2 - 36, w: 150, h: 72 }
  nodes.push({ id: ID.dn, ...dn, icon: 'cloud', label: 'DN', sub: 'internet' })
  for (const [i, a] of dnAnchors.entries()) {
    const from = expandCore ? nodes.find((n) => n.x === a.x && n.y === a.y)!.id : ID.core
    links.push({ id: `n6:${i}`, from, to: ID.dn, d: hCurve(a, dn), live: up(coreStatus), dashed: externalCore, label: i === 0 ? mid(a, dn, true, 'N6') : undefined })
  }

  // Core Network shows the core alone: its NFs and their links
  if (focus === 'core') {
    const isCore = (id: string) => id.startsWith('nf:') || id === 'sbi'
    return { nodes: nodes.filter((n) => isCore(n.id)), links: links.filter((l) => isCore(l.from) && isCore(l.to)), frames, notes }
  }

  return { nodes, links, frames, notes }
}

// gnbIdFor guesses the gNB a UE with this sd attaches to, as the backend
// picks one when no gNB is given: a running gNB with the same SD, else the
// first running one. Only the fallback for a UE whose status has no
// recorded gnb.
export function gnbIdFor(lab: Lab, sd: string): string {
  const gnbs = lab.gnbs
  return (gnbs.find((g) => g.sd && g.sd.toLowerCase() === sd.toLowerCase()) ?? gnbs[0])?.id ?? ''
}

// ueGnbId is the gNB a UE is on: the one the backend recorded when it was
// deployed, else the SD guess.
export function ueGnbId(lab: Lab, row: UeRow): string {
  return row.gnb || gnbIdFor(lab, row.sd)
}

// the most UEs the overview draws one by one; the rest fold into "+N more"
const MAX_OVERVIEW_UES = 24
const RADIO_GAP = 150

const gnbSub = (g: LabGnb, lab: Lab) => (g.coreNetwork
  ? (lab.coreNetworks.networks.find((n) => n.id === g.coreNetwork)?.name ?? 'a deleted core')
  : g.core === 'external' ? `external · AMF ${g.deployment?.amfIp ?? ''}` : 'lab free5GC')

// buildOverviewGraph is the overview map: every connected UE on the radio
// to its gNB, every running gNB, the core(s) as one node each and the data
// network.
// one core as the Overview draws it (topology and twin alike)
export interface OverviewCore {
  id: string
  node: Omit<CanvasNode, 'x' | 'y' | 'w' | 'h' | 'id'>
  live: boolean
  dashed: boolean
  dns: string[]
}

// overviewCores is which cores the Overview shows, in order, and which of
// them each gNB belongs to (every gNB lands on a shown core).
export function overviewCores(lab: Lab): { specs: OverviewCore[]; laneOf: (g: LabGnb) => string } {
  const coreStatus = lab.nodes.core.status
  // ----- which cores show: the lab free5GC once deployed, every free5GC
  // core network that is up (or has a gNB on it), every enabled core
  // outside the lab, and (only for an external gNB dialing none of those)
  // a plain "External core" node -----
  const enabled = lab.coreNetworks.networks.filter((n) => n.enabled && !isFree5gcCore(n))
  const shownCores = lab.free5gcCores.filter((c) => c.status !== 'stopped' || c.gnbs.length > 0)
  const ext = lab.externalGnbs
  // an external gNB's core: the one recorded on it, else the enabled one
  // whose AMF it dials
  const extCoreOf = (g: LabGnb) => {
    const n = g.coreNetwork ? enabled.find((c) => c.id === g.coreNetwork) : enabled.find((c) => c.n2Ip && c.n2Ip === g.deployment?.amfIp)
    return n ? `core:${n.id}` : ID.externalCore
  }
  const coreOfGnb = (g: LabGnb) => {
    if (g.coreNetwork && shownCores.some((c) => c.id === g.coreNetwork)) return `core:${g.coreNetwork}`
    if (g.core !== 'external') return ID.core
    return extCoreOf(g)
  }
  const orphans = ext.filter((g) => extCoreOf(g) === ID.externalCore)
  const showLab = lab.coreUp || lab.gnbs.some((g) => g.core === 'lab' && !g.coreNetwork) || (enabled.length === 0 && ext.length === 0 && shownCores.length === 0)
  const specs: OverviewCore[] = []
  const nfs = lab.free5gc.networkFunctions
  if (showLab) {
    specs.push({
      id: ID.core,
      node: { icon: 'core', label: 'free5GC', sub: lab.runningTemplate ? templateLabel(lab.runningTemplate) : 'not deployed', tag: nfs.length ? `${lab.healthyNfs} / ${nfs.length} NF healthy` : undefined, status: coreStatus },
      live: up(coreStatus),
      dashed: false,
      dns: ['internet'],
    })
  }
  for (const c of shownCores) {
    const cnfs = c.networkFunctions
    specs.push({
      id: `core:${c.id}`,
      node: { icon: 'core', label: c.name, sub: c.template ? templateLabel(c.template) : 'not deployed', tag: cnfs.length ? `${c.healthyNfs} / ${cnfs.length} NF healthy` : 'free5GC on this host', status: c.status },
      live: up(c.status),
      dashed: false,
      dns: c.network.dns.length ? c.network.dns.map((d) => d.name) : ['internet'],
    })
  }
  for (const n of enabled) {
    specs.push({ id: `core:${n.id}`, node: { icon: 'core', label: n.name, sub: `AMF ${n.n2Ip ?? ''}`, tag: 'outside the lab' }, live: true, dashed: false, dns: n.dns.map((d) => d.name) })
  }
  if (orphans.length) {
    const amfs = [...new Set(orphans.map((g) => g.deployment?.amfIp).filter(Boolean))]
    specs.push({ id: ID.externalCore, node: { icon: 'core', label: 'External core', sub: `AMF ${amfs.join(', ')}`, tag: 'not in Core Network' }, live: false, dashed: true, dns: [] })
  }

  const laneOf = (g: LabGnb) => (specs.some((c) => c.id === coreOfGnb(g)) ? coreOfGnb(g) : specs[0].id)
  return { specs, laneOf }
}

export function buildOverviewGraph(lab: Lab, ueLive: Record<string, UeLive> = {}): Graph {
  const nodes: CanvasNode[] = []
  const links: CanvasLink[] = []
  const upUes = lab.upUes

  const { specs, laneOf } = overviewCores(lab)

  // ----- lanes: every core is an invisible row, left to right its UEs,
  // its gNBs, the core, and its own data networks; each UE sits level with
  // its gNB, so no link crosses another row -----
  const gnbX = UE.w + RADIO_GAP
  const coreX = gnbX + BIG.w + 130
  const dnX = coreX + BIG.w + 110
  const STEP = UE.h + UE_GAP
  const SLOT_GAP = 30
  const LANE_GAP = 60
  const DNB = { w: 150, h: 56 }
  const DN_STEP = DNB.h + 20

  const hidden = Math.max(0, upUes.length - MAX_OVERVIEW_UES)
  const shownUes = hidden ? upUes.slice(0, MAX_OVERVIEW_UES - 1) : upUes
  const lanes = specs.map((c) => ({ spec: c, gnbs: lab.gnbs.filter((g) => laneOf(g) === c.id) }))
  const firstGnb = lanes.find((l) => l.gnbs.length)?.gnbs[0]
  // each UE under its gNB (a UE whose gNB is not drawn joins the first one)
  const uesOf = (g: LabGnb) => shownUes.filter((r) => (lab.gnbs.some((x) => x.id === ueGnbId(lab, r)) ? ueGnbId(lab, r) === g.id : g === firstGnb))
  // the "+N more" node goes under the last gNB that has UEs
  const moreGnb = hidden ? [...lab.gnbs].reverse().find((g) => uesOf(g).length) ?? firstGnb : undefined
  const ueCount = (g: LabGnb) => uesOf(g).length + (g === moreGnb ? 1 : 0)
  // the lane's own data networks (a DN shared with another lane is placed
  // between them below)
  const dnKey = (n: string) => n.toLowerCase()
  const sharedDn = (key: string) => specs.filter((c) => c.dns.some((n) => dnKey(n) === key)).length > 1

  let y = 0
  type Placed = { id: string; box: Box; live: boolean; dashed: boolean; dns: string[]; mid: number }
  const placed: Placed[] = []
  const gnbBoxes = new Map<string, Box>()
  for (const lane of lanes) {
    const ownDns = lane.spec.dns.filter((n) => !sharedDn(dnKey(n)))
    // a gNB's slot is as tall as its UE column (at least a gNB)
    const slots = lane.gnbs.map((g) => Math.max(BIG.h, ueCount(g) * STEP - UE_GAP))
    const gnbsH = lane.gnbs.length ? slots.reduce((a, b) => a + b, 0) + SLOT_GAP * (lane.gnbs.length - 1) : BIG.h
    const laneH = Math.max(BIG.h, gnbsH, ownDns.length * DN_STEP - 20)
    const laneMid = y + laneH / 2

    // the core, level with the middle of its lane
    const coreBox = { x: coreX, y: laneMid - BIG.h / 2, ...BIG }
    nodes.push({ id: lane.spec.id, ...coreBox, ...lane.spec.node })
    placed.push({ id: lane.spec.id, box: coreBox, live: lane.spec.live, dashed: lane.spec.dashed, dns: lane.spec.dns, mid: laneMid })

    // its gNBs, each centred in its slot, its UEs level with it
    let gy = laneMid - gnbsH / 2
    lane.gnbs.forEach((g, i) => {
      const slotMid = gy + slots[i] / 2
      const box = { x: gnbX, y: slotMid - BIG.h / 2, ...BIG }
      gnbBoxes.set(g.id, box)
      const ues = uesOf(g)
      const attached = upUes.filter((r) => ueGnbId(lab, r) === g.id && ueLive[r.ueId]?.stage !== 'failed').length
      nodes.push({ id: g.id, ...box, icon: 'gnb', label: g.profileName, sub: gnbSub(g, lab), tag: `${attached} UE connected`, status: g.status })
      const count = ueCount(g)
      const top = slotMid - (count * STEP - UE_GAP) / 2
      ues.forEach((row, j) => {
        const ub = { x: 0, y: top + j * STEP, ...UE }
        const id = ID.ue(row.ueId)
        const live = ueLive[row.ueId]
        const tag = live?.stage ? [STAGE_LABEL[live.stage], live.stage !== 'failed' ? live.ip : undefined].filter(Boolean).join(' · ') : undefined
        nodes.push({ id, ...ub, icon: 'ue', label: row.ueId.replace(/^imsi-/, ''), status: live?.stage === 'failed' && row.status === 'running' ? 'unhealthy' : row.status, tag, compact: true })
        links.push({ id: `uu:${row.ueId}`, from: id, to: g.id, d: hCurve(ub, box), live: up(row.status) && up(g.status), radio: true, label: mid(ub, box, true, 'Uu') })
      })
      if (g === moreGnb) {
        nodes.push({ id: ID.fleet, x: 0, y: top + ues.length * STEP, ...UE, icon: 'fleet', label: `+${hidden + 1} more UEs`, tag: 'see free-ue', compact: true })
      }
      gy += slots[i] + SLOT_GAP
    })
    // no gNB yet: a faded placeholder in the first lane
    if (lane === lanes[0] && lab.gnbs.length === 0) {
      const box = { x: gnbX, y: laneMid - BIG.h / 2, ...BIG }
      gnbBoxes.set(ID.gnbNone, box)
      nodes.push({ id: ID.gnbNone, ...box, icon: 'gnb', label: 'gNB', sub: 'not deployed', tag: '0 UE connected', status: 'stopped', ghost: true })
    }
    y += laneH + LANE_GAP
  }
  if (upUes.length === 0) {
    const mid0 = placed[0].mid
    nodes.push({ id: ID.fleet, x: UE.w - BIG.w, y: mid0 - BIG.h / 2, ...BIG, icon: 'fleet', label: 'No UE connected', sub: 'launch UEs on free-ue', ghost: true })
  }

  // gNB to core, within the lane
  for (const g of lab.gnbs) {
    const c = placed.find((x) => x.id === laneOf(g)) ?? placed[0]
    const box = gnbBoxes.get(g.id)!
    links.push({ id: `n2n3:${g.id}`, from: g.id, to: c.id, d: hCurve(box, c.box), live: up(g.status) && c.live, dashed: c.dashed, label: mid(box, c.box, true, 'N2 · N3') })
  }

  // ----- data networks: one node per DN name. A lane's own DNs stay in its
  // row; a DN shared by several cores sits level with the middle of them.
  // Then they are pushed apart top to bottom so none overlap -----
  const keys = [...new Set(placed.flatMap((c) => c.dns.map(dnKey)))]
  const wants = keys.map((key) => {
    const on = placed.filter((c) => c.dns.some((n) => dnKey(n) === key))
    let target: number
    if (on.length > 1) target = on.reduce((t, c) => t + c.mid, 0) / on.length
    else {
      const own = on[0].dns.filter((n) => !sharedDn(dnKey(n))).map(dnKey)
      const j = own.indexOf(key)
      target = on[0].mid + (j - (own.length - 1) / 2) * DN_STEP
    }
    const label = on[0].dns.find((n) => dnKey(n) === key) ?? key
    return { key, label, on, target }
  }).sort((a, b) => a.target - b.target)
  let floor = -Infinity
  for (const w of wants) {
    const top = Math.max(w.target - DNB.h / 2, floor)
    floor = top + DN_STEP
    const box = { x: dnX, y: top, ...DNB }
    const id = `${ID.dn}:${w.key}`
    nodes.push({ id, ...box, icon: 'cloud', label: w.label, sub: w.on.length > 1 ? `DN · ${w.on.length} cores` : 'DN' })
    for (const c of w.on) {
      links.push({ id: `n6:${c.id}:${w.key}`, from: c.id, to: id, d: hCurve(c.box, box), live: c.live, label: mid(c.box, box, true, 'N6') })
    }
  }

  return { nodes, links, frames: [], notes: [] }
}

// one core as the Core Network pages draw it: the core with its N2 (AMF)
// and N3 (UPF) lines leaving its left edge, labelled with their addresses,
// the gNBs attached at their far ends, and each data network on the right
export interface CoreView {
  id: string
  name: string
  sub?: string
  status?: NodeStatus
  n2Ip: string
  n2Interface?: string
  n3Ip: string
  n3Interface?: string
  dns: { name: string }[]
  // the running gNBs attached to this core
  gnbs: { id: string; name: string; status: NodeStatus }[]
}

// buildCoreGraph draws one core with its N2 / N3 / DN links.
export function buildCoreGraph(core: CoreView): Graph {
  const nodes: CanvasNode[] = []
  const links: CanvasLink[] = []
  const up = core.status === undefined || core.status === 'running'
  const DNB = { w: 180, h: 56 }
  const STUB = 240
  const GAP = 18

  const coreBox = { x: 640, y: 0, ...BIG }
  nodes.push({ id: core.id, ...coreBox, icon: 'core', label: core.name, sub: core.sub, status: core.status })
  const coreMid = cy(coreBox)

  // the N2 and N3 lines leave the core's left edge, one above the other,
  // and meet the gNBs at a junction STUB to the left
  const x0 = coreBox.x
  const jx = x0 - STUB
  const y2 = coreMid - GAP
  const y3 = coreMid + GAP
  const tag = (kind: string, ip: string, ifc?: string) => [kind, ip, ifc].filter(Boolean).join(' · ')
  links.push({ id: 'n2', from: `${core.id}:n2`, to: core.id, d: `M${jx} ${y2}H${x0}`, live: up && core.gnbs.length > 0, label: { x: (jx + x0) / 2, y: y2 - 7, text: tag('N2', core.n2Ip, core.n2Interface) } })
  links.push({ id: 'n3', from: `${core.id}:n3`, to: core.id, d: `M${jx} ${y3}H${x0}`, live: up && core.gnbs.length > 0, label: { x: (jx + x0) / 2, y: y3 + 15, text: tag('N3', core.n3Ip, core.n3Interface) } })

  // the gNBs attached, at the lines' far end
  const gx = jx - 110 - BIG.w
  const gStep = BIG.h + 30
  const gTop = coreMid - (Math.max(1, core.gnbs.length) * gStep - 30) / 2
  const curve = (x1: number, ya: number, x2: number, yb: number) => {
    const dx = Math.max(30, (x2 - x1) / 2)
    return `M${x1} ${ya}C${x1 + dx} ${ya} ${x2 - dx} ${yb} ${x2} ${yb}`
  }
  core.gnbs.forEach((g, i) => {
    const box = { x: gx, y: gTop + i * gStep, ...BIG }
    nodes.push({ id: g.id, ...box, icon: 'gnb', label: g.name, status: g.status })
    const live = up && g.status === 'running'
    const right = box.x + box.w
    links.push({ id: `${g.id}:n2`, from: g.id, to: core.id, d: curve(right, cy(box) - 10, jx, y2), live })
    links.push({ id: `${g.id}:n3`, from: g.id, to: core.id, d: curve(right, cy(box) + 10, jx, y3), live })
  })
  if (core.gnbs.length === 0) {
    nodes.push({ id: `${core.id}:nognb`, x: gx, y: coreMid - BIG.h / 2, ...BIG, icon: 'gnb', label: 'No gNB attached', sub: 'deploy one on free-ran', ghost: true })
  }

  // data networks, right of the core
  const dStep = DNB.h + 24
  const dTop = coreMid - (core.dns.length * dStep - 24) / 2
  core.dns.forEach((dn, i) => {
    const box = { x: coreBox.x + BIG.w + 150, y: dTop + i * dStep, ...DNB }
    nodes.push({ id: `${core.id}:dn:${dn.name}`, ...box, icon: 'cloud', label: dn.name, sub: 'DN' })
    links.push({ id: `n6:${dn.name}`, from: core.id, to: `${core.id}:dn:${dn.name}`, d: hCurve(coreBox, box), live: up, label: i === 0 ? mid(coreBox, box, true, 'N6') : undefined })
  })

  return { nodes, links, frames: [], notes: [] }
}
