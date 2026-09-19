import { getStatusMeta } from './statusMeta'
import type { UeRow } from './useUeInstances'
import styles from './detail-panel.module.css'
import ueStyles from './ue-panel.module.css'

interface UePanelProps {
  rows: UeRow[]
  isLoading: boolean
  loadError: string | null
  pendingInstances: Record<string, boolean>
  onDeploy: (ueId: string) => void
  onStop: (ueId: string) => void
  onViewLogs: (ueId: string) => void
  onOpenTerminal: (ueId: string) => void
}

export default function UePanel({ rows, isLoading, loadError, pendingInstances, onDeploy, onStop, onViewLogs, onOpenTerminal }: UePanelProps) {
  return (
    <div className={styles.panel}>
      <div>
        <div className={styles.eyebrow}>Selected node</div>
        <h4 className={styles.nodeLabel}>UE</h4>
        <p className={styles.nodeSublabel}>free-ran-ue · one container per subscriber</p>
      </div>

      <div className={styles.divider} />

      <div>
        <div className={styles.sectionTitle}>UE Instances</div>
        {isLoading ? (
          <p className={ueStyles.emptyState}>Loading subscribers…</p>
        ) : loadError ? (
          <p className={ueStyles.emptyState}>{loadError}</p>
        ) : rows.length === 0 ? (
          <p className={ueStyles.emptyState}>No subscribers yet. Create one on the 5G Subscriber page first.</p>
        ) : (
          <div className={ueStyles.instanceList}>
            {rows.map((row) => {
              const meta = getStatusMeta(row.status)
              const isPending = pendingInstances[row.ueId] ?? false
              const isRunning = row.status === 'running'
              return (
                <div key={row.ueId} className={ueStyles.instanceRow}>
                  <div className={ueStyles.instanceInfo}>
                    <span className={ueStyles.instanceId} title={row.ueId}>{row.ueId}</span>
                    <span className={styles.statusPill} style={{ background: meta.bg, color: meta.color, borderColor: meta.border }}>
                      <span className={styles.statusDot} style={{ background: meta.dot }} />
                      {meta.text}
                    </span>
                  </div>
                  <div className={ueStyles.instanceActions}>
                    {row.hasBeenDeployed && (
                      <button className={ueStyles.linkButton} onClick={() => onViewLogs(row.ueId)}>Logs</button>
                    )}
                    {isRunning && (
                      <button className={ueStyles.linkButton} onClick={() => onOpenTerminal(row.ueId)}>Terminal</button>
                    )}
                    {isRunning ? (
                      <button className={ueStyles.linkButton} onClick={() => onStop(row.ueId)} disabled={isPending}>
                        {isPending ? 'Stopping…' : 'Stop'}
                      </button>
                    ) : (
                      <button className={ueStyles.linkButton} onClick={() => onDeploy(row.ueId)} disabled={isPending}>
                        {isPending ? 'Deploying…' : row.hasBeenDeployed ? 'Redeploy' : 'Deploy'}
                      </button>
                    )}
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </div>
    </div>
  )
}
