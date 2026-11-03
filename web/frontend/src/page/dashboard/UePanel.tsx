import Button from '../../components/button/button'
import DetailPanel from './DetailPanel'
import StatusDot from './StatusDot'
import { getStatusMeta } from './statusMeta'
import type { DeploymentNode } from './types'
import type { UeRow } from './useUeInstances'
import styles from './ue-panel.module.css'

interface UePanelProps {
  node: DeploymentNode
  rows: UeRow[]
  selectedUe: string | null
  onSelectUe: (ueId: string) => void
  isLoading: boolean
  loadError: string | null
  pendingInstances: Record<string, boolean>
  onDeploy: (ueId: string) => void
  onStop: (ueId: string) => void
  onStopAll: () => void
  onOpenTerminal: (ueId: string) => void
  onViewLogs: () => void
  // set when gNB isn't running - deploying a UE now would just fail to dial
  // it, so every "Deploy" button is disabled with this as the reason.
  deployBlockedReason?: string
}

// The "Selected node" aside for the UE fleet: one row per subscriber with
// its own deploy / stop / terminal actions.
export default function UePanel({ node, rows, selectedUe, onSelectUe, isLoading, loadError, pendingInstances, onDeploy, onStop, onStopAll, onOpenTerminal, onViewLogs, deployBlockedReason }: UePanelProps) {
  const isDeployBlocked = Boolean(deployBlockedReason)
  const runningRows = rows.filter((row) => row.status !== 'stopped')
  const isAnyPending = runningRows.some((row) => pendingInstances[row.ueId])
  const selectedRow = rows.find((row) => row.ueId === selectedUe)

  const detailRows = [
    { k: 'Template', v: 'free-ran-ue · basic' },
    { k: 'Instances', v: node.statusLabel ?? '—' },
    ...(selectedRow
      ? [
        { k: 'Selected', v: selectedRow.ueId },
        { k: 'Status', v: getStatusMeta(selectedRow.status).text },
        { k: 'Last deployed', v: selectedRow.lastDeployed },
      ]
      : [{ k: 'Last deployed', v: node.lastDeployed }]),
  ]

  return (
    <DetailPanel
      name="UE"
      status={node.status}
      statusText={node.statusLabel}
      rows={detailRows}
      hint={deployBlockedReason}
      footer={(
        <>
          <Button variant="secondary" onClick={onViewLogs}>View logs</Button>
          {runningRows.length > 0 && (
            <Button variant="secondary" onClick={onStopAll} disabled={isAnyPending}>
              {isAnyPending ? 'Stopping…' : 'Stop all'}
            </Button>
          )}
        </>
      )}
    >
      <span className={styles.sectionTitle}>UE INSTANCES</span>
      {isLoading ? (
        <p className={styles.emptyState}>Loading subscribers…</p>
      ) : loadError ? (
        <p className={styles.emptyState}>{loadError}</p>
      ) : rows.length === 0 ? (
        <p className={styles.emptyState}>No subscribers yet. Create one on the Subscribers page first.</p>
      ) : (
        <div className={styles.instanceList}>
          {rows.map((row) => {
            const isPending = pendingInstances[row.ueId] ?? false
            const isRunning = row.status === 'running'
            const on = row.ueId === selectedUe
            return (
              <div key={row.ueId} className={`${styles.instanceRow} ${on ? styles.instanceRowOn : ''}`}>
                <button type="button" className={styles.instanceInfo} onClick={() => onSelectUe(row.ueId)} title={row.ueId}>
                  <StatusDot status={row.status} />
                  <span className={`num ${styles.instanceId}`}>{row.ueId}</span>
                </button>
                <div className={styles.instanceActions}>
                  {isRunning && (
                    <button type="button" className={styles.linkButton} onClick={() => onOpenTerminal(row.ueId)}>Terminal</button>
                  )}
                  {row.status !== 'stopped' ? (
                    // a deploying (possibly stuck) or unhealthy UE can be stopped too
                    <button type="button" className={styles.linkButton} onClick={() => onStop(row.ueId)} disabled={isPending}>
                      {isPending ? 'Stopping…' : 'Stop'}
                    </button>
                  ) : (
                    <button type="button" className={styles.linkButton} onClick={() => onDeploy(row.ueId)} disabled={isPending || isDeployBlocked}>
                      {isPending ? 'Deploying…' : 'Deploy'}
                    </button>
                  )}
                </div>
              </div>
            )
          })}
        </div>
      )}
    </DetailPanel>
  )
}
