import { getStatusMeta } from './statusMeta'
import { detectFree5gcTemplate } from './free5gcTemplate'
import type { NetworkFunction, NodeStatus } from './types'
import type { UeRow } from './useUeInstances'
import styles from './detailed-topology.module.css'

interface DetailedTopologyProps {
  free5gcNfs: NetworkFunction[]
  // the legacy singleton gNB (basic/ulcl); ignored under ulcl-2slice, where
  // gnbSlice1Status/gnbSlice2Status are the real, independently deployable
  // gNBs instead.
  gnbStatus: NodeStatus
  gnbSlice1Status: NodeStatus
  gnbSlice2Status: NodeStatus
  ueRows: UeRow[]
}

// The 9 SBI-connected control-plane NFs, split across the two rows that
// straddle the SBI bus line. The data-plane UPF(s) are deliberately
// excluded - they're controlled over N4 (PFCP), not part of the SBI mesh
// the others talk over, and rendered separately in row 3 below. SMF/AMF sit
// in row 2 (closest to the bus/RAN side) so their N4/N2 drops stay short.
const ROW1_NFS = ['AUSF', 'UDM', 'UDR', 'NRF', 'NSSF']
const ROW2_NFS = ['PCF', 'AMF', 'SMF', 'CHF']
// ulcl-2slice runs one dedicated SMF per slice instead of one shared SMF -
// see smf1cfg.yaml's comment in the backend template for why.
const ROW2_NFS_2SLICE = ['PCF', 'AMF', 'SMF1', 'SMF2', 'CHF']

// mirrors backend's constant.SLICE1_SD/SLICE2_SD - used to pick which gNB
// slice's box a UE's Uu line is drawn from (see the render loop below).
const SLICE1_SD = '010203'
const SLICE2_SD = '112233'

const VIEWBOX_WIDTH = 980
const NF_MARGIN_X = 40
const NF_BOX = { width: 74, height: 32 }

const ROW1_Y = 20
const BUS_Y = 92
const ROW2_Y = 128
const ROW3_Y = 216
const ROW3_H = 40
const ROW3_GAP = 90
const UE_ROW_SINGLE_Y = 320
const UE_BOX = { width: 74, height: 32 }

// ulcl-2slice threads slice 2's chain through row 4 as row 3's chain
// shifted right by a fixed amount (see ROW4_SHIFT below) - same internal
// shape (gNB - N3 - I-UPF - N9 - PSA-UPF - N6 - DN, plus SMF's two N4
// drops), just offset, so row 4's own links stay exactly as clean as row
// 3's, and only the links reaching up to row 2 (N2, N4) end up diagonal.
const ROW4_ROW_GAP = 64
const ROW4_Y = ROW3_Y + ROW4_ROW_GAP
const UE_ROW_TWO_SLICE_Y = ROW4_Y + ROW4_ROW_GAP
// every two-slice Uu line's horizontal jog happens here, just below row 4's
// bottom edge - a slice-1 UE's line starts up at gNB Slice 1 (row 3) and has
// to cross row 4's y-band to reach the UE row below it, so jogging at the
// midpoint (like ElbowDropLink does elsewhere) would cut straight through
// whichever row-4 box happens to sit under that UE's column. Below row 4
// entirely, the band is clear regardless of x, for every source and target.
const UU_WAYPOINT_Y_TWO_SLICE = ROW4_Y + ROW3_H + 8

function rowBoxX(index: number, count: number): number {
  const usableWidth = VIEWBOX_WIDTH - NF_MARGIN_X * 2 - NF_BOX.width
  const step = count > 1 ? usableWidth / (count - 1) : 0
  return NF_MARGIN_X + step * index
}

function rowBoxCenterX(index: number, count: number): number {
  return rowBoxX(index, count) + NF_BOX.width / 2
}

type Box = { x: number; y: number; width: number; height: number }

function boxCenterX(box: Box): number {
  return box.x + box.width / 2
}

// translates a row-3 box down to row 4 by a fixed horizontal shift.
function shiftToRow4(box: Box, shiftX: number): Box {
  return { x: box.x + shiftX, y: ROW4_Y, width: box.width, height: box.height }
}

