import StatusDot from './StatusDot'
import type { DeploymentNode, NetworkFunction, NodeId, NodeStatus } from './types'
import type { UeRow } from './useUeInstances'
import styles from './topology-canvas.module.css'

// how many UE chips the column shows before collapsing the rest into "+N"
const MAX_UE_CHIPS = 8

export interface TopologySelection {
  node: NodeId
  // a single NF inside the core box, or a single UE instance chip
  nf: string | null
  ue: string | null
}

interface TopologyCanvasProps {
  ueRows: UeRow[]
  gnb: DeploymentNode
  gnbDetail: string
  coreStatus: NodeStatus
  coreNfs: NetworkFunction[]
  selection: TopologySelection
  onSelect: (selection: TopologySelection) => void
}

// The prototype's topology panel body: a UE column of instance chips, the
// Uu link, the RAN gNB node, the N2 / N3 link and the 5G core box with one
// node per network function.
export default function TopologyCanvas({ ueRows, gnb, gnbDetail, coreStatus, coreNfs, selection, onSelect }: TopologyCanvasProps) {
  const shownUes = ueRows.slice(0, MAX_UE_CHIPS)
  const hiddenUes = ueRows.length - shownUes.length
  const coreSelected = selection.node === 'core' && selection.nf === null

  return (
    <div className={styles.canvas}>
      <div className={styles.ueColumn}>
        <span className={styles.colLabel}>UE · <span className="num">{ueRows.length}</span></span>
        {shownUes.map((row) => {
          const on = selection.node === 'ue' && selection.ue === row.ueId
          return (
            <button
              key={row.ueId}
              type="button"
              className={`${styles.node} ${styles.ueChip} ${on ? styles.nodeOn : ''}`}
              onClick={() => onSelect({ node: 'ue', nf: null, ue: row.ueId })}
              aria-pressed={on}
              title={row.ueId}
            >
              <StatusDot status={row.status} />
              <span className={`num ${styles.ueId}`}>{shortUeId(row.ueId)}</span>
            </button>
          )
        })}
        {hiddenUes > 0 && (
          <button
            type="button"
            className={`${styles.node} ${styles.ueChip} ${selection.node === 'ue' && selection.ue === null ? styles.nodeOn : ''}`}
            onClick={() => onSelect({ node: 'ue', nf: null, ue: null })}
          >
            <span className={`num ${styles.ueId}`}>+{hiddenUes} more</span>
          </button>
        )}
        {ueRows.length === 0 && (
          <button
            type="button"
            className={`${styles.node} ${styles.ueChip} ${selection.node === 'ue' ? styles.nodeOn : ''}`}
            onClick={() => onSelect({ node: 'ue', nf: null, ue: null })}
          >
            <StatusDot status="stopped" />
            <span className={styles.ueEmpty}>No subscribers</span>
          </button>
        )}
      </div>

      <div className={styles.link} aria-hidden="true"><span>Uu</span><span className={styles.linkLine} /></div>

      <div className={styles.ranColumn}>
        <span className={styles.colLabel}>RAN</span>
        <button
          type="button"
          className={`${styles.node} ${styles.gnbNode} ${selection.node === 'gnb' ? styles.nodeOn : ''}`}
          onClick={() => onSelect({ node: 'gnb', nf: null, ue: null })}
          aria-pressed={selection.node === 'gnb'}
        >
          <span className={styles.gnbHead}><StatusDot status={gnb.status} />gNB</span>
          <span className={`num ${styles.gnbSub}`}>{gnbDetail}</span>
        </button>
      </div>

      <div className={styles.link} aria-hidden="true"><span>N2 / N3</span><span className={styles.linkLine} /></div>

      <div className={styles.coreColumn}>
        {/* the column label selects the core as a whole (template, deploy/stop) */}
        <button
          type="button"
          className={`${styles.colLabel} ${styles.colLabelButton}`}
          onClick={() => onSelect({ node: 'core', nf: null, ue: null })}
          aria-pressed={coreSelected}
        >
          5G CORE · FREE5GC
        </button>
        <div className={`${styles.coreBox} ${coreSelected ? styles.coreBoxOn : ''}`}>
          {coreNfs.length === 0 ? (
            <button
              type="button"
              className={`${styles.node} ${styles.coreEmpty}`}
              onClick={() => onSelect({ node: 'core', nf: null, ue: null })}
              aria-pressed={coreSelected}
            >
              <StatusDot status={coreStatus} />
              {coreStatus === 'stopped' ? 'Not deployed' : 'Waiting for network functions…'}
            </button>
          ) : coreNfs.map((nf) => {
            const on = selection.node === 'core' && selection.nf === nf.name
            return (
              <button
                key={nf.name}
                type="button"
                className={`${styles.node} ${on ? styles.nodeOn : ''}`}
                onClick={() => onSelect({ node: 'core', nf: nf.name, ue: null })}
                aria-pressed={on}
              >
                <StatusDot status={nf.status} />
                <span className={styles.nfName}>{nf.name}</span>
              </button>
            )
          })}
        </div>
      </div>
    </div>
  )
}

// "imsi-208930000000001" → "imsi-…0001", the prototype's chip label
function shortUeId(ueId: string): string {
  const match = /^([a-z]+-)(\d{5,})$/i.exec(ueId)
  return match ? `${match[1]}…${match[2].slice(-4)}` : ueId
}
