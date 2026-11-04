import type { KeyboardEvent, ReactNode } from 'react'
import type { Free5gcTemplate } from './free5gcTemplate'
import type { NetworkFunction, NodeId, NodeStatus } from './types'
import type { UeRow } from './useUeInstances'
import styles from './topology-schematic.module.css'

export interface TopologySelection {
  node: NodeId
  // a single NF inside the core frame, or a single UE instance
  nf: string | null
  ue: string | null
}

export interface GnbSlice {
  name: string
  status: NodeStatus
  // UE instances currently up on this slice (by the subscriber's sd)
  ueCount: number
}

interface TopologySchematicProps {
  template: Free5gcTemplate
  templateLabel: string
  coreStatus: NodeStatus
  coreNfs: NetworkFunction[]
  gnbStatus: NodeStatus
  gnbTemplate: string
  // ulcl-2slice only: the two slice gNBs the one gNB node stands for
  gnbSlices: GnbSlice[] | null
  ueStatus: NodeStatus
  ueRows: UeRow[]
  selection: TopologySelection
  onSelect: (selection: TopologySelection) => void
}

// The v2 dashboard's one engineered schematic: UE → Uu → gNB → N2 / N3 →
// the free5GC core frame (SBI bus with the control-plane NFs, AMF, SMF(s)
// and the UPF chain per template) → N6 → DN. Geometry is in viewBox units
// and follows LabDashboard.dc.html; ulcl-2slice adds a second SMF and a
// second UPF lane below the first.

// the SBI control-plane NFs drawn on the top row; any other live service
// that isn't one of the plane NFs below is appended after them.
const TOP_NFS = ['NRF', 'AUSF', 'UDM', 'UDR', 'PCF', 'NSSF', 'CHF']
const PLANE_NFS = new Set(['AMF', 'SMF', 'SMF1', 'SMF2', 'UPF', 'I-UPF', 'PSA-UPF', 'I-UPF-1', 'PSA-UPF-1', 'I-UPF-2', 'PSA-UPF-2'])

const NF_H = 26
const CORE_X = 450
const CORE_Y = 20
const X0 = 466
const TOP_Y = 56
const BUS_Y = 104
const ROW2_Y = 130
const UE_BOX: Box = { x: 20, y: 150, w: 140, h: 84 }
const UU_Y = 186
const GNB_X = 240
const GNB_Y = 128
const GNB_W = 140
const N2_Y = ROW2_Y + NF_H / 2
const MAX_UE_SWATCHES = 11

interface Box { x: number; y: number; w: number; h: number }
interface Label { x: number; y: number; text: string; anchor?: 'middle' | 'end' }
interface Link { d: string; ext?: boolean }

const cx = (b: Box) => b.x + b.w / 2
const cy = (b: Box) => b.y + b.h / 2
const right = (b: Box) => b.x + b.w
const bottom = (b: Box) => b.y + b.h

// mono 10.5 is ~6.3 units per glyph; 18 units for the swatch + left pad
function nfWidth(name: string, min = 48): number {
  return Math.max(min, Math.ceil(24 + name.length * 6.3))
}

