import { useCallback, useEffect, useRef, useState } from 'react'
import { useEvents } from '../console/EventsContext'
import type { LogSource } from '../console/stream'
import { useStoredState } from '../useProfiles'
import EventsTab from './EventsTab'
import LogsTab, { type SourceOption } from './LogsTab'
import TerminalTab from './TerminalTab'
import TestsTab from './TestsTab'
import styles from './console.module.css'

export type ConsoleTab = 'logs' | 'events' | 'terminal' | 'tests'

// useConsole is the console's state, owned by the view so selections and
// inspector buttons can point it somewhere.
export function useConsole() {
  const [tab, setTabState] = useStoredState<ConsoleTab>('frulab-console-tab', 'events')
  const [open, setOpen] = useStoredState('frulab-console-open', true)
  const [height, setHeight] = useStoredState('frulab-console-height', 260)
  const [source, setSource] = useState<LogSource | null>(null)
  const show = useCallback((t: ConsoleTab) => { setTabState(t); setOpen(true) }, [setTabState, setOpen])
  return { tab, setTab: setTabState, show, open, setOpen, height, setHeight, source, setSource }
}

export type ConsoleState = ReturnType<typeof useConsole>

interface Props {
  state: ConsoleState
  options: SourceOption[]
  selectedId: string | null
  onSelectNode: (id: string) => void
  // the selected UE when it runs: the terminal and tests act on it
  ue: { id: string; tun?: string; ready: boolean } | null
}

export default function Console({ state, options, selectedId, onSelectNode, ue }: Props) {
  const { events } = useEvents()
  const errors = events.filter((e) => e.level === 'error').length
  const [resizing, setResizing] = useState(false)
  const start = useRef<{ y: number; h: number } | null>(null)

  useEffect(() => {
    if (!resizing) return
    const move = (e: PointerEvent) => {
      if (!start.current) return
      state.setHeight(Math.min(window.innerHeight - 200, Math.max(120, start.current.h + start.current.y - e.clientY)))
    }
    const up = () => setResizing(false)
    window.addEventListener('pointermove', move)
    window.addEventListener('pointerup', up)
    return () => { window.removeEventListener('pointermove', move); window.removeEventListener('pointerup', up) }
  }, [resizing, state])

  const tabs: { id: ConsoleTab; label: string; badge?: number; bad?: boolean }[] = [
    { id: 'events', label: 'Events', badge: errors || events.length || undefined, bad: errors > 0 },
    { id: 'logs', label: 'Logs' },
    { id: 'terminal', label: 'Terminal' },
    { id: 'tests', label: 'Tests' },
  ]

  return (
    <section className={styles.console} style={{ height: state.open ? state.height : undefined }} aria-label="Console">
      {state.open && (
        <div
          className={`${styles.resize} ${resizing ? styles.resizing : ''}`}
          onPointerDown={(e) => { start.current = { y: e.clientY, h: state.height }; setResizing(true) }}
          role="separator"
          aria-orientation="horizontal"
          aria-label="Resize console"
        />
      )}
      <div className={styles.bar} role="tablist">
        {tabs.map((t) => (
          <button key={t.id} type="button" role="tab" className={styles.tab} aria-selected={state.open && state.tab === t.id} onClick={() => (state.open && state.tab === t.id ? state.setOpen(false) : state.show(t.id))}>
            {t.label}
            {t.badge !== undefined && <span className={`${styles.count} ${t.bad ? styles.countBad : ''}`}>{t.bad ? `${t.badge} err` : t.badge}</span>}
          </button>
        ))}
        <div className={styles.barTools}>
          <button type="button" className={styles.iconBtn} onClick={() => state.setOpen(!state.open)} aria-label={state.open ? 'Hide console' : 'Show console'} title={state.open ? 'Hide console' : 'Show console'}>
            <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true"><path d={state.open ? 'M4 6l4 4 4-4' : 'M4 10l4-4 4 4'} /></svg>
          </button>
        </div>
      </div>
      {state.open && (
        <div className={styles.body}>
          {state.tab === 'logs' && <LogsTab options={options} source={state.source} onSource={state.setSource} />}
          {state.tab === 'events' && <EventsTab selectedId={selectedId} onSelectNode={onSelectNode} />}
          {state.tab === 'terminal' && <TerminalTab instance={ue?.id ?? null} />}
          {state.tab === 'tests' && <TestsTab instance={ue?.id ?? null} tun={ue?.tun} ready={ue?.ready ?? false} />}
        </div>
      )}
    </section>
  )
}
