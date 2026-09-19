import { Fragment } from 'react'
import { getStatusMeta } from './statusMeta'
import type { DeploymentNode, NodeId } from './types'
import styles from './topology-canvas.module.css'

interface TopologyCanvasProps {
  nodes: Record<NodeId, DeploymentNode>
  selected: NodeId
  onSelect: (id: NodeId) => void
}

const ORDER: NodeId[] = ['ue', 'gnb', 'core']

// one label per gap between ORDER[i] and ORDER[i + 1]
const CONNECTOR_LABELS = ['Uu', 'N2 / N3']

const ICONS: Record<NodeId, string> = {
  ue: '📱',
  gnb: '📡',
  core: '5G',
}

export default function TopologyCanvas({ nodes, selected, onSelect }: TopologyCanvasProps) {
  return (
    <div className={styles.canvas}>
      {ORDER.map((id, index) => {
        const node = nodes[id]
        const meta = getStatusMeta(node.status)
        const isSelected = selected === id

        return (
          <Fragment key={id}>
            {index > 0 && (
              <div className={styles.connector} aria-hidden="true">
                <span className={styles.connectorLabel}>{CONNECTOR_LABELS[index - 1]}</span>
                <span className={styles.connectorLine} />
              </div>
            )}

            <button
              className={`${styles.node} ${isSelected ? styles.nodeSelected : ''}`}
              onClick={() => onSelect(id)}
            >
              <div className={styles.nodeTop}>
                <div className={`${styles.nodeIcon} ${id === 'core' ? styles.nodeIconCore : ''}`}>{ICONS[id]}</div>
                <span className={styles.dragHandle} aria-hidden="true">⠿</span>
              </div>
              <div className={styles.nodeLabel}>{node.label}</div>
              <div className={styles.nodeSublabel}>{node.sublabel}</div>
              <span className={styles.statusPill} style={{ background: meta.bg, color: meta.color, borderColor: meta.border }}>
                <span className={styles.statusDot} style={{ background: meta.dot }} />
                {node.statusLabel ?? meta.text}
              </span>
            </button>
          </Fragment>
        )
      })}
    </div>
  )
}
