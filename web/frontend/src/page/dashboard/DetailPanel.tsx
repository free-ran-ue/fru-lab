import Button from '../../components/button/button'
import { getStatusMeta } from './statusMeta'
import type { DeploymentNode, NetworkFunction } from './types'
import styles from './detail-panel.module.css'

interface DetailPanelProps {
  node: DeploymentNode
  networkFunctions: NetworkFunction[]
  onViewLogs: () => void
}

export default function DetailPanel({ node, networkFunctions, onViewLogs }: DetailPanelProps) {
  const meta = getStatusMeta(node.status)
  const isCore = node.id === 'core'
  const isRunning = node.status === 'running'

  return (
    <div className={styles.panel}>
      <div>
        <div className={styles.eyebrow}>Selected node</div>
        <h4 className={styles.nodeLabel}>{node.label}</h4>
        <p className={styles.nodeSublabel}>{node.sublabel}</p>
        <span className={styles.statusPill} style={{ background: meta.bg, color: meta.color, borderColor: meta.border }}>
          <span className={styles.statusDot} style={{ background: meta.dot }} />
          {meta.text}
        </span>
      </div>

      <div className={styles.divider} />

      {isCore ? (
        <div>
          <div className={styles.sectionTitle}>Network Functions</div>
          <div className={styles.nfGrid}>
            {networkFunctions.map((nf) => {
              const nfMeta = getStatusMeta(nf.status)
              return (
                <div key={nf.name} className={styles.nfChip} style={{ background: nfMeta.bg, color: nfMeta.color, borderColor: nfMeta.border }}>
                  <span className={styles.nfDot} style={{ background: nfMeta.dot }} />
                  {nf.name}
                </div>
              )
            })}
          </div>
        </div>
      ) : (
        <div className={styles.infoList}>
          <div className={styles.infoRow}>
            <span>Container</span>
            <span className={styles.infoValue}>{`fru-${node.id}-1`}</span>
          </div>
          <div className={styles.infoRow}>
            <span>Image</span>
            <span className={styles.infoValue}>{`free-ran-ue:${node.id}`}</span>
          </div>
        </div>
      )}

      <div className={styles.divider} />

      <div className={styles.metaList}>
        <div className={styles.metaRow}>
          <span>Last deployed</span>
          <span className={styles.metaValue}>{node.lastDeployed}</span>
        </div>
      </div>

      <div className={styles.actions}>
        <Button variant="primary">{isRunning ? 'Stop Service' : 'Deploy Service'}</Button>
        <Button variant="secondary" onClick={onViewLogs}>View Logs</Button>
      </div>
    </div>
  )
}
