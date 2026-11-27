import type { ReactNode } from 'react'
import type { NodeStatus } from '../types'
import { NodeIcon, type IconName } from '../canvas/icons'
import { StatusTag } from '../StatusTag'
import { Tabs } from '../LabUi'
import styles from './detail-header.module.css'

interface Props<T extends string> {
  icon: IconName
  name: string
  sub?: string
  status?: NodeStatus
  statusText?: string
  // every action on this one item, in one row
  actions?: ReactNode
  tab?: T
  tabs?: { value: T; label: ReactNode }[]
  onTab?: (t: T) => void
}

// DetailHeader tops a detail pane and stays put while it scrolls: what the
// item is, its status, every action on it, and the pane's tabs.
export default function DetailHeader<T extends string>({ icon, name, sub, status, statusText, actions, tab, tabs, onTab }: Props<T>) {
  return (
    <header className={styles.head}>
      <div className={styles.top}>
        <span className={styles.icon}><svg width="27" height="27" viewBox="0 0 24 24" aria-hidden="true"><NodeIcon name={icon} x={0} y={0} size={24} /></svg></span>
        <div className={styles.title}>
          <h2>{name}</h2>
          {sub && <span>{sub}</span>}
        </div>
        {status && <StatusTag status={status} text={statusText} />}
      </div>
      {actions && <div className={styles.actions}>{actions}</div>}
      {tabs && tab && onTab && <Tabs value={tab} tabs={tabs} onChange={onTab} />}
    </header>
  )
}