function buildLayout(template: Free5gcTemplate, liveNames: string[]) {
  const nfs: { name: string; box: Box }[] = []
  const links: Link[] = []
  const labels: Label[] = []
  const lanes: { y: number; first: Box; last: Box }[] = []
  const twoSlice = template === 'ulcl-2slice'

  const extras = liveNames.filter((name) => !TOP_NFS.includes(name) && !PLANE_NFS.has(name))
  let x = X0
  for (const name of [...TOP_NFS, ...extras]) {
    const box = { x, y: TOP_Y, w: nfWidth(name), h: NF_H }
    nfs.push({ name, box })
    links.push({ d: `M${cx(box)} ${bottom(box)}V${BUS_Y}` })
    x += box.w + 8
  }
  const topEnd = x - 8

  const amf: Box = { x: X0, y: ROW2_Y, w: 60, h: NF_H }
  nfs.push({ name: 'AMF', box: amf })
  links.push({ d: `M${cx(amf)} ${BUS_Y}V${amf.y}` })

  let minRight: number
  if (!twoSlice) {
    const smf: Box = { x: 590, y: ROW2_Y, w: 60, h: NF_H }
    const first: Box = { x: 590, y: 210, w: 60, h: NF_H }
    nfs.push({ name: 'SMF', box: smf }, { name: template === 'ulcl' ? 'I-UPF' : 'UPF', box: first })
    links.push({ d: `M${cx(smf)} ${BUS_Y}V${smf.y}` })
    links.push({ d: `M${cx(smf)} ${bottom(smf)}V${first.y}` })
    labels.push({ x: cx(smf) + 6, y: 187, text: 'N4' })
    let last = first
    if (template === 'ulcl') {
      const psa: Box = { x: 710, y: 210, w: 72, h: NF_H }
      nfs.push({ name: 'PSA-UPF', box: psa })
      links.push({ d: `M${right(smf)} ${cy(smf)}H${cx(psa)}V${psa.y}` })
      labels.push({ x: cx(psa) + 6, y: 187, text: 'N4' })
      links.push({ d: `M${right(first)} ${cy(first)}H${psa.x}` })
      labels.push({ x: (right(first) + psa.x) / 2, y: cy(first) - 6, text: 'N9', anchor: 'middle' })
      last = psa
    }
    lanes.push({ y: cy(first), first, last })
    minRight = 870
  } else {
    const smf1: Box = { x: 590, y: ROW2_Y, w: 60, h: NF_H }
    const smf2: Box = { x: 700, y: ROW2_Y, w: 60, h: NF_H }
    const iw = nfWidth('I-UPF-1', 60)
    const pw = nfWidth('PSA-UPF-1', 60)
    const iUpf1: Box = { x: cx(smf1) - iw / 2, y: 196, w: iw, h: NF_H }
    const psa1: Box = { x: 760, y: 196, w: pw, h: NF_H }
    const iUpf2: Box = { x: cx(smf2) - iw / 2, y: 250, w: iw, h: NF_H }
    const psa2: Box = { x: 850, y: 250, w: pw, h: NF_H }
    nfs.push(
      { name: 'SMF1', box: smf1 }, { name: 'SMF2', box: smf2 },
      { name: 'I-UPF-1', box: iUpf1 }, { name: 'PSA-UPF-1', box: psa1 },
      { name: 'I-UPF-2', box: iUpf2 }, { name: 'PSA-UPF-2', box: psa2 },
    )
    links.push({ d: `M${cx(smf1)} ${BUS_Y}V${smf1.y}` }, { d: `M${cx(smf2)} ${BUS_Y}V${smf2.y}` })
    // slice 1: SMF1 drops straight to I-UPF-1 and elbows over to PSA-UPF-1
    links.push({ d: `M${cx(smf1)} ${bottom(smf1)}V${iUpf1.y}` })
    links.push({ d: `M${cx(smf1) + 16} ${bottom(smf1)}V176H${cx(psa1)}V${psa1.y}` })
    labels.push({ x: cx(smf1) + 6, y: 190, text: 'N4' }, { x: cx(psa1) + 6, y: 190, text: 'N4' })
    // slice 2: same shape one lane down, passing behind lane 1's N9
    links.push({ d: `M${cx(smf2)} ${bottom(smf2)}V${iUpf2.y}` })
    links.push({ d: `M${cx(smf2) + 16} ${bottom(smf2)}V236H${cx(psa2)}V${psa2.y}` })
    labels.push({ x: cx(smf2) - 6, y: 245, text: 'N4', anchor: 'end' }, { x: cx(psa2) + 6, y: 246, text: 'N4' })
    for (const [first, last] of [[iUpf1, psa1], [iUpf2, psa2]] as const) {
      links.push({ d: `M${right(first)} ${cy(first)}H${last.x}` })
      labels.push({ x: (right(first) + last.x) / 2, y: cy(first) - 6, text: 'N9', anchor: 'middle' })
      lanes.push({ y: cy(first), first, last })
    }
    minRight = Math.ceil(right(psa2) + 16)
  }

  const coreRight = Math.max(minRight, topEnd + 20)
  const coreBottom = twoSlice ? 290 : 260
  const firstLane = lanes[0]
  const lastLane = lanes[lanes.length - 1]
  const dn: Box = {
    x: coreRight + 25,
    y: firstLane.first.y,
    w: 50,
    h: bottom(lastLane.first) - firstLane.first.y,
  }

  for (const lane of lanes) {
    // N3 from the gNB's right edge into the lane's first UPF
    links.push({ d: `M${GNB_X + GNB_W} ${lane.y}H${lane.first.x}` })
    labels.push({ x: (GNB_X + GNB_W + X0) / 2, y: lane.y - 6, text: 'N3', anchor: 'middle' })
    // N6 out of the core to the data network
    links.push({ d: `M${right(lane.last)} ${lane.y}H${dn.x}`, ext: true })
    labels.push({ x: (right(lane.last) + dn.x) / 2, y: lane.y - 6, text: 'N6', anchor: 'middle' })
  }

  return {
    nfs,
    links,
    labels,
    lanes,
    coreRight,
    coreBottom,
    dn,
    gnbH: twoSlice ? bottom(lastLane.first) - GNB_Y : 108,
    width: dn.x + dn.w + 15,
    height: coreBottom + 16,
  }
}

