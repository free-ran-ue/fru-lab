import { useRef, useState, type PointerEvent, type ReactNode } from 'react'
import { useStoredState } from '../useProfiles'
import styles from './split.module.css'

interface Props {
  // remembers the user's split per view
  storageKey: string
  left: ReactNode
  right: ReactNode
  leftLabel: string
  rightLabel: string
  // the left pane's share before the user drags (and on double-click)
  defaultRatio?: number
}

const MIN = 0.2
const MAX = 0.8

// Split puts a list on the left and its detail on the right (defaultRatio
// to start, 4:6 unless the view says otherwise);
// drag the divider to resize (double-click resets it).
export default function Split({ storageKey, left, right, leftLabel, rightLabel, defaultRatio = 0.4 }: Props) {
  const [ratio, setRatio] = useStoredState(storageKey, defaultRatio)
  const [dragging, setDragging] = useState(false)
  const wrap = useRef<HTMLDivElement>(null)

  function onMove(event: PointerEvent<HTMLDivElement>) {
    if (!dragging || !wrap.current) return
    const r = wrap.current.getBoundingClientRect()
    setRatio(Math.min(MAX, Math.max(MIN, (event.clientX - r.left) / r.width)))
  }

  return (
    <div ref={wrap} className={`${styles.split} ${dragging ? styles.dragging : ''}`} style={{ gridTemplateColumns: `${ratio}fr 0 ${1 - ratio}fr` }}>
      <section className={styles.left} aria-label={leftLabel}>{left}</section>
      <div
        className={styles.divider}
        role="separator"
        aria-orientation="vertical"
        aria-valuenow={Math.round(ratio * 100)}
        title="Drag to resize · double-click to reset"
        onPointerDown={(e) => { e.currentTarget.setPointerCapture(e.pointerId); setDragging(true) }}
        onPointerMove={onMove}
        onPointerUp={() => setDragging(false)}
        onPointerCancel={() => setDragging(false)}
        onDoubleClick={() => setRatio(defaultRatio)}
      />
      <section className={styles.right} aria-label={rightLabel}>{right}</section>
    </div>
  )
}
