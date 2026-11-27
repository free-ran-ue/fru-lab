import { useState } from 'react'
import AppLayout from '../../components/layout/AppLayout'
import { useEvents } from '../../lab/console/EventsContext'
import EventsTab from '../../lab/workbench/EventsTab'
import Workbench from '../../lab/workbench/Workbench'
import styles from './overview-pages.module.css'

// EventsPage is the Overview's event timeline: what the core, the gNBs and
// the UEs did, parsed from their logs. Click an event to narrow the list
// to its node.
export default function EventsPage() {
  const { events } = useEvents()
  const [node, setNode] = useState<string | null>(null)
  const errors = events.filter((e) => e.level === 'error').length
  return (
    <AppLayout crumbs={[{ label: 'Events' }]} fill>
      <Workbench
        title="Events"
        status={<><span>events <b>{events.length}</b></span><span>errors <b>{errors}</b></span>{node && <span>node <b>{node.replace(/^ue:/, '')}</b></span>}</>}
        body={<div className={styles.panel}><EventsTab selectedId={node} onSelectNode={(id) => setNode(id === node ? null : id)} /></div>}
      />
    </AppLayout>
  )
}
