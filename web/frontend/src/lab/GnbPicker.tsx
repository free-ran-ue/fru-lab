import { useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { useLab } from './LabContext'
import { Button } from './LabUi'
import styles from './deploy-menu.module.css'

export interface PickItem {
  value: string
  label: string
  sub?: string
}

interface PickProps {
  label: string
  // the menu's heading, e.g. "Deploy onto"
  title: string
  items: PickItem[]
  small?: boolean
  busy?: boolean
  // why it cannot act now, if it cannot
  blocked?: string
  onPick: (value: string) => void
}

// PickMenu is a button that opens a short list to pick from; picking acts.
// It draws in a portal, so a scrolling list never clips it.
export function PickMenu({ label, title, items, small = true, busy, blocked, onPick }: PickProps) {
  const [at, setAt] = useState<{ x: number; y: number } | null>(null)
  const anchor = useRef<HTMLSpanElement>(null)
  const menu = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!at) return
    const away = (e: MouseEvent) => {
      if (!menu.current?.contains(e.target as Node) && !anchor.current?.contains(e.target as Node)) setAt(null)
    }
    const key = (e: KeyboardEvent) => { if (e.key === 'Escape') setAt(null) }
    const gone = () => setAt(null)
    document.addEventListener('mousedown', away)
    document.addEventListener('keydown', key)
    window.addEventListener('scroll', gone, true)
    window.addEventListener('resize', gone)
    return () => {
      document.removeEventListener('mousedown', away)
      document.removeEventListener('keydown', key)
      window.removeEventListener('scroll', gone, true)
      window.removeEventListener('resize', gone)
    }
  }, [at])

  function open() {
    if (at) { setAt(null); return }
    const r = anchor.current?.getBoundingClientRect()
    if (r) setAt({ x: r.right, y: r.bottom + 4 })
  }

  return (
    <span ref={anchor} className={styles.anchor}>
      <Button kind="primary" small={small} busy={busy} disabled={Boolean(blocked)} title={blocked ?? title} onClick={open}>{label} ▾</Button>
      {at && createPortal(
        <div ref={menu} className={styles.menu} role="menu" style={{ top: at.y, left: at.x }}>
          <p className={styles.title}>{title}</p>
          {items.map((it) => (
            <button key={it.value} type="button" role="menuitem" className={styles.item} onClick={() => { setAt(null); onPick(it.value) }}>
              <b>{it.label}</b>{it.sub && <span>{it.sub}</span>}
            </button>
          ))}
        </div>,
        document.body,
      )}
    </span>
  )
}

interface Props {
  label?: string
  small?: boolean
  busy?: boolean
  // why it cannot deploy now, if it cannot
  blocked?: string
  // '' is Auto: the backend picks a running gNB by the UE's SD
  onPick: (gnb: string) => void
}

// DeployMenu is a UE's Deploy button: pressing it opens the list of gNBs
// to deploy onto ("Auto" first, then every running gNB), and picking one
// deploys.
export function DeployMenu({ label = 'Deploy', small = true, busy, blocked, onPick }: Props) {
  const lab = useLab()
  const items: PickItem[] = [
    { value: '', label: 'Auto', sub: "a running gNB with the UE's SD" },
    ...lab.gnbs.filter((g) => g.status === 'running').map((g) => ({ value: g.id, label: g.profileName, sub: [g.sd && `SD ${g.sd}`, g.deployment?.n2Ip || g.deployment?.hostIp].filter(Boolean).join(' · ') })),
  ]
  return <PickMenu label={label} title="Deploy onto" items={items} small={small} busy={busy} blocked={blocked} onPick={onPick} />
}
