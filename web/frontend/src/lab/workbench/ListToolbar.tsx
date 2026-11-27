import type { ReactNode } from 'react'
import { Button, Segmented, ui } from '../LabUi'
import styles from './list-toolbar.module.css'

interface Props {
  // row 1, left: create
  onNew: () => void
  bulk: ReactNode
  find: string
  onFind: (v: string) => void
  findLabel: string
  // row 1, right: which rows show
  show: 'all' | 'running'
  onShow: (v: 'all' | 'running') => void
  counts: { all: number; running: number }
  // row 2, left: what to do with the ticked rows
  ticked: number
  deploySelected: ReactNode
  onDeleteSelected: () => void
  deleting: boolean
  // row 2, right: the whole list at once
  startAll: ReactNode
  onStopAll: () => void
  stopAll: { disabled?: boolean; busy?: boolean; title?: string }
}

// ListToolbar is the gNB and UE lists' one toolbar, laid out the same on
// both pages:
//   + New · Bulk create · find ............................ All | Running
//   Deploy selected ▾ · Delete selected ......... Start all stopped ▾ · Stop all
export default function ListToolbar(p: Props) {
  return (
    <div className={styles.bar}>
      <div className={styles.row}>
        <Button kind="primary" small onClick={p.onNew}>+ New</Button>
        {p.bulk}
        <input className={`${ui.select} ${styles.find}`} placeholder="find" value={p.find} onChange={(e) => p.onFind(e.target.value)} aria-label={p.findLabel} />
        <span className={styles.push} />
        <Segmented value={p.show} options={[{ value: 'all', label: `All ${p.counts.all}` }, { value: 'running', label: `Running ${p.counts.running}` }]} onChange={p.onShow} />
      </div>
      <div className={styles.row}>
        {p.ticked > 0
          ? (
            <>
              {p.deploySelected}
              <Button kind="danger" small busy={p.deleting} onClick={p.onDeleteSelected}>Delete selected ({p.ticked})</Button>
            </>
          )
          : <span className={styles.hint}>Tick rows to deploy or delete several</span>}
        <span className={styles.push} />
        {p.startAll}
        <Button kind="danger" small disabled={p.stopAll.disabled} busy={p.stopAll.busy} title={p.stopAll.title} onClick={p.onStopAll}>Stop all</Button>
      </div>
    </div>
  )
}
