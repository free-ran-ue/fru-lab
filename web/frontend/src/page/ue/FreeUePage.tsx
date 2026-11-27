import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import AppLayout from '../../components/layout/AppLayout'
import NotificationContainer from '../../components/notifications/NotificationContainer'
import { useNotifications } from '../../hooks/useNotifications'
import { extractErrorMessage } from '../../apiClient'
import SelectedNodePanel, { type DetailRow } from '../../lab/SelectedNodePanel'
import UeInstancesPanel from '../../lab/UeInstancesPanel'
import UeTerminalModal from '../../lab/UeTerminalModal'
import { StatusTag } from '../../lab/StatusTag'
import { useLab } from '../../lab/useLab'
import styles from '../../lab/lab.module.css'

// FreeUePage runs one free-ran-ue UE container per subscriber: deploy,
// stop, logs and a terminal into each.
export default function FreeUePage() {
  const navigate = useNavigate()
  const lab = useLab()
  const { ue, nodes, ueCounts } = lab
  const { errors, successes, addError, addSuccess, removeNotification } = useNotifications()
  const [selectedUe, setSelectedUe] = useState<string | null>(null)
  const [terminal, setTerminal] = useState<string | null>(null)

  const row = ue.rows.find((r) => r.ueId === selectedUe) ?? null
  const deployBlocked = lab.blocked.ueDeploy
  const anyPending = lab.upUes.some((r) => ue.pendingInstances[r.ueId])

  async function handleDeploy(ueId: string) {
    try {
      await ue.deploy(ueId)
      addSuccess(`Deploying UE for ${ueId}`)
    } catch (error) {
      addError(extractErrorMessage(error, `Failed to deploy UE for ${ueId}`))
    }
  }

  async function handleStop(ueId: string) {
    try {
      await ue.stop(ueId)
      addSuccess(`UE ${ueId} stopped`)
    } catch (error) {
      addError(extractErrorMessage(error, `Failed to stop UE ${ueId}`))
    }
  }

  async function handleStopAll() {
    try {
      await ue.stopAll()
      addSuccess('All UE instances stopped')
    } catch (error) {
      addError(extractErrorMessage(error, 'Failed to stop all UE instances'))
    }
  }

  function viewLogs() {
    navigate(row && row.hasBeenDeployed ? `/logs?target=ue&instance=${encodeURIComponent(row.ueId)}` : '/logs?target=ue')
  }

  const details: DetailRow[] = [
    { k: 'Simulator', v: 'free-ran-ue' },
    { k: 'Instances', v: `${ueCounts.running} / ${ueCounts.total} running`, mono: true },
    ...(row
      ? [
        { k: 'PLMN', v: row.plmnId, mono: true },
        ...(row.gpsi ? [{ k: 'GPSI', v: row.gpsi, mono: true }] : []),
        { k: 'Last deployed', v: row.lastDeployed, mono: true },
      ]
      : [{ k: 'Last deployed', v: nodes.ue.lastDeployed, mono: true }]),
  ]
  const rowPending = row ? (ue.pendingInstances[row.ueId] ?? false) : false

  return (
    <AppLayout
      crumbs={[{ label: 'UE instances' }]}
      overlay={<NotificationContainer errors={errors} successes={successes} onClose={removeNotification} />}
    >
      <section className={styles.pageHead}>
        <h1 className={styles.title}>free-ue</h1>
        <span className={styles.subtitle}>free-ran-ue UE simulator · one container per subscriber</span>
        <div className={styles.headActions}>
          <button type="button" className={styles.btn} onClick={() => navigate('/core-network/subscribers')}>Manage subscribers</button>
          <button type="button" className={styles.btn} onClick={viewLogs}>View logs</button>
        </div>
      </section>

      <section className={styles.meta} aria-label="UE status">
        <div><span>Running</span><b><StatusTag status={nodes.ue.status} text={`${ueCounts.running} / ${ueCounts.total}`} /></b></div>
        <div><span>Starting</span><b><span className="num">{ueCounts.starting}</span></b></div>
        <div><span>Unhealthy</span><b><span className="num">{ueCounts.unhealthy}</span></b></div>
        <div><span>gNB</span><b><StatusTag status={nodes.gnb.status} /></b></div>
      </section>

      <div className={styles.dash}>
        <div className={styles.dashMain}>
          <UeInstancesPanel
            rows={ue.rows}
            isLoading={ue.isLoadingSubscribers}
            loadError={ue.subscribersError}
            pendingInstances={ue.pendingInstances}
            selectedUe={selectedUe}
            onSelectUe={setSelectedUe}
            onDeploy={handleDeploy}
            onStop={handleStop}
            onStopAll={handleStopAll}
            onOpenTerminal={setTerminal}
            deployBlockedReason={deployBlocked}
          />
        </div>

        <SelectedNodePanel
          name={row?.ueId ?? 'UE'}
          status={row?.status ?? nodes.ue.status}
          statusText={row ? undefined : nodes.ue.statusLabel}
          rows={details}
          hint={!row || row.status === 'stopped' ? deployBlocked : undefined}
          actions={row ? (
            <>
              {row.status !== 'stopped' ? (
                <button type="button" className={`${styles.btn} ${styles.btnDanger}`} onClick={() => handleStop(row.ueId)} disabled={rowPending}>
                  {rowPending ? 'Stopping…' : 'Stop UE'}
                </button>
              ) : (
                <button type="button" className={`${styles.btn} ${styles.btnPrimary}`} onClick={() => handleDeploy(row.ueId)} disabled={rowPending || Boolean(deployBlocked)}>
                  {rowPending ? 'Deploying…' : 'Deploy UE'}
                </button>
              )}
              {row.status === 'running' && (
                <button type="button" className={styles.btn} onClick={() => setTerminal(row.ueId)}>Terminal</button>
              )}
            </>
          ) : (
            lab.ueUp && (
              <button type="button" className={`${styles.btn} ${styles.btnDanger}`} onClick={handleStopAll} disabled={anyPending}>
                {anyPending ? 'Stopping…' : 'Stop all'}
              </button>
            )
          )}
        />
      </div>

      <UeTerminalModal instance={terminal} onClose={() => setTerminal(null)} />
    </AppLayout>
  )
}
