import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState, type KeyboardEvent, type PointerEvent, type ReactNode } from 'react'
import type { NodeStatus } from '../types'
import { NodeIcon, type IconName } from './icons'
import styles from './lab-canvas.module.css'

export interface CanvasNode {
  id: string
  x: number
  y: number
  w: number
  h: number
  icon: IconName
  label: string
  sub?: string
  // no status draws a neutral node (the DN)
  status?: NodeStatus
  // drawn faded: a part of the lab that lives on another page
  ghost?: boolean
  // small mono tag under the label (e.g. "3 UE attached")
  tag?: string
  // per-UE dots for a fleet node
  dots?: { id: string; status: NodeStatus }[]
  compact?: boolean
}

export interface CanvasLink {
  id: string
  from: string
  to: string
  d: string
  label?: { x: number; y: number; text: string }
  // packets flow along it
  live?: boolean
  dashed?: boolean
  // a radio link (Uu): drawn dotted with signal waves off the UE end
  radio?: boolean
}

// three arcs fanning out from the start of a path toward its end
function radioWaves(d: string): string {
  const m = /^M([\d.-]+) ([\d.-]+)/.exec(d)
  if (!m) return ''
  const x = Number(m[1]) + 8
  const y = Number(m[2])
  return [5, 9, 13].map((r) => `M${x + r * 0.5} ${y - r}A${r} ${r} 0 0 1 ${x + r * 0.5} ${y + r}`).join('')
}

export interface CanvasFrame {
  id: string
  x: number
  y: number
  w: number
  h: number
  label: string
  sub?: string
  status?: NodeStatus
  ghost?: boolean
}

interface LabCanvasProps {
  nodes: CanvasNode[]
  links: CanvasLink[]
  frames?: CanvasFrame[]
  // free-floating labels (e.g. "SBI")
  notes?: { x: number; y: number; text: string; anchor?: 'start' | 'middle' | 'end' }[]
  selectedId: string | null
  onSelect: (id: string | null) => void
  // a node double-clicked (the Overview opens its terminal)
  onOpen?: (id: string) => void
  // overlay in the top-left corner (a legend)
  overlay?: ReactNode
  label: string
}

interface View { k: number; tx: number; ty: number }

const PAD = 40
const MIN_K = 0.25
// the UI is drawn at 1.5x, so the map may zoom in that much further too
const MAX_K = 3.75
// at most this many links animate packets at once
const MAX_LIVE = 40

function usePrefersReducedMotion(): boolean {
  const [reduced, setReduced] = useState(() => window.matchMedia('(prefers-reduced-motion: reduce)').matches)
  useEffect(() => {
    const m = window.matchMedia('(prefers-reduced-motion: reduce)')
    const on = () => setReduced(m.matches)
    m.addEventListener('change', on)
    return () => m.removeEventListener('change', on)
  }, [])
  return reduced
}

const RING: Record<NodeStatus, string> = {
  running: styles.ringRunning,
  deploying: styles.ringDeploying,
  unhealthy: styles.ringUnhealthy,
  stopped: styles.ringStopped,
}

