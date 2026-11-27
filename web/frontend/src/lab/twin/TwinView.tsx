import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState, type PointerEvent } from 'react'
import { ID, overviewCores, ueGnbId } from '../canvas/graph'
import { NodeIcon, type IconName } from '../canvas/icons'
import { useEvents } from '../console/EventsContext'
import { STAGE_LABEL, type UeStage } from '../console/events'
import { useLab } from '../LabContext'
import type { NodeStatus } from '../types'
import { useStoredState } from '../useProfiles'
import styles from './twin.module.css'

interface Props {
  selectedId: string | null
  onSelect: (id: string | null) => void
  // a node double-clicked (the Overview opens its terminal)
  onOpen?: (id: string) => void
}

type Pt = { x: number; y: number }
interface View { k: number; tx: number; ty: number }
export type TwinLayer = 'ue' | 'gnb' | 'core' | 'dn'

// the layers, top to bottom as the panel lists them (drawn bottom-up)
const LAYERS: { id: TwinLayer; label: string; icon: IconName }[] = [
  { id: 'ue', label: 'UE', icon: 'ue' },
  { id: 'gnb', label: 'gNB', icon: 'gnb' },
  { id: 'core', label: 'Core', icon: 'core' },
  { id: 'dn', label: 'DN', icon: 'cloud' },
]

const GRID = 40 // one map cell; dropped points snap to half of it
const SNAP = GRID / 2
const MAX_UES = 300
const PAD = 60
const ROW = { dn: 0, core: 240, gnb: 520 } // the default rows on the map
const UE_RING = 120
const DOUBLE_MS = 350

const snap = (v: number) => Math.round(v / SNAP) * SNAP

function stageClass(stage: UeStage | undefined, status: NodeStatus): string {
  if (stage === 'failed' || status === 'unhealthy') return styles.bad
  if (stage === 'ready' || stage === 'pdu') return styles.ok
  return styles.wait
}

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

interface Point {
  key: string // `${layer}:${id}`, what positions are stored by
  id: string // the node id the inspector knows
  layer: TwinLayer
  icon: IconName
  label: string
  sub?: string
  status?: NodeStatus
  // UEs: coloured by stage
  tone?: string
  dashed?: boolean
}
interface Edge { id: string; from: string; to: string; label: string; live: boolean; dashed?: boolean }

