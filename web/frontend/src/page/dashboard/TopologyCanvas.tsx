import { getStatusMeta } from './statusMeta'
import type { DeploymentNode, NodeId } from './types'
import styles from './topology-canvas.module.css'

interface TopologyCanvasProps {
  nodes: Record<NodeId, DeploymentNode>
  selected: NodeId
  onSelect: (id: NodeId) => void
}

const ORDER: NodeId[] = ['ue', 'gnb', 'core']

const ICONS: Record<NodeId, string> = {
  ue: '📱',
  gnb: '📡',
  core: '5G',
}

export default function TopologyCanvas({ nodes, selected, onSelect }: TopologyCanvasProps) {
  return (
    <div className={styles.canvas}>
      <svg className={styles.lines} width="800" height="380" viewBox="0 0 800 380">
        <defs>
          <marker id="topology-arrow" markerWidth="8" markerHeight="8" refX="6" refY="3" orient="auto">
            <path d="M0,0 L6,3 L0,6 Z" fill="#94a3b8" />
          </marker>
        </defs>
        <line x1="220" y1="185" x2="299" y2="185" stroke="#94a3b8" strokeWidth="2" markerEnd="url(#topology-arrow)" />
        <text x="240" y="177" fill="#94a3b8" fontSize="11">Uu</text>
        <line x1="495" y1="185" x2="574" y2="185" stroke="#94a3b8" strokeWidth="2" markerEnd="url(#topology-arrow)" />
        <text x="512" y="177" fill="#94a3b8" fontSize="11">N2 / N3</text>
      </svg>

      {ORDER.map((id) => {
        const node = nodes[id]
        const meta = getStatusMeta(node.status)
        const isSelected = selected === id

        return (
          <button
            key={id}
            className={`${styles.node} ${styles[id]} ${isSelected ? styles.nodeSelected : ''}`}
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
              {meta.text}
            </span>
          </button>
        )
      })}
    </div>
  )
}
