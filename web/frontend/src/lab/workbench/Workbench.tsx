import type { ReactNode } from 'react'
import { useLab } from '../LabContext'
import { useEvents } from '../console/EventsContext'
import type { LogSource } from '../console/stream'
import Console, { type ConsoleState } from './Console'
import type { SourceOption } from './LogsTab'
import styles from './workbench.module.css'

interface Props {
  // the toolbar: view name and status on the left, actions on the right
  title: ReactNode
  status?: ReactNode
  actions?: ReactNode
  canvas: ReactNode
  inspector: ReactNode
  console: ConsoleState
  selectedId: string | null
  onSelect: (id: string | null) => void
}

// logSourceFor is where a canvas node's logs come from.
export function logSourceFor(id: string | null): LogSource | null {
  if (!id) return null
  if (id === 'core') return { target: 'free5gc' }
  if (id.startsWith('nf:')) return { target: 'free5gc', service: id.slice(3).toLowerCase() }
  if (id === 'gnb' || id === 'gnb-slice1' || id === 'gnb-slice2') return { target: id }
  if (id.startsWith('ue:')) return { target: 'ue', instance: id.slice(3) }
  return null
}

// Workbench lays out a lab view: toolbar, topology, inspector on the right
// and the console underneath.
export default function Workbench({ title, status, actions, canvas, inspector, console: con, selectedId, onSelect }: Props) {
  const lab = useLab()
  const { ueLive } = useEvents()

  const options: SourceOption[] = []
  if (lab.coreUp) {
    options.push({ label: 'free5GC · all NFs', source: { target: 'free5gc' } })
    for (const nf of lab.free5gc.networkFunctions) options.push({ label: `free5GC · ${nf.name}`, source: { target: 'free5gc', service: nf.name.toLowerCase() } })
  }
  if (lab.gnb.node.status !== 'stopped') options.push({ label: 'gNB', source: { target: 'gnb' } })
  if (lab.gnbSlice1.node.status !== 'stopped') options.push({ label: 'gNB slice1', source: { target: 'gnb-slice1' } })
  if (lab.gnbSlice2.node.status !== 'stopped') options.push({ label: 'gNB slice2', source: { target: 'gnb-slice2' } })
  for (const r of lab.ue.rows) if (r.hasBeenDeployed) options.push({ label: `UE ${r.ueId}`, source: { target: 'ue', instance: r.ueId } })

  const ueId = selectedId?.startsWith('ue:') ? selectedId.slice(3) : null
  const ueRow = ueId ? lab.ue.rows.find((r) => r.ueId === ueId) : null
  const live = ueId ? ueLive[ueId] : undefined
  const ue = ueRow && ueRow.status === 'running' ? { id: ueRow.ueId, tun: live?.tun, ready: Boolean(live?.ip) || live?.stage === 'ready' } : null

  function select(id: string | null) {
    onSelect(id)
    const src = logSourceFor(id)
    if (src) con.setSource(src)
  }

  return (
    <div className={styles.wb}>
      <div className={styles.toolbar}>
        <h1 className={styles.title}>{title}</h1>
        {status && <div className={styles.status}>{status}</div>}
        <div className={styles.actions}>{actions}</div>
      </div>
      <div className={styles.canvas}>{canvas}</div>
      <aside className={styles.inspector} aria-label="Inspector">{inspector}</aside>
      <div className={styles.console}>
        <Console state={con} options={options} selectedId={selectedId} onSelectNode={select} ue={ue} />
      </div>
    </div>
  )
}

export { Workbench }
