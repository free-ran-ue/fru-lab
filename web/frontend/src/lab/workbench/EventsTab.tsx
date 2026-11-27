import { useEffect, useMemo, useRef, useState } from 'react'
import { useEvents } from '../console/EventsContext'
import styles from './console.module.css'

const clock = (t: number) => new Date(t).toISOString().slice(11, 23)

// EventsTab lists the lab events parsed from every running component's log,
// oldest first. Clicking one selects its node on the topology.
export default function EventsTab({ selectedId, onSelectNode }: { selectedId: string | null; onSelectNode: (id: string) => void }) {
  const { events, clear } = useEvents()
  const [onlySelected, setOnlySelected] = useState(false)
  const [errorsOnly, setErrorsOnly] = useState(false)
  const box = useRef<HTMLDivElement>(null)
  const pinned = useRef(true)

  const shown = useMemo(() => events.filter((e) => (!errorsOnly || e.level === 'error')
    && (!onlySelected || !selectedId || e.node === selectedId || (selectedId.startsWith('ue:') && e.ue?.id === selectedId.slice(3)))), [events, errorsOnly, onlySelected, selectedId])

  useEffect(() => {
    if (pinned.current && box.current) box.current.scrollTop = box.current.scrollHeight
  }, [shown])

  return (
    <>
      <div className={styles.tools}>
        <label className={styles.check}><input type="checkbox" checked={onlySelected} onChange={(e) => setOnlySelected(e.target.checked)} />selected node only</label>
        <label className={styles.check}><input type="checkbox" checked={errorsOnly} onChange={(e) => setErrorsOnly(e.target.checked)} />errors only</label>
        <button type="button" className={styles.ctlBtn} onClick={clear}>clear</button>
        <span className={styles.status}>{shown.length} events · parsed from AMF, gNB and UE logs</span>
      </div>
      <div className={styles.lines} ref={box} onScroll={() => { const b = box.current; if (b) pinned.current = b.scrollTop + b.clientHeight >= b.scrollHeight - 20 }}>
        {shown.length === 0 && <p className={styles.empty}>No events yet. They appear as the core, gNB and UEs come up.</p>}
        {shown.map((e) => {
          const target = e.ue ? `ue:${e.ue.id}` : e.node
          return (
            <button key={e.id} type="button" className={`${styles.event} ${target === selectedId ? styles.eventOn : ''}`} onClick={() => onSelectNode(target)}>
              <span className={styles.ts}>{clock(e.t)}</span>
              <span className={`${styles.lvl} ${styles[`lvl_${e.level}`]}`} aria-label={e.level} />
              <span className={styles.node}>{e.ue ? e.ue.id : e.node === 'core' ? 'AMF' : e.node}</span>
              <span className={e.level === 'error' ? styles.lvErr : undefined}>{e.text}</span>
            </button>
          )
        })}
      </div>
    </>
  )
}