const SWATCH: Record<NodeStatus, string> = {
  running: styles.swRunning,
  deploying: styles.swDeploying,
  unhealthy: styles.swUnhealthy,
  stopped: styles.swStopped,
}

function Swatch({ x, y, status, title }: { x: number; y: number; status: NodeStatus; title?: string }) {
  return (
    <circle cx={x} cy={y} r={3} className={SWATCH[status]}>
      {title && <title>{title}</title>}
    </circle>
  )
}

// a focusable, clickable group (Enter / Space activate it like a button)
function Pressable({ label, pressed, onPress, children }: { label: string; pressed: boolean; onPress: () => void; children: ReactNode }) {
  function handleKey(event: KeyboardEvent<SVGGElement>) {
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault()
      onPress()
    }
  }
  return (
    <g role="button" tabIndex={0} aria-label={label} aria-pressed={pressed} className={styles.press} onClick={onPress} onKeyDown={handleKey}>
      {children}
    </g>
  )
}

export default function TopologySchematic({ template, templateLabel, coreStatus, coreNfs, gnbStatus, gnbTemplate, gnbSlices, ueStatus, ueRows, selection, onSelect }: TopologySchematicProps) {
  const nfStatus = new Map(coreNfs.map((nf) => [nf.name, nf.status]))
  const layout = buildLayout(template, coreNfs.map((nf) => nf.name))
  const healthy = coreNfs.filter((nf) => nf.status === 'running').length
  const coreOn = selection.node === 'core'

  const ueRunning = ueRows.filter((row) => row.status === 'running').length
  const shownUes = ueRows.length > MAX_UE_SWATCHES ? ueRows.slice(0, MAX_UE_SWATCHES - 1) : ueRows
  const hiddenUes = ueRows.length - shownUes.length
  const gnbBox: Box = { x: GNB_X, y: GNB_Y, w: GNB_W, h: layout.gnbH }

  const selectCore = () => onSelect({ node: 'core', nf: null, ue: null })

  return (
    <div className={styles.wrap}>
      <svg
        viewBox={`0 0 ${layout.width} ${layout.height}`}
        className={styles.svg}
        role="group"
        aria-label="UE connects over Uu to the gNB; the gNB connects over N2 to the AMF and over N3 to the UPF inside the free5GC core; SMF controls the UPFs over N4; the last UPF reaches the data network over N6. Select a node for details."
      >
        {/* core frame first, so its fill sits behind every link and node;
            clicking anywhere on it that isn't an NF selects the core */}
        <Pressable label={`free5GC core, ${templateLabel}`} pressed={coreOn && selection.nf === null} onPress={selectCore}>
          <rect
            x={CORE_X + 0.5}
            y={CORE_Y + 0.5}
            width={layout.coreRight - CORE_X}
            height={layout.coreBottom - CORE_Y}
            rx={4}
            className={`${styles.frame} ${coreOn ? styles.frameOn : ''}`}
          />
          <text x={X0} y={41} fontSize={12} className={styles.title}>free5GC core</text>
          <text x={layout.coreRight - 28} y={41} fontSize={10.5} textAnchor="end" className={styles.monoMuted}>
            {templateLabel.toUpperCase()} · {coreNfs.length > 0 ? `${healthy} / ${coreNfs.length} NF` : 'not deployed'}
          </text>
          <Swatch x={layout.coreRight - 15} y={37} status={coreStatus} />
        </Pressable>

        <g className={styles.links}>
          <path d={`M160 ${UU_Y}H${GNB_X}`} />
          <path d={`M${GNB_X + GNB_W} ${N2_Y}H${X0}`} />
          <path d={`M${X0} ${BUS_Y}H${layout.coreRight - 20}`} />
          {layout.links.map((link) => (
            <path key={link.d} d={link.d} className={link.ext ? styles.ext : undefined} />
          ))}
        </g>

        <g fontSize={10.5} className={styles.iface}>
          <text x={200} y={UU_Y - 6} textAnchor="middle">Uu</text>
          <text x={(GNB_X + GNB_W + X0) / 2} y={N2_Y - 6} textAnchor="middle">N2</text>
          <text x={layout.coreRight - 20} y={BUS_Y + 14} textAnchor="end" className={styles.monoMuted}>SBI</text>
          {layout.labels.map((label) => (
            <text key={`${label.text}-${label.x}-${label.y}`} x={label.x} y={label.y} textAnchor={label.anchor}>{label.text}</text>
          ))}
        </g>

        {/* UE: the whole fleet as one node, one swatch per subscriber */}
        <Pressable label={`UE, ${ueRunning} of ${ueRows.length} running`} pressed={selection.node === 'ue'} onPress={() => onSelect({ node: 'ue', nf: null, ue: null })}>
          <rect x={UE_BOX.x + 0.5} y={UE_BOX.y + 0.5} width={UE_BOX.w} height={UE_BOX.h} rx={4} className={`${styles.node} ${selection.node === 'ue' ? styles.nodeOn : ''}`} />
          <text x={32} y={170} fontSize={12} className={styles.title}>UE</text>
          <Swatch x={145} y={166} status={ueStatus} />
          <text x={32} y={186} fontSize={11} className={styles.sub}>free-ran-ue</text>
          {ueRows.length === 0 ? (
            <text x={32} y={204} fontSize={10.5} className={styles.monoMuted}>no subscribers</text>
          ) : (
            <>
              {shownUes.map((row, index) => (
                <Swatch key={row.ueId} x={35 + index * 9} y={200} status={row.status} title={row.ueId} />
              ))}
              {hiddenUes > 0 && (
                <text x={32 + shownUes.length * 9 + 3} y={204} fontSize={10.5} className={styles.monoMuted}>+{hiddenUes}</text>
              )}
            </>
          )}
          <text x={32} y={222} fontSize={10.5} className={styles.mono}>{ueRunning} / {ueRows.length} up</text>
        </Pressable>

        {/* gNB: one node; under ulcl-2slice it stands for both slice gNBs */}
        <Pressable label={`gNB, ${gnbStatus}`} pressed={selection.node === 'gnb'} onPress={() => onSelect({ node: 'gnb', nf: null, ue: null })}>
          <rect x={gnbBox.x + 0.5} y={gnbBox.y + 0.5} width={gnbBox.w} height={gnbBox.h} rx={4} className={`${styles.node} ${selection.node === 'gnb' ? styles.nodeOn : ''}`} />
          <text x={252} y={148} fontSize={12} className={styles.title}>gNB</text>
          <Swatch x={365} y={144} status={gnbStatus} />
          <text x={252} y={164} fontSize={11} className={styles.sub}>free-ran-ue</text>
          <text x={252} y={179} fontSize={11} className={styles.sub}>{gnbTemplate}</text>
          {gnbSlices && gnbSlices.map((slice, index) => {
            const y = layout.lanes[index]?.y ?? 200 + index * 20
            return (
              <g key={slice.name}>
                <Swatch x={255} y={y} status={slice.status} />
                <text x={263} y={y + 4} fontSize={10.5} className={styles.mono}>{slice.name} · {slice.ueCount} UE</text>
              </g>
            )
          })}
        </Pressable>

        {layout.nfs.map(({ name, box }) => {
          const status = nfStatus.get(name)
          const on = coreOn && selection.nf === name
          return (
            <Pressable
              key={name}
              label={`${name}, ${status ?? 'not deployed'}`}
              pressed={on}
              // an NF that isn't live has nothing of its own to show - select the core
              onPress={() => (status ? onSelect({ node: 'core', nf: name, ue: null }) : selectCore())}
            >
              <rect x={box.x + 0.5} y={box.y + 0.5} width={box.w} height={box.h} rx={3} className={`${styles.nf} ${on ? styles.nfOn : ''}`} />
              <Swatch x={box.x + 11} y={cy(box)} status={status ?? 'stopped'} />
              <text x={box.x + 18} y={cy(box) + 4} fontSize={10.5} className={status ? styles.mono : styles.monoMuted}>{name}</text>
            </Pressable>
          )
        })}

        <rect x={layout.dn.x + 0.5} y={layout.dn.y + 0.5} width={layout.dn.w} height={layout.dn.h} rx={3} className={styles.dn} />
        <text x={cx(layout.dn)} y={cy(layout.dn) + 4} fontSize={10.5} textAnchor="middle" className={styles.monoMuted}>DN</text>
      </svg>
    </div>
  )
}
