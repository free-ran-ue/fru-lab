import { StatusTag } from './StatusTag'
import type { UeRow } from './useUeInstances'
import styles from './dashboard-page.module.css'

interface UeInstancesPanelProps {
  rows: UeRow[]
  isLoading: boolean
  loadError: string | null
  pendingInstances: Record<string, boolean>
  selectedUe: string | null
  onSelectUe: (ueId: string) => void
  onDeploy: (ueId: string) => void
  onStop: (ueId: string) => void
  onStopAll: () => void
  onOpenTerminal: (ueId: string) => void
  // set while gNB isn't running - deploying a UE now would just fail to
  // dial it, so every Deploy is disabled with this as the reason
  deployBlockedReason?: string
}

// The v2 "UE instances" panel: one row per subscriber (deployed or not),
// each with its own terminal / deploy / stop actions.
export default function UeInstancesPanel({ rows, isLoading, loadError, pendingInstances, selectedUe, onSelectUe, onDeploy, onStop, onStopAll, onOpenTerminal, deployBlockedReason }: UeInstancesPanelProps) {
  const upRows = rows.filter((row) => row.status !== 'stopped')
  const isAnyPending = upRows.some((row) => pendingInstances[row.ueId])

  let emptyText: string | null = null
  if (isLoading) emptyText = 'Loading subscribers…'
  else if (loadError) emptyText = loadError
  else if (rows.length === 0) emptyText = 'No subscribers yet. Create one on the Subscribers page first.'

  return (
    <section className={styles.panel} aria-label="UE instances">
      <div className={styles.ph}>
        <h2>UE instances</h2>
        <span className={styles.aux}>free-ran-ue · one container per subscriber</span>
        <button type="button" className={styles.btn} onClick={onStopAll} disabled={upRows.length === 0 || isAnyPending}>
          {isAnyPending ? 'Stopping…' : 'Stop all'}
        </button>
      </div>
      <div className={`${styles.tableWrap} ${styles.ueScroll}`}>
        <table className={`${styles.tbl} ${styles.ueTable}`}>
          <thead>
            <tr>
              <th>UE ID</th>
              <th>PLMN</th>
              <th>Status</th>
              <th className={styles.r}>Last deployed</th>
              <th className={styles.actionsCol}>Actions</th>
            </tr>
          </thead>
          <tbody>
            {emptyText ? (
              <tr><td colSpan={5} className={styles.emptyCell}>{emptyText}</td></tr>
            ) : rows.map((row) => {
              const isPending = pendingInstances[row.ueId] ?? false
              const on = row.ueId === selectedUe
              return (
                <tr key={row.ueId} className={on ? styles.rowOn : undefined} onClick={() => onSelectUe(row.ueId)}>
                  <td>
                    <button type="button" className={`num ${styles.cellButton}`} aria-pressed={on} onClick={(event) => { event.stopPropagation(); onSelectUe(row.ueId) }}>
                      {row.ueId}
                    </button>
                  </td>
                  <td className="num">{row.plmnId}</td>
                  <td><StatusTag status={row.status} /></td>
                  <td className={styles.r}>{row.lastDeployed}</td>
                  <td className={styles.actionsCol} onClick={(event) => event.stopPropagation()}>
                    {row.status === 'running' && (
                      <>
                        <button type="button" className={styles.lnk} onClick={() => onOpenTerminal(row.ueId)}>Terminal</button>
                        <span className={styles.sep} aria-hidden="true">|</span>
                      </>
                    )}
                    {row.status !== 'stopped' ? (
                      // a deploying (possibly stuck) or unhealthy UE can be stopped too
                      <button type="button" className={`${styles.lnk} ${styles.lnkDanger}`} onClick={() => onStop(row.ueId)} disabled={isPending}>
                        {isPending ? 'Stopping…' : 'Stop'}
                      </button>
                    ) : (
                      <button
                        type="button"
                        className={styles.lnk}
                        onClick={() => onDeploy(row.ueId)}
                        disabled={isPending || Boolean(deployBlockedReason)}
                        title={deployBlockedReason}
                      >
                        {isPending ? 'Deploying…' : 'Deploy'}
                      </button>
                    )}
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
    </section>
  )
}