function statusColors(status: NodeStatus) {
  const meta = getStatusMeta(status)
  return { fill: meta.bg, stroke: meta.border, text: meta.color, dot: meta.dot }
}

function NodeBox({ x, y, width, height, label, status, title }: { x: number; y: number; width: number; height: number; label: string; status: NodeStatus; title?: string }) {
  const colors = statusColors(status)
  return (
    <g>
      {title && <title>{title}</title>}
      <rect x={x} y={y} width={width} height={height} rx={7} fill={colors.fill} stroke={colors.stroke} strokeWidth={1.5} />
      <circle cx={x + 10} cy={y + height / 2} r={3.5} fill={colors.dot} />
      <text x={x + width / 2 + 4} y={y + height / 2} dominantBaseline="middle" textAnchor="middle" className={styles.nodeLabel} fill={colors.text}>
        {label}
      </text>
    </g>
  )
}

function InterfaceLabel({ x, y, text }: { x: number; y: number; text: string }) {
  return (
    <text x={x} y={y} textAnchor="middle" className={styles.ifaceLabel}>
      {text}
    </text>
  )
}

// N2 (AMF->gNB) and N4 (SMF->I-UPF) drop from the parent's bottom-center to
// the child's top-center. Every DropLink usage has the child's box centered
// directly under the parent's, so this stays a straight vertical - never
// diagonal. Anything not x-aligned (row 4's shifted nodes, SMF's second N4)
// uses ElbowDropLink instead.
function DropLink({ from, to, label, labelY }: { from: Box; to: Box; label: string; labelY: number }) {
  const x1 = boxCenterX(from)
  const y1 = from.y + from.height
  const x2 = boxCenterX(to)
  const y2 = to.y
  return (
    <>
      <line x1={x1} y1={y1} x2={x2} y2={y2} className={styles.connectorLine} />
      <InterfaceLabel x={(x1 + x2) / 2 + 14} y={labelY} text={label} />
    </>
  )
}

function elbowPath(x1: number, y1: number, x2: number, y2: number, waypointY: number): string {
  return `M ${x1} ${y1} L ${x1} ${waypointY} L ${x2} ${waypointY} L ${x2} ${y2}`
}

// For any link whose parent/child aren't x-aligned (SMF's second N4 to
// PSA-UPF; N2 to row 4's shifted gNB), routes down-across-down instead of a
// raw diagonal - vertical out of the parent, horizontal at waypointY, then
// vertical into the child. Keeps every segment horizontal or vertical, and
// the horizontal jog stays in the empty band above row 3, clear of boxes.
function ElbowDropLink({ from, to, label, waypointY }: { from: Box; to: Box; label: string; waypointY: number }) {
  const x1 = boxCenterX(from)
  const y1 = from.y + from.height
  const x2 = boxCenterX(to)
  const y2 = to.y
  return (
    <>
      <path d={elbowPath(x1, y1, x2, y2, waypointY)} className={styles.connectorLine} fill="none" />
      <InterfaceLabel x={x2 + 18} y={waypointY - 6} text={label} />
    </>
  )
}

// N3 (gNB->I-UPF), N9 (I-UPF->PSA-UPF) and N6 (PSA-UPF->DN) sit side by
// side in the same row, right edge to left edge.
function RowLink({ from, to, label, labelY, dashed }: { from: Box; to: Box; label: string; labelY: number; dashed?: boolean }) {
  const x1 = from.x + from.width
  const y1 = from.y + from.height / 2
  const x2 = to.x
  const y2 = to.y + to.height / 2
  return (
    <>
      <line x1={x1} y1={y1} x2={x2} y2={y2} className={dashed ? styles.connectorLineDashed : styles.connectorLine} />
      <InterfaceLabel x={(x1 + x2) / 2} y={labelY} text={label} />
    </>
  )
}

