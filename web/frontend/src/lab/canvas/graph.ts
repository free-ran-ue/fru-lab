import type { Free5gcTemplate } from '../free5gcTemplate'
import type { NodeStatus } from '../types'
import type { Lab, LabGnb } from '../useLab'
import { templateLabel } from '../useLab'
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

const gnbSub = (g: LabGnb) => (g.core === 'external' ? `external · AMF ${g.deployment?.amfIp ?? ''}` : 'lab free5GC')

// buildOverviewGraph is the overview map: every connected UE on the radio
// to its gNB, every running gNB, the core(s) as one node each and the data
// network.
export function buildOverviewGraph(lab: Lab, ueLive: Record<string, UeLive> = {}): Graph {
  const nodes: CanvasNode[] = []
  const links: CanvasLink[] = []
  const coreStatus = lab.nodes.core.status

  // ----- gNBs: one node per running gNB -----
  const gnbX = UE.w + RADIO_GAP
  const upUes = lab.upUes
  const gnbStep = BIG.h + 60
  const shownGnbs = lab.gnbs.length ? lab.gnbs : [null]
  const gnbs = shownGnbs.map((g, i) => {
    const box = { x: gnbX, y: i * gnbStep, ...BIG }
    if (!g) {
      nodes.push({ id: ID.gnbNone, ...box, icon: 'gnb', label: 'gNB', sub: 'not deployed', tag: '0 UE connected', status: 'stopped', ghost: true })
      return { id: ID.gnbNone, box, status: 'stopped' as NodeStatus, core: 'lab' as const }
    }
    const attached = upUes.filter((r) => ueGnbId(lab, r) === g.id && ueLive[r.ueId]?.stage !== 'failed').length
    nodes.push({ id: g.id, ...box, icon: 'gnb', label: g.profileName, sub: gnbSub(g), tag: `${attached} UE connected`, status: g.status })
    return { id: g.id, box, status: g.status, core: g.core }
  })
  const gnbTop = gnbs[0].box.y
  const gnbBottom = gnbs[gnbs.length - 1].box.y + BIG.h
  const midY = (gnbTop + gnbBottom) / 2

  // ----- UEs: one node per connected UE, on the radio to its gNB -----
  const hidden = Math.max(0, upUes.length - MAX_OVERVIEW_UES)
  const shown = hidden ? upUes.slice(0, MAX_OVERVIEW_UES - 1) : upUes
  const drawn = shown.length + (hidden ? 1 : 0)
  const step = UE.h + UE_GAP
  const top = midY - (Math.max(1, drawn) * step - UE_GAP) / 2
  shown.forEach((row, i) => {
    const box = { x: 0, y: top + i * step, ...UE }
    const id = ID.ue(row.ueId)
    const live = ueLive[row.ueId]
    const tag = live?.stage ? [STAGE_LABEL[live.stage], live.stage !== 'failed' ? live.ip : undefined].filter(Boolean).join(' · ') : undefined
    nodes.push({ id, ...box, icon: 'ue', label: row.ueId.replace(/^imsi-/, ''), status: live?.stage === 'failed' && row.status === 'running' ? 'unhealthy' : row.status, tag, compact: true })
    const g = gnbs.find((x) => x.id === ueGnbId(lab, row)) ?? gnbs[0]
    links.push({ id: `uu:${row.ueId}`, from: id, to: g.id, d: hCurve(box, g.box), live: up(row.status) && up(g.status), radio: true })
  })
  if (hidden) {
    const box = { x: 0, y: top + shown.length * step, ...UE }
    nodes.push({ id: ID.fleet, ...box, icon: 'fleet', label: `+${hidden + 1} more UEs`, tag: 'see free-ue', compact: true })
  }
  if (drawn === 0) {
    const box = { x: UE.w - BIG.w, y: midY - BIG.h / 2, ...BIG }
    nodes.push({ id: ID.fleet, ...box, icon: 'fleet', label: 'No UE connected', sub: 'launch UEs on free-ue', ghost: true })
  }

  // ----- cores: the lab free5GC and/or the external core the external
  // gNBs dial, one node each -----
  const ext = lab.externalGnbs
  const showLab = lab.coreUp || gnbs.some((g) => g.core === 'lab') || ext.length === 0
  const coreX = gnbX + BIG.w + 130
  const cores: { id: string; box: Box; external: boolean }[] = []
  const coreCount = (showLab ? 1 : 0) + (ext.length ? 1 : 0)
  const coreY = (i: number) => midY - BIG.h / 2 + (i - (coreCount - 1) / 2) * (BIG.h + 40)
  const nfs = lab.free5gc.networkFunctions
  if (showLab) {
    const box = { x: coreX, y: coreY(0), ...BIG }
    nodes.push({
      id: ID.core,
      ...box,
      icon: 'core',
      label: 'free5GC core',
      sub: lab.runningTemplate ? templateLabel(lab.runningTemplate) : 'not deployed',
      tag: nfs.length ? `${lab.healthyNfs} / ${nfs.length} NF healthy` : undefined,
      status: coreStatus,
    })
    cores.push({ id: ID.core, box, external: false })
  }
  if (ext.length) {
    const box = { x: coreX, y: coreY(showLab ? 1 : 0), ...BIG }
    const amfs = [...new Set(ext.map((g) => g.deployment?.amfIp).filter(Boolean))]
    nodes.push({ id: ID.externalCore, ...box, icon: 'core', label: 'External core', sub: `AMF ${amfs.join(', ')}`, tag: 'outside the lab' })
    cores.push({ id: ID.externalCore, box, external: true })
  }
  let labelled = false
  for (const g of gnbs) {
    const c = cores.find((x) => x.external === (g.core === 'external')) ?? cores[0]
    links.push({ id: `n2n3:${g.id}`, from: g.id, to: c.id, d: hCurve(g.box, c.box), live: up(g.status) && (c.external || up(coreStatus)), dashed: c.external, label: labelled ? undefined : mid(g.box, c.box, true, 'N2 · N3') })
    labelled = true
  }

  // ----- data network -----
  const dn = { x: coreX + BIG.w + 90, y: midY - 36, w: 150, h: 72 }
  nodes.push({ id: ID.dn, ...dn, icon: 'cloud', label: 'DN', sub: 'internet' })
  for (const [i, c] of cores.entries()) {
    links.push({ id: `n6:${c.id}`, from: c.id, to: ID.dn, d: hCurve(c.box, dn), live: c.external ? false : up(coreStatus), dashed: c.external, label: i === 0 ? mid(c.box, dn, true, 'N6') : undefined })
  }

  return { nodes, links, frames: [], notes: [] }
}
