import { useCallback, useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import AppLayout from '../../components/layout/AppLayout'
import { api, extractErrorMessage } from '../../apiClient'
import type { ImageInfo } from '../../api'
import { useFree5gcStatus } from '../dashboard/useFree5gcStatus'
import { useGnbStatus } from '../dashboard/useGnbStatus'
import { useGnbSliceStatus } from '../dashboard/useGnbSliceStatus'
import { getStatusMeta } from '../dashboard/statusMeta'
import type { NodeStatus } from '../dashboard/types'
import { useUeInstances } from '../dashboard/useUeInstances'
import { detectFree5gcTemplate } from '../dashboard/free5gcTemplate'
import styles from './logs-page.module.css'

type LogTargetKind = 'core' | 'gnb' | 'gnb-slice1' | 'gnb-slice2' | 'ue'

interface LogTarget {
  kind: LogTargetKind
  id: string
  label: string
}

// level words and [TAG]s in a log line, coloured like the prototype viewer
const TOKEN = /(\b(?:INFO|WARN|WARNING|ERRO|ERROR|FATA|FATAL|DEBU|DEBUG|TRAC|TRACE)\b|\[[A-Za-z0-9_.\-]+\])/g
const LEVEL_CLASS: Record<string, string> = { INF: 'lvInfo', WAR: 'lvWarn', ERR: 'lvErr', FAT: 'lvErr', DEB: 'lvDebug', TRA: 'lvDebug' }
// a leading timestamp (ISO or time of day), muted
const STAMP = /^(\S*\d{2}:\d{2}:\d{2}\S*)/

function LogLine({ line, no }: { line: string; no: number }) {
  const stamp = line.match(STAMP)?.[1] ?? ''
  const rest = line.slice(stamp.length)
  const parts = rest.split(TOKEN)
  return (
    <div className={styles.ln}>
      <span className={styles.no}>{no}</span>
      <span className={styles.msg}>
        {stamp && <span className={styles.stamp}>{stamp}</span>}
        {parts.map((part, i) => {
          if (i % 2 === 0) return part
          const word = part.replace(/[[\]]/g, '')
          const level = LEVEL_CLASS[word.slice(0, 3).toUpperCase()]
          const isLevel = level && /^(INFO|WARN|WARNING|ERRO|ERROR|FATA|FATAL|DEBU|DEBUG|TRAC|TRACE)$/.test(word)
          return <span key={i} className={isLevel ? styles[level] : styles.cat}>{part}</span>
        })}
      </span>
    </div>
  )
}

function sameTarget(a: LogTarget | null, b: LogTarget): boolean {
  return a !== null && a.kind === b.kind && a.id === b.id
}

export default function LogsPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const ue = useUeInstances()
  const free5gc = useFree5gcStatus()
  const gnb = useGnbStatus()
  const gnbSlice1 = useGnbSliceStatus('slice1')
  const gnbSlice2 = useGnbSliceStatus('slice2')
  // image refs for the viewer's head bar (e.g. free5gc/amf:latest)
  const [images, setImages] = useState<ImageInfo[]>([])
  useEffect(() => {
    api.imageList().then((response) => setImages(response.data.images ?? [])).catch(() => {})
  }, [])

  // free5GC's project has one compose service per NF - let the viewer pick
  // one instead of always seeing every NF's output interleaved.
  const nfOptions = useMemo(() => free5gc.networkFunctions.map((nf) => nf.name), [free5gc.networkFunctions])
  const [selectedNf, setSelectedNf] = useState<string | null>(null)

  // only subscribers that have actually been deployed at least once have
  // real logs on disk - the backend errors on a never-deployed instance.
  const deployedUeRows = useMemo(() => ue.rows.filter((row) => row.hasBeenDeployed), [ue.rows])

  // gnb-slice1/gnb-slice2 are independent, concurrently deployable gNBs
  // under the ulcl-2slice template - shown instead of the single "gNB"
  // entry, same condition the Dashboard's canvas uses.
  const isTwoSliceCore = detectFree5gcTemplate(free5gc.networkFunctions) === 'ulcl-2slice'

  const targets: LogTarget[] = useMemo(() => [
    { kind: 'core', id: 'core', label: 'free5GC core' },
    ...(isTwoSliceCore
      ? [
        { kind: 'gnb-slice1' as const, id: 'gnb-slice1', label: 'gNB Slice 1' },
        { kind: 'gnb-slice2' as const, id: 'gnb-slice2', label: 'gNB Slice 2' },
      ]
      : [{ kind: 'gnb' as const, id: 'gnb', label: 'gNB' }]),
    ...deployedUeRows.map((row): LogTarget => ({ kind: 'ue', id: row.ueId, label: row.ueId })),
  ], [deployedUeRows, isTwoSliceCore])

  const [selected, setSelected] = useState<LogTarget | null>(null)
  const [lines, setLines] = useState<string[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [fetchedAt, setFetchedAt] = useState<Date | null>(null)

  const fetchLogs = useCallback(async (target: LogTarget, nf: string | null) => {
    setIsLoading(true)
    setError(null)
    try {
      const response = target.kind === 'core'
        ? await api.deployFree5gcLogs(nf ? nf.toLowerCase() : undefined)
        : target.kind === 'gnb'
          ? await api.deployGnbLogs()
          : target.kind === 'gnb-slice1'
            ? await api.deployGnbSliceLogs('slice1')
            : target.kind === 'gnb-slice2'
              ? await api.deployGnbSliceLogs('slice2')
              : await api.deployUeLogs(target.id)
      setLines(response.data.lines ?? [])
      setFetchedAt(new Date())
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
      : requestedKind === 'core' || requestedKind === 'gnb' || requestedKind === 'gnb-slice1' || requestedKind === 'gnb-slice2'
        ? targets.find((target) => target.kind === requestedKind)
        : undefined

    setSelected(match ?? targets[0])
  }, [targets, selected, searchParams, ue.isLoadingSubscribers, ue.isLoadingInstances])

  useEffect(() => {
    if (selected) fetchLogs(selected, selected.kind === 'core' ? selectedNf : null)
  }, [selected, selectedNf, fetchLogs])

  function statusOf(target: LogTarget): NodeStatus {
    switch (target.kind) {
      case 'core': return free5gc.node.status
      case 'gnb': return gnb.node.status
      case 'gnb-slice1': return gnbSlice1.node.status
      case 'gnb-slice2': return gnbSlice2.node.status
      default: return deployedUeRows.find((row) => row.ueId === target.id)?.status ?? 'stopped'
    }
  }

  // what the head bar names: the NF / gNB / UE, and its image
  const headName = !selected ? ''
    : selected.kind === 'core' ? (selectedNf ? selectedNf.toLowerCase() : 'free5gc')
      : selected.kind === 'ue' ? selected.id : selected.kind
  const imageKey = !selected ? ''
    : selected.kind === 'core' ? (selectedNf ? selectedNf.toLowerCase() : '')
      : 'free-ran-ue'
  const headImage = images.find((image) => image.key === imageKey)?.image
    ?? (selected?.kind === 'core' && !selectedNf ? 'all network functions' : '')

  function handleDownload() {
    const blob = new Blob([lines.join('\n') + '\n'], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    const stamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19)
    a.href = url
    a.download = `${headName || 'logs'}-${stamp}.txt`
    document.body.appendChild(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(url)
  }

  function sourceButton(target: LogTarget) {
    const on = sameTarget(selected, target)
    return (
      <li key={`${target.kind}-${target.id}`}>
        <button
          type="button"
          className={`${styles.tgt} ${on ? styles.tgtOn : ''}`}
          aria-current={on ? 'true' : undefined}
          onClick={() => handleSelect(target)}
          title={getStatusMeta(statusOf(target)).text}
        >
          <span className={styles.dot} style={{ background: getStatusMeta(statusOf(target)).dot }} aria-hidden="true" />
          {target.kind === 'ue' ? <span className={styles.tgtMono}>{target.label}</span> : target.label}
          {target.kind === 'core' && free5gc.networkFunctions.length > 0 && (
            <span className={`num ${styles.tgtCount}`}>{free5gc.networkFunctions.length}</span>
          )}
        </button>
      </li>
    )
  }

  function handleSelect(target: LogTarget) {
    setSelected(target)
    setSelectedNf(null)
    setSearchParams(target.kind === 'ue' ? { target: 'ue', instance: target.id } : { target: target.kind })
  }

  return (
    <AppLayout crumbs={[{ label: 'Logs' }]}>
      <section className={styles.header}>
        <h1 className={styles.title}>Logs</h1>
        <span className={styles.subtitle}>Container logs for the core, gNB and each deployed UE instance</span>
        <div className={styles.headerActions}>
          <button type="button" className={styles.btn} onClick={handleDownload} disabled={isLoading || lines.length === 0}>
            Download
          </button>
          <button
            type="button"
            className={styles.btn}
            onClick={() => selected && fetchLogs(selected, selected.kind === 'core' ? selectedNf : null)}
            disabled={!selected || isLoading}
          >
            {isLoading ? 'Refreshing…' : 'Refresh'}
          </button>
        </div>
      </section>

      <div className={styles.body}>
        <nav className={styles.sources} aria-label="Log sources">
          <div className={styles.ph}><h2>Sources</h2></div>
          <ul className={styles.list}>
            {targets.filter((target) => target.kind !== 'ue').map(sourceButton)}
          </ul>
          <h3 className={styles.groupLabel}>UE instances</h3>
          {deployedUeRows.length === 0 ? (
            <p className={styles.emptyTargets}>No UE instances deployed yet.</p>
          ) : (
            <ul className={`${styles.list} ${styles.listTight}`}>
              {targets.filter((target) => target.kind === 'ue').map(sourceButton)}
            </ul>
          )}
        </nav>

        <div className={styles.viewerColumn}>
          {selected?.kind === 'core' && nfOptions.length > 0 && (
            <div className={styles.seg} role="group" aria-label="Network function">
              <button
                type="button"
                className={`${styles.segBtn} ${selectedNf === null ? styles.segOn : ''}`}
                aria-pressed={selectedNf === null}
                onClick={() => setSelectedNf(null)}
              >
                All NFs
              </button>
              {nfOptions.map((nf) => (
                <button
                  key={nf}
                  type="button"
                  className={`${styles.segBtn} ${selectedNf === nf ? styles.segOn : ''}`}
                  aria-pressed={selectedNf === nf}
                  onClick={() => setSelectedNf(nf)}
                >
                  {nf}
                </button>
              ))}
            </div>
          )}

          <section className={styles.viewerFrame} aria-label="Log output">
            {selected && (
              <div className={styles.viewerHead}>
                <span className={styles.viewerSource}>{headName}</span>
                {headImage && <span>{headImage}</span>}
                <span className={styles.viewerMeta}>
                  {lines.length.toLocaleString()} line{lines.length === 1 ? '' : 's'}
                  {fetchedAt && <> · fetched {fetchedAt.toISOString().slice(11, 19)} UTC</>}
                </span>
              </div>
            )}
            <div className={styles.viewer}>
              {isLoading ? (
                <div className={styles.note}>Loading…</div>
              ) : error ? (
                <div className={styles.note}>{error}</div>
              ) : lines.length === 0 ? (
                <div className={styles.note}>No log output yet.</div>
              ) : (
                lines.map((line, index) => <LogLine key={index} no={index + 1} line={line} />)
              )}
            </div>
          </section>
        </div>
      </div>
    </AppLayout>
  )
}