// TwinView is the Overview's 2D map: a grid on which every core, data
// network, gNB and UE is a point with its icon, on four layers (UE, gNB,
// Core, DN) that can be shown, hidden or locked. Any point can be dragged
// (it snaps to the grid; positions are kept per browser). Click selects,
// double-click opens a terminal.
export default function TwinView({ selectedId, onSelect, onOpen }: Props) {
  const lab = useLab()
  const { ueLive } = useEvents()
  const reduced = usePrefersReducedMotion()
  const [placed, setPlaced] = useStoredState<Record<string, Pt>>('frulab-twin-map', {})
  const [hiddenLayers, setHiddenLayers] = useStoredState<TwinLayer[]>('frulab-twin-hidden', [])
  const [lockedLayers, setLockedLayers] = useStoredState<TwinLayer[]>('frulab-twin-locked', [])
  const [drag, setDrag] = useState<{ key: string; p: Pt } | null>(null)
  const [hovered, setHovered] = useState<string | null>(null)

  // ----- the model: points on four layers and the links between them -----
  const { specs, laneOf } = overviewCores(lab)
  const ues = lab.upUes.slice(0, MAX_UES)
  const points: Point[] = []
  const edges: Edge[] = []
  const dnKeys = [...new Set(specs.flatMap((c) => c.dns.map((d) => d.toLowerCase())))]
  for (const key of dnKeys) {
    const on = specs.filter((c) => c.dns.some((d) => d.toLowerCase() === key))
    const id = `${ID.dn}:${key}`
    points.push({ key: `dn:${key}`, id, layer: 'dn', icon: 'cloud', label: on[0].dns.find((d) => d.toLowerCase() === key) ?? key, sub: 'DN' })
    for (const c of on) edges.push({ id: `n6:${c.id}:${key}`, from: `core:${c.id}`, to: `dn:${key}`, label: 'N6', live: c.live })
  }
  for (const c of specs) {
    points.push({ key: `core:${c.id}`, id: c.id, layer: 'core', icon: 'core', label: c.node.label, sub: c.node.sub, status: c.node.status, dashed: c.dashed })
  }
  for (const g of lab.gnbs) {
    const core = specs.find((c) => c.id === laneOf(g))
    points.push({ key: `gnb:${g.id}`, id: g.id, layer: 'gnb', icon: 'gnb', label: g.profileName, sub: g.deployment?.n2Ip || g.deployment?.hostIp || undefined, status: g.status })
    if (core) edges.push({ id: `bh:${g.id}`, from: `gnb:${g.id}`, to: `core:${core.id}`, label: 'N2 · N3', live: core.live && g.status === 'running', dashed: core.dashed })
  }
  for (const r of ues) {
    const live = ueLive[r.ueId]
    const gid = ueGnbId(lab, r)
    points.push({ key: `ue:${r.ueId}`, id: ID.ue(r.ueId), layer: 'ue', icon: 'ue', label: r.ueId.replace(/^imsi-/, ''), sub: live?.ip ?? (live?.stage ? STAGE_LABEL[live.stage] : r.status), status: r.status, tone: stageClass(live?.stage, r.status) })
    if (lab.gnbs.some((g) => g.id === gid)) edges.push({ id: `uu:${r.ueId}`, from: `ue:${r.ueId}`, to: `gnb:${gid}`, label: 'Uu', live: r.status === 'running' && live?.stage !== 'failed' })
  }

  // ----- where a point sits by default: DNs on the top row, cores under
  // them, each core's gNBs under it, each gNB's UEs on a ring around it -----
  const defaults = useMemo(() => {
    const at = new Map<string, Pt>()
    let x = 0
    for (const c of specs) {
      const gnbs = lab.gnbs.filter((g) => laneOf(g) === c.id)
      const width = Math.max(1, gnbs.length) * 320
      at.set(`core:${c.id}`, { x: snap(x + width / 2), y: ROW.core })
      gnbs.forEach((g, i) => {
        const gp = { x: snap(x + 160 + i * 320), y: ROW.gnb }
        at.set(`gnb:${g.id}`, gp)
        const mine = ues.filter((r) => ueGnbId(lab, r) === g.id)
        mine.forEach((r, j) => {
          // a fan under the gNB, widening with the count
          const span = Math.min(Math.PI * 1.6, 0.5 + mine.length * 0.35)
          const a = Math.PI / 2 - span / 2 + (mine.length === 1 ? span / 2 : (j * span) / (mine.length - 1))
          const ring = UE_RING + Math.floor(j / 12) * 70
          at.set(`ue:${r.ueId}`, { x: snap(gp.x + Math.cos(a) * ring), y: snap(gp.y + Math.sin(a) * ring) })
        })
      })
      x += width + 80
    }
    // UEs whose gNB is not drawn: a row under everything
    ues.filter((r) => !at.has(`ue:${r.ueId}`)).forEach((r, i) => at.set(`ue:${r.ueId}`, { x: snap(i * 80), y: ROW.gnb + 320 }))
    // DNs above the cores that have them
    // (pushed apart left to right so none overlap)
    const wants = dnKeys.map((key) => {
      const on = specs.filter((c) => c.dns.some((d) => d.toLowerCase() === key))
      return { key, x: on.reduce((t, c) => t + (at.get(`core:${c.id}`)?.x ?? 0), 0) / Math.max(1, on.length) }
    }).sort((a, b) => a.x - b.x)
    let floor = -Infinity
    for (const w of wants) {
      const x = Math.max(w.x, floor)
      floor = x + 200
      at.set(`dn:${w.key}`, { x: snap(x), y: ROW.dn })
    }
    return at
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [points.map((p) => p.key).join('|')])

  const posOf = (key: string): Pt => (drag?.key === key ? drag.p : placed[key] ?? defaults.get(key) ?? { x: 0, y: 0 })
  const shown = (layer: TwinLayer) => !hiddenLayers.includes(layer)
  const layerOf = (key: string) => key.slice(0, key.indexOf(':')) as TwinLayer

  // ----- pan / zoom -----
  const wrap = useRef<HTMLDivElement>(null)
  const [size, setSize] = useState({ w: 0, h: 0 })
  const [view, setView] = useState<View>({ k: 1, tx: 0, ty: 0 })
  const panRef = useRef<{ x: number; y: number; tx: number; ty: number; moved: boolean } | null>(null)
  const dragRef = useRef<{ key: string; sx: number; sy: number; start: Pt; moved: boolean } | null>(null)

  const bounds = useMemo(() => {
    const ps = points.map((p) => posOf(p.key))
    if (ps.length === 0) return { x: -200, y: -100, w: 400, h: 300 }
    const xs = ps.map((p) => p.x)
    const ys = ps.map((p) => p.y)
    return { x: Math.min(...xs) - 80, y: Math.min(...ys) - 80, w: Math.max(...xs) - Math.min(...xs) + 160, h: Math.max(...ys) - Math.min(...ys) + 200 }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [defaults, placed])

  const fit = useCallback(() => {
    if (!size.w || !size.h) return
    const k = Math.min(2, Math.max(0.15, Math.min((size.w - PAD * 2) / bounds.w, (size.h - PAD * 2) / bounds.h, 1.4)))
    setView({ k, tx: (size.w - bounds.w * k) / 2 - bounds.x * k, ty: (size.h - bounds.h * k) / 2 - bounds.y * k })
  }, [size.w, size.h, bounds])

  useLayoutEffect(() => {
    const el = wrap.current
    if (!el) return
    const ro = new ResizeObserver(([e]) => setSize({ w: e.contentRect.width, h: e.contentRect.height }))
    ro.observe(el)
    return () => ro.disconnect()
  }, [])
  // refit when the size or the set of points changes (not while dragging)
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => { if (!dragRef.current) fit() }, [size.w, size.h, defaults])

  const zoomAt = useCallback((f: number, sx: number, sy: number) => {
    setView((v) => {
      const k = Math.min(4, Math.max(0.15, v.k * f))
      return { k, tx: sx - (sx - v.tx) * (k / v.k), ty: sy - (sy - v.ty) * (k / v.k) }
    })
  }, [])
  useEffect(() => {
    const el = wrap.current
    if (!el) return
    const onWheel = (e: WheelEvent) => {
      if ((e.target as HTMLElement).closest('[data-no-pan]')) return
      e.preventDefault()
      const r = el.getBoundingClientRect()
      zoomAt(Math.exp(-e.deltaY * 0.0015), e.clientX - r.left, e.clientY - r.top)
    }
    el.addEventListener('wheel', onWheel, { passive: false })
    return () => el.removeEventListener('wheel', onWheel)
  }, [zoomAt])

  function down(e: PointerEvent<SVGSVGElement>) {
    if (e.button !== 0) return
    const key = (e.target as Element).closest('[data-point]')?.getAttribute('data-point')
    if (key) {
      // a locked point clicks but never moves (see move)
      dragRef.current = { key, sx: e.clientX, sy: e.clientY, start: posOf(key), moved: false }
    } else {
      panRef.current = { x: e.clientX, y: e.clientY, tx: view.tx, ty: view.ty, moved: false }
    }
    e.currentTarget.setPointerCapture(e.pointerId)
  }
  function move(e: PointerEvent<SVGSVGElement>) {
    const d = dragRef.current
    if (d) {
      if (lockedLayers.includes(layerOf(d.key))) return
      const dx = (e.clientX - d.sx) / view.k
      const dy = (e.clientY - d.sy) / view.k
      if (!d.moved && Math.hypot(dx, dy) * view.k < 4) return
      d.moved = true
      setDrag({ key: d.key, p: { x: d.start.x + dx, y: d.start.y + dy } })
      return
    }
    const p = panRef.current
    if (!p) return
    const dx = e.clientX - p.x
    const dy = e.clientY - p.y
    if (!p.moved && Math.hypot(dx, dy) < 4) return
    p.moved = true
    setView((v) => ({ ...v, tx: p.tx + dx, ty: p.ty + dy }))
  }
  // the pointer is captured by the map while a point is pressed (so a drag
  // can't lose it), which also sends the click and dblclick events to the
  // map instead of the point - so a press that didn't move is the click,
  // and two of them on one point within DOUBLE_MS are the double-click
  const lastClick = useRef<{ key: string; t: number } | null>(null)
  function clickPoint(key: string) {
    const id = points.find((p) => p.key === key)?.id
    if (!id) return
    const now = performance.now()
    const prev = lastClick.current
    onSelect(id)
    if (prev && prev.key === key && now - prev.t < DOUBLE_MS) {
      lastClick.current = null
      onOpen?.(id)
    } else lastClick.current = { key, t: now }
  }
  function up() {
    const d = dragRef.current
    dragRef.current = null
    if (d) {
      if (d.moved && drag) setPlaced({ ...placed, [d.key]: { x: snap(drag.p.x), y: snap(drag.p.y) } })
      else if (!d.moved) clickPoint(d.key)
      setDrag(null)
      return
    }
    const p = panRef.current
    panRef.current = null
    if (p && !p.moved) onSelect(null)
  }

  const toggle = (list: TwinLayer[], set: (v: TwinLayer[]) => void, id: TwinLayer) => set(list.includes(id) ? list.filter((x) => x !== id) : [...list, id])
  const focusKey = hovered ?? points.find((p) => p.id === selectedId)?.key ?? null
  const count = (layer: TwinLayer) => points.filter((p) => p.layer === layer).length

  // the grid, drawn as two patterns that follow the pan / zoom
  const gridT = `translate(${view.tx} ${view.ty}) scale(${view.k})`

  return (
    <div ref={wrap} className={styles.wrap}>
      <svg className={styles.svg} role="group" aria-label="Lab map: cores, data networks, gNBs and UEs as points on a grid, on four layers" onPointerDown={down} onPointerMove={move} onPointerUp={up} onPointerCancel={up}>
        <defs>
          <pattern id="twin-minor" width={GRID} height={GRID} patternUnits="userSpaceOnUse" patternTransform={gridT}>
            <path d={`M${GRID} 0H0V${GRID}`} fill="none" className={styles.gridMinor} />
          </pattern>
          <pattern id="twin-major" width={GRID * 5} height={GRID * 5} patternUnits="userSpaceOnUse" patternTransform={gridT}>
            <path d={`M${GRID * 5} 0H0V${GRID * 5}`} fill="none" className={styles.gridMajor} />
          </pattern>
        </defs>
        <rect width="100%" height="100%" fill="url(#twin-minor)" />
        <rect width="100%" height="100%" fill="url(#twin-major)" />

        <g transform={gridT}>
          {/* links: under every layer, shown when both ends are */}
          {edges.filter((e) => shown(layerOf(e.from)) && shown(layerOf(e.to))).map((e) => {
            const a = posOf(e.from)
            const b = posOf(e.to)
            const hot = focusKey === e.from || focusKey === e.to
            const isUu = e.label === 'Uu'
            return (
              <g key={e.id}>
                <line x1={a.x} y1={a.y} x2={b.x} y2={b.y} className={`${styles.link} ${e.live ? styles.live : ''} ${e.dashed ? styles.dashed : ''} ${isUu ? styles.radio : ''} ${hot ? styles.hot : ''}`} />
                {e.live && !isUu && !reduced && (
                  <circle r={3} className={styles.packet}>
                    <animateMotion dur="2.4s" repeatCount="indefinite" path={`M${a.x} ${a.y}L${b.x} ${b.y}`} />
                  </circle>
                )}
                <text x={(a.x + b.x) / 2} y={(a.y + b.y) / 2 - 6} textAnchor="middle" className={`${styles.linkLabel} ${hot ? styles.linkLabelHot : ''}`}>{e.label}</text>
              </g>
            )
          })}

          {/* the layers, bottom up: DN, Core, gNB, UE */}
          {(['dn', 'core', 'gnb', 'ue'] as TwinLayer[]).filter(shown).map((layer) => (
            <g key={layer} aria-label={`${layer} layer`}>
              {points.filter((p) => p.layer === layer).map((p) => {
                const at = posOf(p.key)
                const small = layer === 'ue'
                const r = small ? 15 : 24
                const on = p.id === selectedId
                const locked = lockedLayers.includes(layer)
                return (
                  <g
                    key={p.key}
                    data-point={p.key}
                    role="button"
                    tabIndex={0}
                    aria-label={`${p.label}${p.status ? `, ${p.status}` : ''}`}
                    aria-pressed={on}
                    className={`${styles.point} ${on ? styles.on : ''} ${locked ? styles.locked : ''} ${drag?.key === p.key ? styles.dragging : ''}`}
                    onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onSelect(p.id) } }}
                    onPointerEnter={() => setHovered(p.key)}
                    onPointerLeave={() => setHovered(null)}
                  >
                    <circle cx={at.x} cy={at.y} r={r} className={`${styles.marker} ${p.tone ?? ''} ${p.dashed ? styles.dashedMarker : ''}`} />
                    {p.status && !p.tone && <circle cx={at.x} cy={at.y} r={r} className={`${styles.ring} ${styles[`st_${p.status}`]}`} />}
                    <g className={p.tone ? styles.iconOnTone : styles.icon}>
                      <NodeIcon name={p.icon} x={at.x - (small ? 9 : 13)} y={at.y - (small ? 9 : 13)} size={small ? 18 : 26} />
                    </g>
                    <text x={at.x} y={at.y + r + 15} textAnchor="middle" className={small ? styles.labelSm : styles.label}>{p.label}</text>
                    {p.sub && (!small || focusKey === p.key || on) && <text x={at.x} y={at.y + r + 29} textAnchor="middle" className={styles.sub}>{p.sub}</text>}
                  </g>
                )
              })}
            </g>
          ))}
        </g>
      </svg>

      <div className={styles.layers} data-no-pan>
        <p className={styles.layersTitle}>Layers</p>
        {LAYERS.map((l) => (
          <div key={l.id} className={styles.layerRow}>
            <label className={styles.layerShow}>
              <input type="checkbox" checked={shown(l.id)} onChange={() => toggle(hiddenLayers, setHiddenLayers, l.id)} />
              <svg width="16" height="16" viewBox="0 0 24 24" aria-hidden="true"><NodeIcon name={l.icon} x={0} y={0} size={24} /></svg>
              <span>{l.label}</span>
              <b>{count(l.id)}</b>
            </label>
            <button type="button" className={`${styles.lock} ${lockedLayers.includes(l.id) ? styles.lockOn : ''}`} aria-pressed={lockedLayers.includes(l.id)} title={lockedLayers.includes(l.id) ? `Unlock: ${l.label} points can be dragged` : `Lock: ${l.label} points stay put`} onClick={() => toggle(lockedLayers, setLockedLayers, l.id)}>
              {lockedLayers.includes(l.id) ? 'locked' : 'lock'}
            </button>
          </div>
        ))}
      </div>

      <div className={styles.legend} data-no-pan>
        <span><i className={styles.ok} /> UE ready</span>
        <span><i className={styles.wait} /> connecting</span>
        <span><i className={styles.bad} /> failed</span>
        <span className={styles.hint}>drag any point · it snaps to the grid</span>
      </div>
      <div className={styles.tools} data-no-pan>
        <button type="button" onClick={() => zoomAt(1.2, size.w / 2, size.h / 2)} aria-label="Zoom in">+</button>
        <button type="button" onClick={() => zoomAt(1 / 1.2, size.w / 2, size.h / 2)} aria-label="Zoom out">−</button>
        <button type="button" onClick={fit} aria-label="Fit to screen" title="Fit">Fit</button>
        <button type="button" onClick={() => setPlaced({})} disabled={Object.keys(placed).length === 0} title="Put every point back where fru-lab places it">Reset layout</button>
      </div>
    </div>
  )
}
