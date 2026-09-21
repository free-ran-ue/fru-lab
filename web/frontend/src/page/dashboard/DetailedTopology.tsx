import { getStatusMeta } from './statusMeta'
import { detectFree5gcTemplate } from './free5gcTemplate'
import type { NetworkFunction, NodeStatus } from './types'
import type { UeRow } from './useUeInstances'
import styles from './detailed-topology.module.css'

interface DetailedTopologyProps {
  free5gcNfs: NetworkFunction[]
  gnbStatus: NodeStatus
  ueRows: UeRow[]
}

// The 9 SBI-connected control-plane NFs, split across the two rows that
// straddle the SBI bus line. The data-plane UPF(s) are deliberately
// excluded - they're controlled over N4 (PFCP), not part of the SBI mesh
// the others talk over, and rendered separately in row 3 below. SMF/AMF sit
// in row 2 (closest to the bus/RAN side) so their N4/N2 drops stay short.
const ROW1_NFS = ['AUSF', 'UDM', 'UDR', 'NRF', 'NSSF']
const ROW2_NFS = ['PCF', 'AMF', 'SMF', 'CHF']

const VIEWBOX_WIDTH = 980
const NF_MARGIN_X = 40
const NF_BOX = { width: 74, height: 32 }

const ROW1_Y = 20
const BUS_Y = 92
const ROW2_Y = 128
const ROW3_Y = 216
const ROW3_H = 40
const ROW3_GAP = 90
const ROW4_Y = 320
const UE_BOX = { width: 74, height: 32 }

function rowBoxX(index: number, count: number): number {
  const usableWidth = VIEWBOX_WIDTH - NF_MARGIN_X * 2 - NF_BOX.width
  const step = count > 1 ? usableWidth / (count - 1) : 0
  return NF_MARGIN_X + step * index
}

