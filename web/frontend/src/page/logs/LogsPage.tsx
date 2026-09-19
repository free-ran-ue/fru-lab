import { useCallback, useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import Sidebar from '../../components/sidebar/Sidebar'
import Button from '../../components/button/button'
import { api, extractErrorMessage } from '../../apiClient'
import { useFree5gcStatus } from '../dashboard/useFree5gcStatus'
import { useUeInstances } from '../dashboard/useUeInstances'
import styles from './logs-page.module.css'

type LogTargetKind = 'core' | 'gnb' | 'ue'

interface LogTarget {
  kind: LogTargetKind
  id: string
  label: string
}

function sameTarget(a: LogTarget | null, b: LogTarget): boolean {
  return a !== null && a.kind === b.kind && a.id === b.id
}

export default function LogsPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const ue = useUeInstances()
  const free5gc = useFree5gcStatus()

  // free5GC's project has one compose service per NF - let the viewer pick
  // one instead of always seeing every NF's output interleaved.
  const nfOptions = useMemo(() => free5gc.networkFunctions.map((nf) => nf.name), [free5gc.networkFunctions])
  const [selectedNf, setSelectedNf] = useState<string | null>(null)

  // only subscribers that have actually been deployed at least once have
  // real logs on disk - the backend errors on a never-deployed instance.
  const deployedUeRows = useMemo(() => ue.rows.filter((row) => row.hasBeenDeployed), [ue.rows])

  const targets: LogTarget[] = useMemo(() => [
    { kind: 'core', id: 'core', label: 'free5GC Core' },
    { kind: 'gnb', id: 'gnb', label: 'gNB' },
    ...deployedUeRows.map((row): LogTarget => ({ kind: 'ue', id: row.ueId, label: `UE — ${row.ueId}` })),
  ], [deployedUeRows])

  const [selected, setSelected] = useState<LogTarget | null>(null)
  const [lines, setLines] = useState<string[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const fetchLogs = useCallback(async (target: LogTarget, nf: string | null) => {
    setIsLoading(true)
    setError(null)
    try {
      const response = target.kind === 'core'
        ? await api.deployFree5gcLogs(nf ? nf.toLowerCase() : undefined)
        : target.kind === 'gnb'
          ? await api.deployGnbLogs()
          : await api.deployUeLogs(target.id)
      setLines(response.data.lines ?? [])
    } catch (err) {
      setError(extractErrorMessage(err, 'Failed to load logs'))
    } finally {
      setIsLoading(false)
    }
  }, [])

  // resolve the initial selection from ?target=&instance= (set when a
  // Dashboard "Logs" button deep-links here) once the target list is known.
  // Must wait for BOTH the subscriber fetch and the deploy-status fetch to
  // finish - `targets` only includes a ue row once both have resolved, and
  // resolving early locks onto the wrong default (core) permanently, since
  // `selected` becoming non-null blocks this effect from ever re-running.
  useEffect(() => {
    if (selected || targets.length === 0 || ue.isLoadingSubscribers || ue.isLoadingInstances) return

    const requestedKind = searchParams.get('target')
    const requestedInstance = searchParams.get('instance')
    const match = requestedKind === 'ue' && requestedInstance
      ? targets.find((target) => target.kind === 'ue' && target.id === requestedInstance)
      : requestedKind === 'core' || requestedKind === 'gnb'
        ? targets.find((target) => target.kind === requestedKind)
        : undefined

    setSelected(match ?? targets[0])
  }, [targets, selected, searchParams, ue.isLoadingSubscribers, ue.isLoadingInstances])

  useEffect(() => {
    if (selected) fetchLogs(selected, selected.kind === 'core' ? selectedNf : null)
  }, [selected, selectedNf, fetchLogs])

  function handleSelect(target: LogTarget) {
    setSelected(target)
    setSelectedNf(null)
    setSearchParams(target.kind === 'ue' ? { target: 'ue', instance: target.id } : { target: target.kind })
  }

  return (
    <div className={styles.layout}>
      <Sidebar />

      <main className={styles.content}>
        <header className={styles.header}>
          <div>
            <h2 className={styles.title}>Logs</h2>
            <p className={styles.subtitle}>Container logs for the core, gNB, and each deployed UE instance.</p>
          </div>
          <Button
            variant="secondary"
            onClick={() => selected && fetchLogs(selected, selected.kind === 'core' ? selectedNf : null)}
            disabled={!selected || isLoading}
          >
            {isLoading ? 'Refreshing…' : 'Refresh'}
          </Button>
        </header>

        <div className={styles.body}>
          <nav className={styles.targetList}>
            {targets.map((target) => (
              <button
                key={`${target.kind}-${target.id}`}
                className={`${styles.targetItem} ${sameTarget(selected, target) ? styles.targetItemActive : ''}`}
                onClick={() => handleSelect(target)}
              >
                {target.label}
              </button>
            ))}
            {deployedUeRows.length === 0 && (
              <p className={styles.emptyTargets}>No UE instances deployed yet.</p>
            )}
          </nav>

          <div className={styles.viewerColumn}>
            {selected?.kind === 'core' && nfOptions.length > 0 && (
              <div className={styles.nfPicker}>
                <button
                  className={`${styles.nfChip} ${selectedNf === null ? styles.nfChipActive : ''}`}
                  onClick={() => setSelectedNf(null)}
                >
                  All NFs
                </button>
                {nfOptions.map((nf) => (
                  <button
                    key={nf}
                    className={`${styles.nfChip} ${selectedNf === nf ? styles.nfChipActive : ''}`}
                    onClick={() => setSelectedNf(nf)}
                  >
                    {nf}
                  </button>
                ))}
              </div>
            )}

            <div className={styles.viewer}>
              {isLoading ? (
                <div className={styles.line}>Loading…</div>
              ) : error ? (
                <div className={styles.line}>{error}</div>
              ) : lines.length === 0 ? (
                <div className={styles.line}>No log output yet.</div>
              ) : (
                lines.map((line, index) => <div key={index} className={styles.line}>{line}</div>)
              )}
            </div>
          </div>
        </div>
      </main>
    </div>
  )
}
