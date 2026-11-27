import { Fragment, useEffect, useRef, useState, type DragEvent, type PointerEvent as ReactPointerEvent } from 'react'
import { useStoredState } from '../useProfiles'
import TerminalTab from './TerminalTab'
import styles from './terminal-dock.module.css'

interface Props {
  // the UE instances with a terminal open, in tab order
  sessions: string[]
  active: string | null
  onActive: (id: string) => void
  onClose: (id: string) => void
  onCloseAll: () => void
  // the name a session's tab shows
  labelOf: (id: string) => string
}

const MAX_PANES = 4
const MIN_SHARE = 0.12
const DRAG_TYPE = 'application/x-frulab-terminal'

type Drop = { pane: number; side: 'left' | 'right' | 'center' } | null

// TerminalDock slides up under the Overview topology with one tab per UE
// shell. Drag a tab onto the left or right half of a pane to split the
// dock (up to four side by side), or onto a pane's middle to show it
// there; drag the dividers to resize the panes and the top edge to resize
// the dock. Every shell is mounted once and only moves between grid
// columns, so splitting never drops a connection. A shell that ends
// (exit) closes its own tab and pane.
export default function TerminalDock({ sessions, active, onActive, onClose, onCloseAll, labelOf }: Props) {
  // which session each pane shows, left to right, and each pane's share
  const [panes, setPanes] = useState<string[]>([])
  const [shares, setShares] = useState<number[]>([])
  const [focus, setFocus] = useState(0)
  const [drop, setDrop] = useState<Drop>(null)
  // a tab is being dragged: the panes become drop targets
  const [dragging, setDragging] = useState(false)
  const [height, setHeight] = useStoredState('frulab-terminal-height', 320)
  const body = useRef<HTMLDivElement>(null)

  // keep the panes in step with the open sessions: drop closed ones, show
  // the active one, always at least one pane while anything is open
  useEffect(() => {
    let next = panes.filter((id) => sessions.includes(id))
    if (next.length === 0 && sessions.length) next = [active && sessions.includes(active) ? active : sessions[0]]
    if (active && sessions.includes(active) && !next.includes(active)) {
      const f = Math.min(focus, next.length - 1)
      next = next.map((id, i) => (i === f ? active : id))
    }
    if (next.join() !== panes.join()) {
      setPanes(next)
      setShares(next.map(() => 1 / next.length))
      setFocus((f) => Math.min(f, Math.max(0, next.length - 1)))
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sessions, active])

  if (sessions.length === 0) return null

  // ----- splitting by drag and drop -----
  function sideOf(e: DragEvent<HTMLDivElement>): 'left' | 'right' | 'center' {
    const r = e.currentTarget.getBoundingClientRect()
    const x = (e.clientX - r.left) / r.width
    return x < 0.3 ? 'left' : x > 0.7 ? 'right' : 'center'
  }

  function onDrop(e: DragEvent<HTMLDivElement>, pane: number) {
    e.preventDefault()
    const id = e.dataTransfer.getData(DRAG_TYPE)
    const side = sideOf(e)
    setDrop(null)
    setDragging(false)
    if (!id || !sessions.includes(id)) return
    const without = panes.filter((p) => p !== id)
    let target = pane - (panes.indexOf(id) !== -1 && panes.indexOf(id) < pane ? 1 : 0)
    let next: string[]
    if (side === 'center' || without.length >= MAX_PANES) {
      next = [...without]
      target = Math.min(Math.max(0, target), Math.max(0, next.length - 1))
      if (next.length === 0) next = [id]
      else next[target] = id
    } else {
      next = [...without]
      next.splice(Math.max(0, side === 'left' ? target : target + 1), 0, id)
    }
    setPanes(next)
    setShares(next.map(() => 1 / next.length))
    setFocus(next.indexOf(id))
    onActive(id)
  }

  function closePane(i: number) {
    const next = panes.filter((_, j) => j !== i)
    setPanes(next)
    setShares(next.map(() => 1 / next.length))
    setFocus(Math.max(0, Math.min(focus, next.length - 1)))
  }

  // ----- resizing -----
  function dragDivider(e: ReactPointerEvent, i: number) {
    e.preventDefault()
    const width = body.current?.getBoundingClientRect().width ?? 1
    const startX = e.clientX
    const start = [...shares]
    const move = (ev: PointerEvent) => {
      const d = (ev.clientX - startX) / width
      const pair = start[i] + start[i + 1]
      const left = Math.min(pair - MIN_SHARE, Math.max(MIN_SHARE, start[i] + d))
      setShares(start.map((s, j) => (j === i ? left : j === i + 1 ? pair - left : s)))
    }
    const up = () => { window.removeEventListener('pointermove', move); window.removeEventListener('pointerup', up) }
    window.addEventListener('pointermove', move)
    window.addEventListener('pointerup', up)
  }

  function dragTop(e: ReactPointerEvent) {
    e.preventDefault()
    const startY = e.clientY
    const startH = height
    const move = (ev: PointerEvent) => setHeight(Math.min(window.innerHeight - 160, Math.max(160, startH + startY - ev.clientY)))
    const up = () => { window.removeEventListener('pointermove', move); window.removeEventListener('pointerup', up) }
    window.addEventListener('pointermove', move)
    window.addEventListener('pointerup', up)
  }

  // panes at odd grid columns, dividers between them
  const columns = shares.map((s) => `${s}fr`).join(' 6px ')
  const shown = panes[focus]

  return (
    <section className={styles.dock} style={{ height }} aria-label="UE terminals">
      <div className={styles.resize} onPointerDown={dragTop} role="separator" aria-orientation="horizontal" aria-label="Resize the terminals" />
      <div className={styles.bar} role="tablist">
        {sessions.map((id) => (
          <div
            key={id}
            className={styles.tab}
            role="tab"
            aria-selected={id === shown}
            draggable
            title="Drag onto a pane's left or right half to split"
            onDragStart={(e) => { e.dataTransfer.setData(DRAG_TYPE, id); e.dataTransfer.effectAllowed = 'move'; setDragging(true) }}
            onDragEnd={() => { setDrop(null); setDragging(false) }}
          >
            <button type="button" className={styles.tabName} onClick={() => onActive(id)}>{labelOf(id)}</button>
            <button type="button" className={styles.close} onClick={() => onClose(id)} aria-label={`Close the terminal of ${labelOf(id)}`}>×</button>
          </div>
        ))}
        <span className={styles.hint}>drag a tab onto a pane to split</span>
        <button type="button" className={styles.closeAll} onClick={onCloseAll}>close all</button>
      </div>
      <div ref={body} className={styles.body} style={{ gridTemplateColumns: columns }}>
        {panes.map((id, i) => (
          <Fragment key={`slot-${i}`}>
            {i > 0 && <div className={styles.divider} style={{ gridColumn: 2 * i }} onPointerDown={(e) => dragDivider(e, i - 1)} role="separator" aria-orientation="vertical" />}
            <div
              className={`${styles.paneHead} ${i === focus ? styles.paneOn : ''}`}
              style={{ gridColumn: 2 * i + 1 }}
              onClick={() => { setFocus(i); onActive(id) }}
            >
              <span>{labelOf(id)}</span>
              {panes.length > 1 && <button type="button" className={styles.close} onClick={(e) => { e.stopPropagation(); closePane(i) }} aria-label="Close this pane">×</button>}
            </div>
            {dragging && (
            <div
              className={`${styles.drop} ${drop?.pane === i ? styles[`drop_${drop.side}`] : ''}`}
              style={{ gridColumn: 2 * i + 1 }}
              onDragOver={(e) => { if (e.dataTransfer.types.includes(DRAG_TYPE)) { e.preventDefault(); setDrop({ pane: i, side: sideOf(e) }) } }}
              onDragLeave={() => setDrop(null)}
              onDrop={(e) => onDrop(e, i)}
            />
            )}
          </Fragment>
        ))}
        {/* every shell, mounted once; only its grid column changes */}
        {sessions.map((id) => {
          const i = panes.indexOf(id)
          return (
            <div key={id} className={styles.pane} hidden={i === -1} style={i === -1 ? undefined : { gridColumn: 2 * i + 1 }} onMouseDown={() => { if (i !== -1) setFocus(i) }}>
              <TerminalTab instance={id} onExit={() => onClose(id)} />
            </div>
          )
        })}
      </div>
    </section>
  )
}