function rowBoxCenterX(index: number, count: number): number {
  return rowBoxX(index, count) + NF_BOX.width / 2
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

export default function DetailedTopology({ free5gcNfs, gnbStatus, ueRows }: DetailedTopologyProps) {
  const nfStatusByName = new Map(free5gcNfs.map((nf) => [nf.name, nf.status]))

  // ULCL splits the data plane into an intermediate UPF (N3-facing) and a
  // PDU session anchor UPF (N6-facing), chained by N9.
  const isUlcl = detectFree5gcTemplate(free5gcNfs) === 'ulcl'

  const smfIndex = ROW2_NFS.indexOf('SMF')
  const amfIndex = ROW2_NFS.indexOf('AMF')
  const smfCenterX = rowBoxCenterX(smfIndex, ROW2_NFS.length)
  const amfCenterX = rowBoxCenterX(amfIndex, ROW2_NFS.length)

  // Row 3 reads left to right as gNB, UPF(s), DN: gNB sits under AMF (short
  // N2 drop), the first UPF sits under SMF (short N4 drop) and to gNB's
  // right (N3). In ULCL mode a second UPF (the PSA) chains off the first
  // via N9, and DN hangs off whichever UPF is last (N6).
  const gnbBox = { x: amfCenterX - 50, y: ROW3_Y, width: 100, height: ROW3_H }
  const iUpfBox = { x: smfCenterX - 42, y: ROW3_Y, width: 84, height: ROW3_H }
  const psaUpfBox = { x: iUpfBox.x + iUpfBox.width + ROW3_GAP, y: ROW3_Y, width: 84, height: ROW3_H }
  const lastUpfBox = isUlcl ? psaUpfBox : iUpfBox
  const dnBox = { x: Math.min(VIEWBOX_WIDTH - NF_MARGIN_X - 70, lastUpfBox.x + lastUpfBox.width + ROW3_GAP), y: ROW3_Y, width: 70, height: ROW3_H }

  const gnbCenterX = gnbBox.x + gnbBox.width / 2
  const gnbBottomY = gnbBox.y + gnbBox.height

  // only currently-up instances get a live Uu link - a never-deployed or
  // stopped subscriber isn't actually attached to anything right now.
  const activeUeRows = ueRows.filter((row) => row.status !== 'stopped')
  const ueBoxX = (index: number): number => {
    const count = activeUeRows.length
    const totalWidth = count * UE_BOX.width + (count - 1) * 40
    const startX = gnbCenterX - totalWidth / 2
    return startX + index * (UE_BOX.width + 40)
  }

  const viewHeight = activeUeRows.length > 0 ? ROW4_Y + UE_BOX.height + 20 : ROW3_Y + ROW3_H + 30

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

        {ROW2_NFS.map((name, index) => {
          const x = rowBoxX(index, ROW2_NFS.length)
          const centerX = x + NF_BOX.width / 2
          const status = nfStatusByName.get(name) ?? 'stopped'
          return (
            <g key={name}>
              <line x1={centerX} y1={BUS_Y} x2={centerX} y2={ROW2_Y} className={styles.connectorLine} />
              <NodeBox x={x} y={ROW2_Y} width={NF_BOX.width} height={NF_BOX.height} label={name} status={status} />
            </g>
          )
        })}

        {/* N4: SMF -> first UPF (I-UPF in ULCL mode, the sole UPF otherwise) */}
        <line x1={smfCenterX} y1={ROW2_Y + NF_BOX.height} x2={iUpfBox.x + iUpfBox.width / 2} y2={iUpfBox.y} className={styles.connectorLine} />
        <InterfaceLabel x={smfCenterX + 14} y={(ROW2_Y + NF_BOX.height + iUpfBox.y) / 2} text="N4" />

        {/* N4: SMF -> PSA-UPF too, ULCL only - a real PFCP session, same as the I-UPF one */}
        {isUlcl && (
          <>
            <line x1={smfCenterX} y1={ROW2_Y + NF_BOX.height} x2={psaUpfBox.x + psaUpfBox.width / 2} y2={psaUpfBox.y} className={styles.connectorLine} />
            <InterfaceLabel x={(smfCenterX + psaUpfBox.x + psaUpfBox.width / 2) / 2} y={ROW2_Y + NF_BOX.height + 14} text="N4" />
          </>
        )}

        {/* N2: AMF -> gNB */}
        <line x1={amfCenterX} y1={ROW2_Y + NF_BOX.height} x2={gnbCenterX} y2={gnbBox.y} className={styles.connectorLine} />
        <InterfaceLabel x={amfCenterX + 14} y={(ROW2_Y + NF_BOX.height + gnbBox.y) / 2} text="N2" />

        {/* N3: gNB -> first UPF (row 3, side by side) */}
        <line x1={gnbBox.x + gnbBox.width} y1={gnbBox.y + gnbBox.height / 2} x2={iUpfBox.x} y2={iUpfBox.y + iUpfBox.height / 2} className={styles.connectorLine} />
        <InterfaceLabel x={(gnbBox.x + gnbBox.width + iUpfBox.x) / 2} y={iUpfBox.y + iUpfBox.height / 2 - 8} text="N3" />

        {/* N9: I-UPF -> PSA-UPF, ULCL only */}
        {isUlcl && (
          <>
            <line x1={iUpfBox.x + iUpfBox.width} y1={iUpfBox.y + iUpfBox.height / 2} x2={psaUpfBox.x} y2={psaUpfBox.y + psaUpfBox.height / 2} className={styles.connectorLine} />
            <InterfaceLabel x={(iUpfBox.x + iUpfBox.width + psaUpfBox.x) / 2} y={psaUpfBox.y + psaUpfBox.height / 2 - 8} text="N9" />
          </>
        )}

        {/* N6: last UPF -> DN (external, always shown as a static endpoint) */}
        <line x1={lastUpfBox.x + lastUpfBox.width} y1={lastUpfBox.y + lastUpfBox.height / 2} x2={dnBox.x} y2={dnBox.y + dnBox.height / 2} className={styles.connectorLineDashed} />
        <InterfaceLabel x={(lastUpfBox.x + lastUpfBox.width + dnBox.x) / 2} y={lastUpfBox.y + lastUpfBox.height / 2 - 8} text="N6" />
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

        {/* Uu: gNB -> each currently-up UE */}
        {activeUeRows.map((row, index) => {
          const x = ueBoxX(index)
          const centerX = x + UE_BOX.width / 2
          return (
            <g key={row.ueId}>
              <line x1={gnbCenterX} y1={gnbBottomY} x2={centerX} y2={ROW4_Y} className={styles.connectorLine} />
              {index === 0 && <InterfaceLabel x={gnbCenterX + 22} y={(gnbBottomY + ROW4_Y) / 2} text="Uu" />}
              <NodeBox
                x={x}
                y={ROW4_Y}
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
