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
  onStopAll: () => void
  onOpenTerminal: (ueId: string) => void
  // set when gNB isn't running - deploying a UE now would just fail to dial
  // it, so every "Deploy" button is disabled with this as the reason.
  deployBlockedReason?: string
}

export default function UePanel({ rows, isLoading, loadError, pendingInstances, onDeploy, onStop, onStopAll, onOpenTerminal, deployBlockedReason }: UePanelProps) {
  const isDeployBlocked = Boolean(deployBlockedReason)
  const runningRows = rows.filter((row) => row.status !== 'stopped')
  const isAnyPending = runningRows.some((row) => pendingInstances[row.ueId])
  return (
    <div className={styles.panel}>
      <div>
        <div className={styles.eyebrow}>Selected node</div>
        <h4 className={styles.nodeLabel}>UE</h4>
        <p className={styles.nodeSublabel}>free-ran-ue · one container per subscriber</p>
      </div>

      <div className={styles.divider} />

      <div>
        <div className={ueStyles.sectionHeader}>
          <div className={styles.sectionTitle}>UE Instances</div>
          {runningRows.length > 0 && (
            <button className={ueStyles.stopAllButton} onClick={onStopAll} disabled={isAnyPending}>
              {isAnyPending ? 'Stopping…' : 'Stop All'}
            </button>
          )}
        </div>
        {isDeployBlocked && (
          <p className={styles.blockedHint}>{deployBlockedReason}</p>
        )}
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
                    {isRunning && (
                      <button className={ueStyles.linkButton} onClick={() => onOpenTerminal(row.ueId)}>Terminal</button>
                    )}
                    {isRunning ? (
                      <button className={ueStyles.linkButton} onClick={() => onStop(row.ueId)} disabled={isPending}>
                        {isPending ? 'Stopping…' : 'Stop'}
                      </button>
                    ) : (
                      <button className={ueStyles.linkButton} onClick={() => onDeploy(row.ueId)} disabled={isPending || isDeployBlocked}>
                        {isPending ? 'Deploying…' : 'Deploy'}
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