// LabCanvas is the interactive lab map: drag to pan, wheel or the buttons
// to zoom, hover a node to light up its links, click it for a floating
// card. Running nodes breathe, deploying ones spin, and packets flow along
// links whose both ends are up.
export default function LabCanvas({ nodes, links, frames = [], notes = [], selectedId, onSelect, onOpen, overlay, label }: LabCanvasProps) {
  const wrapRef = useRef<HTMLDivElement>(null)
  const [size, setSize] = useState({ w: 0, h: 0 })
  const [view, setView] = useState<View>({ k: 1, tx: 0, ty: 0 })
  const [hovered, setHovered] = useState<string | null>(null)
  const [panning, setPanning] = useState(false)
  const drag = useRef<{ x: number; y: number; tx: number; ty: number; moved: boolean } | null>(null)
  const reducedMotion = usePrefersReducedMotion()

  // content bounds; the view refits when they change (not on status polls)
  const bounds = useMemo(() => {
    const boxes = [...nodes, ...frames]
    if (boxes.length === 0) return { x: 0, y: 0, w: 1, h: 1 }
    const x0 = Math.min(...boxes.map((b) => b.x))
    const y0 = Math.min(...boxes.map((b) => b.y))
    const x1 = Math.max(...boxes.map((b) => b.x + b.w))
    const y1 = Math.max(...boxes.map((b) => b.y + b.h))
    return { x: x0, y: y0, w: x1 - x0, h: y1 - y0 }
  }, [nodes, frames])
  const boundsKey = `${Math.round(bounds.x)},${Math.round(bounds.y)},${Math.round(bounds.w)},${Math.round(bounds.h)}`

  const fit = useCallback(() => {
    if (!size.w || !size.h) return
    const k = Math.min(MAX_K, Math.max(MIN_K, Math.min((size.w - PAD * 2) / bounds.w, (size.h - PAD * 2) / bounds.h, 1.875)))
    setView({ k, tx: (size.w - bounds.w * k) / 2 - bounds.x * k, ty: (size.h - bounds.h * k) / 2 - bounds.y * k })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [size.w, size.h, boundsKey])

  useLayoutEffect(() => {
    const el = wrapRef.current
    if (!el) return
    const ro = new ResizeObserver(([entry]) => setSize({ w: entry.contentRect.width, h: entry.contentRect.height }))
    ro.observe(el)
    return () => ro.disconnect()
  }, [])

  useEffect(() => { fit() }, [fit])

  // zoom around a point in screen coordinates
  const zoomAt = useCallback((factor: number, sx: number, sy: number) => {
    setView((v) => {
      const k = Math.min(MAX_K, Math.max(MIN_K, v.k * factor))
      return { k, tx: sx - (sx - v.tx) * (k / v.k), ty: sy - (sy - v.ty) * (k / v.k) }
    })
  }, [])

  // wheel needs a non-passive listener to stop the page from scrolling
  useEffect(() => {
    const el = wrapRef.current
    if (!el) return
    const onWheel = (event: WheelEvent) => {
      if ((event.target as HTMLElement).closest('[data-no-pan]')) return
      event.preventDefault()
      const rect = el.getBoundingClientRect()
      zoomAt(Math.exp(-event.deltaY * 0.0015), event.clientX - rect.left, event.clientY - rect.top)
    }
    el.addEventListener('wheel', onWheel, { passive: false })
    return () => el.removeEventListener('wheel', onWheel)
  }, [zoomAt])

  function onPointerDown(event: PointerEvent<SVGSVGElement>) {
    if (event.button !== 0 || (event.target as Element).closest('[data-node]')) return
    drag.current = { x: event.clientX, y: event.clientY, tx: view.tx, ty: view.ty, moved: false }
    event.currentTarget.setPointerCapture(event.pointerId)
  }

  function onPointerMove(event: PointerEvent<SVGSVGElement>) {
    const d = drag.current
    if (!d) return
    const dx = event.clientX - d.x
    const dy = event.clientY - d.y
    if (!d.moved && Math.hypot(dx, dy) < 4) return
    if (!d.moved) setPanning(true)
    d.moved = true
    setView((v) => ({ ...v, tx: d.tx + dx, ty: d.ty + dy }))
  }

  function onPointerUp() {
    const d = drag.current
    drag.current = null
    setPanning(false)
    // a click on the empty canvas closes the card
    if (d && !d.moved) onSelect(null)
  }

  useEffect(() => {
    const onKey = (event: globalThis.KeyboardEvent) => {
      if (event.key === 'Escape') onSelect(null)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onSelect])

  function pressKey(id: string) {
    return (event: KeyboardEvent) => {
      if (event.key === 'Enter' || event.key === ' ') {
        event.preventDefault()
        onSelect(id)
      }
    }
  }

  // links touching the hovered (or selected) node light up; the rest dim
  const focus = hovered ?? selectedId
  const hot = useMemo(() => {
    if (!focus) return null
    const set = new Set<string>()
    for (const l of links) if (l.from === focus || l.to === focus) set.add(l.id)
    return set
  }, [focus, links])

  const liveLinks = reducedMotion ? [] : links.filter((l) => l.live).slice(0, MAX_LIVE)


  return (
    <div ref={wrapRef} className={`${styles.wrap} ${panning ? styles.panning : ''}`}>
      <svg
        className={styles.svg}
        role="group"
        aria-label={label}
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={onPointerUp}
        onPointerCancel={onPointerUp}
      >
        <defs>
          <pattern id="lab-grid" width={40} height={40} patternUnits="userSpaceOnUse" patternTransform={`translate(${view.tx} ${view.ty}) scale(${view.k})`}>
            <path d="M40 0H0V40" fill="none" className={styles.grid} />
          </pattern>
        </defs>
        <rect width="100%" height="100%" fill="url(#lab-grid)" />

        <g transform={`translate(${view.tx} ${view.ty}) scale(${view.k})`}>
          {frames.map((f) => {
            const on = f.id === selectedId
            return (
              <g
                key={f.id}
                data-node
                role="button"
                tabIndex={0}
                aria-label={`${f.label}${f.status ? `, ${f.status}` : ''}`}
                aria-pressed={on}
                className={`${styles.frame} ${on ? styles.on : ''} ${f.ghost ? styles.ghost : ''}`}
                onClick={() => onSelect(f.id)}
                onKeyDown={pressKey(f.id)}
                onPointerEnter={() => setHovered(f.id)}
                onPointerLeave={() => setHovered(null)}
              >
                <rect x={f.x} y={f.y} width={f.w} height={f.h} rx={18} className={styles.frameBody} />
                <text x={f.x + 20} y={f.y + 28} className={styles.frameLabel}>{f.label}</text>
                {f.sub && <text x={f.x + f.w - 40} y={f.y + 28} textAnchor="end" className={styles.frameSub}>{f.sub}</text>}
                {f.status && <circle cx={f.x + f.w - 24} cy={f.y + 24} r={5} className={`${styles.ring} ${RING[f.status]}`} />}
              </g>
            )
          })}

          <g className={styles.links}>
            {links.map((l) => (
              <path
                key={l.id}
                d={l.d}
                className={`${styles.link} ${l.live ? styles.linkLive : ''} ${l.dashed ? styles.linkDashed : ''} ${l.radio ? styles.linkRadio : ''} ${hot ? (hot.has(l.id) ? styles.linkHot : styles.linkCold) : ''}`}
              />
            ))}
            {links.filter((l) => l.radio).map((l) => (
              <path
                key={`w-${l.id}`}
                d={radioWaves(l.d)}
                className={`${styles.link} ${styles.radioWaves} ${l.live ? styles.linkLive : ''} ${hot ? (hot.has(l.id) ? styles.linkHot : styles.linkCold) : ''}`}
              />
            ))}
          </g>

          {liveLinks.map((l, i) => (
            <g key={`p-${l.id}`} className={hot && !hot.has(l.id) ? styles.packetCold : undefined}>
              {[0, 1].map((n) => (
                <circle key={n} r={2.5} className={styles.packet}>
                  <animateMotion dur="2.6s" repeatCount="indefinite" path={l.d} begin={`${-((i * 0.37 + n * 1.3) % 2.6)}s`} />
                </circle>
              ))}
            </g>
          ))}

          <g className={styles.notes}>
            {links.filter((l) => l.label).map((l) => (
              <text key={`t-${l.id}`} x={l.label!.x} y={l.label!.y} textAnchor="middle" className={hot && hot.has(l.id) ? styles.noteHot : undefined}>{l.label!.text}</text>
            ))}
            {notes.map((n) => <text key={`${n.text}-${n.x}-${n.y}`} x={n.x} y={n.y} textAnchor={n.anchor}>{n.text}</text>)}
          </g>

          {nodes.map((n) => {
            const on = n.id === selectedId
            const cy = n.y + (n.compact ? n.h / 2 : 22)
            return (
              <g
                key={n.id}
                data-node
                role="button"
                tabIndex={0}
                aria-label={`${n.label}${n.status ? `, ${n.status}` : ''}`}
                aria-pressed={on}
                className={`${styles.node} ${on ? styles.on : ''} ${n.ghost ? styles.ghost : ''} ${n.status ? styles[`st_${n.status}`] : ''}`}
                onClick={() => onSelect(n.id)}
                onDoubleClick={() => onOpen?.(n.id)}
                onKeyDown={pressKey(n.id)}
                onPointerEnter={() => setHovered(n.id)}
                onPointerLeave={() => setHovered(null)}
              >
                <rect x={n.x} y={n.y} width={n.w} height={n.h} rx={6} className={styles.nodeBody} />
                {n.status && <rect x={n.x} y={n.y} width={3} height={n.h} rx={1.5} className={`${styles.stripe} ${RING[n.status]}`} />}
                <g className={styles.nodeIcon}>
                  <NodeIcon name={n.icon} x={n.x + 12} y={cy - (n.compact ? 8 : 9)} size={n.compact ? 16 : 18} />
                </g>
                <text x={n.x + (n.compact ? 34 : 38)} y={cy + 4} className={n.compact ? styles.labelSm : styles.label}>{n.label}</text>
                {n.sub && !n.compact && <text x={n.x + 38} y={n.y + 42} className={styles.sub}>{n.sub}</text>}
                {n.tag && <text x={n.x + (n.compact ? n.w - 10 : 12)} y={n.compact ? cy + 4 : n.y + n.h - 12} textAnchor={n.compact ? 'end' : 'start'} className={styles.tag}>{n.tag}</text>}
                {n.dots && (
                  <g>
                    {n.dots.slice(0, 16).map((d, i) => (
                      <rect key={d.id} x={n.x + 12 + i * 9} y={n.y + n.h - 18} width={6} height={6} rx={1.5} className={`${styles.ueDot} ${RING[d.status]}`} />
                    ))}
                    {n.dots.length > 16 && <text x={n.x + 12 + 16 * 9 + 2} y={n.y + n.h - 12} className={styles.tag}>+{n.dots.length - 16}</text>}
                  </g>
                )}
                {n.status && !n.compact && <circle cx={n.x + n.w - 12} cy={n.y + 12} r={3.5} className={`${styles.ring} ${RING[n.status]}`} />}
              </g>
            )
          })}
        </g>
      </svg>

      {overlay && <div className={styles.overlay} data-no-pan>{overlay}</div>}

      <div className={styles.zoom} data-no-pan>
        <button type="button" onClick={() => zoomAt(1.2, size.w / 2, size.h / 2)} aria-label="Zoom in">+</button>
        <button type="button" onClick={() => zoomAt(1 / 1.2, size.w / 2, size.h / 2)} aria-label="Zoom out">−</button>
        <button type="button" onClick={fit} aria-label="Fit to screen" title="Fit">
          <svg viewBox="0 0 16 16" width="21" height="21" fill="none" stroke="currentColor" strokeWidth="1.6" aria-hidden="true"><path d="M2 6V2h4M10 2h4v4M14 10v4h-4M6 14H2v-4" /></svg>
        </button>
      </div>
      <div className={styles.hint} aria-hidden="true">drag to pan · scroll to zoom · click a node</div>

    </div>
  )
}