export default function DetailedTopology({ free5gcNfs, gnbStatus, gnbSlice1Status, gnbSlice2Status, ueRows }: DetailedTopologyProps) {
  const nfStatusByName = new Map(free5gcNfs.map((nf) => [nf.name, nf.status]))

  const template = detectFree5gcTemplate(free5gcNfs)
  // ULCL splits the data plane into an intermediate UPF (N3-facing) and a
  // PDU session anchor UPF (N6-facing), chained by N9.
  const isUlcl = template === 'ulcl'
  const isTwoSlice = template === 'ulcl-2slice'

  const row2Nfs = isTwoSlice ? ROW2_NFS_2SLICE : ROW2_NFS
  const amfIndex = row2Nfs.indexOf('AMF')
  const amfCenterX = rowBoxCenterX(amfIndex, row2Nfs.length)
  const row2BottomY = ROW2_Y + NF_BOX.height
  const amfBox: Box = { x: amfCenterX - NF_BOX.width / 2, y: ROW2_Y, width: NF_BOX.width, height: NF_BOX.height }

  // Single-SMF layouts (basic/ulcl): one gNB, under AMF (short N2 drop),
  // with its N3 connecting directly into the one UPF chain under SMF.
  const gnbBox: Box = { x: amfCenterX - 50, y: ROW3_Y, width: 100, height: ROW3_H }

  const smfIndex = row2Nfs.indexOf('SMF')
  const smfCenterX = smfIndex >= 0 ? rowBoxCenterX(smfIndex, row2Nfs.length) : 0
  const smfBox: Box = { x: smfCenterX - NF_BOX.width / 2, y: ROW2_Y, width: NF_BOX.width, height: NF_BOX.height }
  const iUpfBox: Box = { x: smfCenterX - 42, y: ROW3_Y, width: 84, height: ROW3_H }
  const psaUpfBox: Box = { x: iUpfBox.x + iUpfBox.width + ROW3_GAP, y: ROW3_Y, width: 84, height: ROW3_H }
  const lastUpfBox = isUlcl ? psaUpfBox : iUpfBox
  const dnBox: Box = { x: Math.min(VIEWBOX_WIDTH - NF_MARGIN_X - 70, lastUpfBox.x + lastUpfBox.width + ROW3_GAP), y: ROW3_Y, width: 70, height: ROW3_H }

  // Two-SMF layout (ulcl-2slice): slice 1's chain sits in row 3, same as
  // the single-gNB layouts. Slice 2's chain interleaves through row 4,
  // each node centered between its slice-1 counterpart and the next one.
  const smf1Index = row2Nfs.indexOf('SMF1')
  const smf2Index = row2Nfs.indexOf('SMF2')
  const smf1CenterX = smf1Index >= 0 ? rowBoxCenterX(smf1Index, row2Nfs.length) : 0
  const smf2CenterX = smf2Index >= 0 ? rowBoxCenterX(smf2Index, row2Nfs.length) : 0
  const smf1Box: Box = { x: smf1CenterX - NF_BOX.width / 2, y: ROW2_Y, width: NF_BOX.width, height: NF_BOX.height }
  const smf2Box: Box = { x: smf2CenterX - NF_BOX.width / 2, y: ROW2_Y, width: NF_BOX.width, height: NF_BOX.height }

  const gnb1Box: Box = { x: amfCenterX - 50, y: ROW3_Y, width: 100, height: ROW3_H }
  const iUpf1Box: Box = { x: smf1CenterX - 42, y: ROW3_Y, width: 84, height: ROW3_H }
  const psaUpf1Box: Box = { x: iUpf1Box.x + iUpf1Box.width + ROW3_GAP, y: ROW3_Y, width: 84, height: ROW3_H }
  const dn1Box: Box = { x: Math.min(VIEWBOX_WIDTH - NF_MARGIN_X - 70, psaUpf1Box.x + psaUpf1Box.width + ROW3_GAP), y: ROW3_Y, width: 70, height: ROW3_H }

  // row 4 is row 3 shifted right by half the gap between gNB1 and I-UPF1 -
  // just enough that row 2's N2/N4 drops land in the gaps between row 3's
  // boxes instead of passing through them, while every row-4 link keeps
  // row 3's exact spacing (so nothing needs re-deriving per node).
  const row4Shift = (boxCenterX(iUpf1Box) - boxCenterX(gnb1Box)) / 2
  const gnb2Box = shiftToRow4(gnb1Box, row4Shift)
  const iUpf2Box = shiftToRow4(iUpf1Box, row4Shift)
  const psaUpf2Box = shiftToRow4(psaUpf1Box, row4Shift)
  const dn2Box: Box = { x: Math.min(VIEWBOX_WIDTH - NF_MARGIN_X - 70, psaUpf2Box.x + psaUpf2Box.width + ROW3_GAP), y: ROW4_Y, width: 70, height: ROW3_H }

  // only currently-up instances get a live Uu link - a never-deployed or
  // stopped subscriber isn't actually attached to anything right now.
  const activeUeRows = ueRows.filter((row) => row.status !== 'stopped')
  const ueRowY = isTwoSlice ? UE_ROW_TWO_SLICE_Y : UE_ROW_SINGLE_Y
  // used only to center/space the row of UE boxes - which gNB each one's Uu
  // line actually originates from is decided per-row below (ueUuSource), not
  // by this shared point.
  const ueFanCenterX = isTwoSlice ? (boxCenterX(gnb1Box) + boxCenterX(gnb2Box)) / 2 : boxCenterX(gnbBox)
  const ueFanBottomY = isTwoSlice ? ROW4_Y + ROW3_H : gnbBox.y + gnbBox.height
  const ueBoxX = (index: number): number => {
    const count = activeUeRows.length
    const totalWidth = count * UE_BOX.width + (count - 1) * 40
    const startX = ueFanCenterX - totalWidth / 2
    return startX + index * (UE_BOX.width + 40)
  }

  // each UE's Uu line starts from its OWN slice's gNB box - known from the
  // subscriber's sd - instead of a shared point, so a slice-1 UE never
  // visually reads as attached to gNB Slice 2. A UE whose sd doesn't match
  // either slice (detail still loading, or no session) falls back to the
  // old shared midpoint between the two gNBs.
  function ueUuSource(row: UeRow): { x: number; y: number } {
    if (isTwoSlice) {
      if (row.sd === SLICE1_SD) return { x: boxCenterX(gnb1Box), y: gnb1Box.y + gnb1Box.height }
      if (row.sd === SLICE2_SD) return { x: boxCenterX(gnb2Box), y: gnb2Box.y + gnb2Box.height }
    }
    return { x: ueFanCenterX, y: ueFanBottomY }
  }

  const viewHeight = activeUeRows.length > 0
    ? ueRowY + UE_BOX.height + 20
    : (isTwoSlice ? ROW4_Y + ROW3_H + 30 : ROW3_Y + ROW3_H + 30)

  return (
    <div className={styles.wrap}>
      <svg viewBox={`0 0 ${VIEWBOX_WIDTH} ${viewHeight}`} className={styles.svg} role="img" aria-label="Detailed 5G network interface topology">
        {/* SBI bus, straddled by row 1 (above) and row 2 (below) - the line
            runs a little wider than the NF rows so the "SBI" label has a
            clear spot at its left end. */}
        <line x1={8} y1={BUS_Y} x2={VIEWBOX_WIDTH - 8} y2={BUS_Y} className={styles.busLine} />
        <text x={8} y={BUS_Y - 6} className={styles.busLabel}>SBI</text>

        {ROW1_NFS.map((name, index) => {
          const x = rowBoxX(index, ROW1_NFS.length)
          const centerX = x + NF_BOX.width / 2
          const status = nfStatusByName.get(name) ?? 'stopped'
          return (
            <g key={name}>
              <line x1={centerX} y1={ROW1_Y + NF_BOX.height} x2={centerX} y2={BUS_Y} className={styles.connectorLine} />
              <NodeBox x={x} y={ROW1_Y} width={NF_BOX.width} height={NF_BOX.height} label={name} status={status} />
            </g>
          )
        })}

        {row2Nfs.map((name, index) => {
          const x = rowBoxX(index, row2Nfs.length)
          const centerX = x + NF_BOX.width / 2
          const status = nfStatusByName.get(name) ?? 'stopped'
          return (
            <g key={name}>
              <line x1={centerX} y1={BUS_Y} x2={centerX} y2={ROW2_Y} className={styles.connectorLine} />
              <NodeBox x={x} y={ROW2_Y} width={NF_BOX.width} height={NF_BOX.height} label={name} status={status} />
            </g>
          )
        })}

        {isTwoSlice ? (
          <>
            {/* row 3: slice 1's chain, same layout as the single-gNB case.
                SMF1 has a real PFCP association with both UPFs, so it gets
                two N4 drops, same as the single-SMF ULCL layout. */}
            <DropLink from={amfBox} to={gnb1Box} label="N2" labelY={(row2BottomY + gnb1Box.y) / 2} />
            <DropLink from={smf1Box} to={iUpf1Box} label="N4" labelY={(row2BottomY + iUpf1Box.y) / 2} />
            <ElbowDropLink from={smf1Box} to={psaUpf1Box} label="N4" waypointY={row2BottomY + 16} />
            <RowLink from={gnb1Box} to={iUpf1Box} label="N3" labelY={iUpf1Box.y - 8} />
            <RowLink from={iUpf1Box} to={psaUpf1Box} label="N9" labelY={psaUpf1Box.y - 8} />
            <RowLink from={psaUpf1Box} to={dn1Box} label="N6" labelY={psaUpf1Box.y - 8} dashed />

            {/* row 4: slice 2's chain, shifted right from row 3 (see
                row4Shift) - gnb2 sits in the gap between row 3's gNB1 and
                I-UPF1, so N2 elbows down through that same gap instead of
                cutting a diagonal across row 3; row 4's own N3/N9/N6/N4-pair
                keep row 3's shape. */}
            <ElbowDropLink from={amfBox} to={gnb2Box} label="N2" waypointY={row2BottomY + 8} />
            <ElbowDropLink from={smf2Box} to={iUpf2Box} label="N4" waypointY={row2BottomY + 28} />
            <ElbowDropLink from={smf2Box} to={psaUpf2Box} label="N4" waypointY={row2BottomY + 40} />
            <RowLink from={gnb2Box} to={iUpf2Box} label="N3" labelY={iUpf2Box.y - 8} />
            <RowLink from={iUpf2Box} to={psaUpf2Box} label="N9" labelY={psaUpf2Box.y - 8} />
            <RowLink from={psaUpf2Box} to={dn2Box} label="N6" labelY={psaUpf2Box.y - 8} dashed />

            <rect x={dn1Box.x} y={dn1Box.y} width={dn1Box.width} height={dn1Box.height} rx={7} className={styles.dnBox} />
            <text x={dn1Box.x + dn1Box.width / 2} y={dn1Box.y + dn1Box.height / 2} dominantBaseline="middle" textAnchor="middle" className={styles.dnLabel}>DN</text>
            <rect x={dn2Box.x} y={dn2Box.y} width={dn2Box.width} height={dn2Box.height} rx={7} className={styles.dnBox} />
            <text x={dn2Box.x + dn2Box.width / 2} y={dn2Box.y + dn2Box.height / 2} dominantBaseline="middle" textAnchor="middle" className={styles.dnLabel}>DN</text>

            <NodeBox x={iUpf1Box.x} y={iUpf1Box.y} width={iUpf1Box.width} height={iUpf1Box.height} label="I-UPF-1" status={nfStatusByName.get('I-UPF-1') ?? 'stopped'} />
            <NodeBox x={psaUpf1Box.x} y={psaUpf1Box.y} width={psaUpf1Box.width} height={psaUpf1Box.height} label="PSA-UPF-1" status={nfStatusByName.get('PSA-UPF-1') ?? 'stopped'} />
            <NodeBox x={iUpf2Box.x} y={iUpf2Box.y} width={iUpf2Box.width} height={iUpf2Box.height} label="I-UPF-2" status={nfStatusByName.get('I-UPF-2') ?? 'stopped'} />
            <NodeBox x={psaUpf2Box.x} y={psaUpf2Box.y} width={psaUpf2Box.width} height={psaUpf2Box.height} label="PSA-UPF-2" status={nfStatusByName.get('PSA-UPF-2') ?? 'stopped'} />
            <NodeBox x={gnb1Box.x} y={gnb1Box.y} width={gnb1Box.width} height={gnb1Box.height} label="gNB Slice 1" status={gnbSlice1Status} />
            <NodeBox x={gnb2Box.x} y={gnb2Box.y} width={gnb2Box.width} height={gnb2Box.height} label="gNB Slice 2" status={gnbSlice2Status} />
          </>
        ) : (
          <>
            {/* N2: AMF -> gNB */}
            <DropLink from={amfBox} to={gnbBox} label="N2" labelY={(row2BottomY + gnbBox.y) / 2} />

            {/* N4: SMF -> first UPF (I-UPF in ULCL mode, the sole UPF otherwise) */}
            <DropLink from={smfBox} to={iUpfBox} label="N4" labelY={(row2BottomY + iUpfBox.y) / 2} />

            {/* N4: SMF -> PSA-UPF too, ULCL only - a real PFCP session, same as the I-UPF one */}
            {isUlcl && <ElbowDropLink from={smfBox} to={psaUpfBox} label="N4" waypointY={row2BottomY + 16} />}

            {/* N3: gNB -> first UPF (row 3, side by side) */}
            <RowLink from={gnbBox} to={iUpfBox} label="N3" labelY={iUpfBox.y - 8} />

            {/* N9: I-UPF -> PSA-UPF, ULCL only */}
            {isUlcl && <RowLink from={iUpfBox} to={psaUpfBox} label="N9" labelY={psaUpfBox.y - 8} />}

            {/* N6: last UPF -> DN (external, always shown as a static endpoint) */}
            <RowLink from={lastUpfBox} to={dnBox} label="N6" labelY={lastUpfBox.y - 8} dashed />
            <g>
              <rect x={dnBox.x} y={dnBox.y} width={dnBox.width} height={dnBox.height} rx={7} className={styles.dnBox} />
              <text x={dnBox.x + dnBox.width / 2} y={dnBox.y + dnBox.height / 2} dominantBaseline="middle" textAnchor="middle" className={styles.dnLabel}>DN</text>
            </g>

            {isUlcl ? (
              <>
                <NodeBox x={iUpfBox.x} y={iUpfBox.y} width={iUpfBox.width} height={iUpfBox.height} label="I-UPF" status={nfStatusByName.get('I-UPF') ?? 'stopped'} />
                <NodeBox x={psaUpfBox.x} y={psaUpfBox.y} width={psaUpfBox.width} height={psaUpfBox.height} label="PSA-UPF" status={nfStatusByName.get('PSA-UPF') ?? 'stopped'} />
              </>
            ) : (
              <NodeBox x={iUpfBox.x} y={iUpfBox.y} width={iUpfBox.width} height={iUpfBox.height} label="UPF" status={nfStatusByName.get('UPF') ?? 'stopped'} />
            )}

            <NodeBox x={gnbBox.x} y={gnbBox.y} width={gnbBox.width} height={gnbBox.height} label="gNB" status={gnbStatus} />
          </>
        )}

        {/* Uu: gNB(s) -> each currently-up UE, from its own slice's gNB box */}
        {activeUeRows.map((row, index) => {
          const x = ueBoxX(index)
          const centerX = x + UE_BOX.width / 2
          const source = ueUuSource(row)
          const waypointY = isTwoSlice ? UU_WAYPOINT_Y_TWO_SLICE : (source.y + ueRowY) / 2
          return (
            <g key={row.ueId}>
              <path d={elbowPath(source.x, source.y, centerX, ueRowY, waypointY)} className={styles.connectorLine} fill="none" />
              {index === 0 && <InterfaceLabel x={centerX + 22} y={waypointY - 6} text="Uu" />}
              <NodeBox
                x={x}
                y={ueRowY}
                width={UE_BOX.width}
                height={UE_BOX.height}
                label={`…${row.ueId.slice(-4)}`}
                status={row.status}
                title={row.ueId}
              />
            </g>
          )
        })}
      </svg>
      {activeUeRows.length === 0 && (
        <p className={styles.emptyUeHint}>No UE instances are up right now - deploy one to see it attach to the gNB here.</p>
      )}
    </div>
  )
}
