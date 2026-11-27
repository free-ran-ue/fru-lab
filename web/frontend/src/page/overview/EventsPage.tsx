import { useMemo, useState } from 'react'
import AppLayout from '../../components/layout/AppLayout'
import { useEvents } from '../../lab/console/EventsContext'
import type { LabEvent } from '../../lab/console/events'
import { clockOf, eventNode, eventSource, levelStatus, nodeLabel } from '../../lab/console/eventSource'
import { useLab } from '../../lab/LabContext'
import { Button, Dot, Segmented, ui } from '../../lab/LabUi'
import { SortTh, useSort } from '../../lab/sort'
import Workbench from '../../lab/workbench/Workbench'
import styles from './events-page.module.css'

// the most rows drawn at once; the search narrows the rest
const MAX_ROWS = 2000
type Level = 'all' | 'error' | 'warn'

// EventsPage is the Overview's event table: everything the cores, the
// gNBs and the UEs did, parsed from their logs since each started. Search
// matches the text, the source and the UE; a source can be picked to see
// only its events; every column sorts (newest first to start).
export default function EventsPage() {
  const lab = useLab()
  const { events, clear } = useEvents()
  const [query, setQuery] = useState('')
  const [level, setLevel] = useState<Level>('all')
  const [node, setNode] = useState<string | null>(null)

  const rows = useMemo(() => {
    const q = query.trim().toLowerCase()
    return events
      .map((e) => ({ e, source: eventSource(lab, e) }))
      .filter(({ e, source }) =>
        (level === 'all' || (level === 'error' ? e.level === 'error' : e.level === 'error' || e.level === 'warn'))
        && (!node || eventNode(e) === node)
        && (!q || e.text.toLowerCase().includes(q) || source.toLowerCase().includes(q) || (e.ue?.id ?? '').includes(q) || (e.ue?.ip ?? '').includes(q)))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [events, query, level, node, lab.gnbs, lab.coreNetworks.networks])

  const { sorted, sort, toggle } = useSort<{ e: LabEvent; source: string }>(rows, {
    time: (r) => r.e.t,
    level: (r) => ['error', 'warn', 'ok', 'info'].indexOf(r.e.level),
    source: (r) => r.source,
    text: (r) => r.e.text,
  }, { key: 'time', dir: 'desc' })
  const shown = sorted.slice(0, MAX_ROWS)

  const counts = { error: events.filter((e) => e.level === 'error').length, warn: events.filter((e) => e.level === 'warn').length }
  const mark = (text: string) => {
    const q = query.trim()
    if (!q) return text
    const i = text.toLowerCase().indexOf(q.toLowerCase())
    if (i < 0) return text
    return <>{text.slice(0, i)}<mark className={styles.mark}>{text.slice(i, i + q.length)}</mark>{text.slice(i + q.length)}</>
  }

  return (
    <AppLayout crumbs={[{ label: 'Events' }]} fill>
      <Workbench
        title="Events"
        status={<><span>events <b>{events.length}</b></span><span>errors <b>{counts.error}</b></span><span>warnings <b>{counts.warn}</b></span></>}
        body={(
          <div className={styles.page}>
            <div className={styles.bar}>
              <input className={`${ui.select} ${styles.search}`} placeholder="search the text, the source, a UE or its IP" value={query} onChange={(e) => setQuery(e.target.value)} aria-label="Search events" autoFocus />
              <Segmented value={level} options={[{ value: 'all', label: 'All' }, { value: 'warn', label: `Warnings+ ${counts.warn + counts.error}` }, { value: 'error', label: `Errors ${counts.error}` }]} onChange={setLevel} />
              {node && (
                <span className={styles.chip}>
                  only {nodeLabel(lab, node)}
                  <button type="button" onClick={() => setNode(null)} aria-label="Show every source">×</button>
                </span>
              )}
              <span className={styles.push} />
              <span className={styles.count}>{rows.length === events.length ? `${rows.length} events` : `${rows.length} of ${events.length} match`}</span>
              <Button small onClick={clear}>Clear</Button>
            </div>
            <div className={styles.scroll}>
              {shown.length === 0
                ? <p className={ui.empty}>{events.length ? 'Nothing matches.' : 'No events yet. They appear as the cores, gNBs and UEs come up.'}</p>
                : (
                  <table className={styles.tbl}>
                    <thead>
                      <tr>
                        <SortTh k="time" sort={sort} onSort={toggle} className={styles.time}>Time (UTC)</SortTh>
                        <SortTh k="level" sort={sort} onSort={toggle} className={styles.level}>Level</SortTh>
                        <SortTh k="source" sort={sort} onSort={toggle} className={styles.source}>Source</SortTh>
                        <SortTh k="text" sort={sort} onSort={toggle}>Event</SortTh>
                      </tr>
                    </thead>
                    <tbody>
                      {shown.map(({ e, source }) => (
                        <tr key={e.id} className={e.level === 'error' ? styles.err : e.level === 'warn' ? styles.warn : undefined}>
                          <td className={styles.time}>{clockOf(e.t)}</td>
                          <td className={styles.level}><Dot status={levelStatus(e.level)} /> {e.level}</td>
                          <td className={styles.source}>
                            <button type="button" className={styles.srcBtn} title="Show only this source" onClick={() => setNode(eventNode(e) === node ? null : eventNode(e))}>{mark(source)}</button>
                          </td>
                          <td>{mark(e.text)}{e.ue?.ip && <span className={styles.ip}> · {mark(e.ue.ip)}</span>}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                )}
              {sorted.length > MAX_ROWS && <p className={ui.empty}>Showing {MAX_ROWS} of {sorted.length}. Search to narrow it down.</p>}
            </div>
          </div>
        )}
      />
    </AppLayout>
  )
}
