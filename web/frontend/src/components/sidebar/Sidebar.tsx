import type { ReactNode } from 'react'
import { Link, NavLink } from 'react-router-dom'
import { useTheme } from '../../theme'
import styles from './sidebar.module.css'

// The cube from the fru-lab logo.
export function LogoMark({ size = 28 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" strokeLinejoin="round" strokeWidth={1.2} aria-hidden="true">
      <path d="M12 2.4L20.6 7 12 11.6 3.4 7z" fill="var(--cube-top)" stroke="var(--cube-top)" />
      <path d="M3 8.6l8.2 4.6V22L3 17.4z" fill="var(--cube-left)" stroke="var(--cube-left)" />
      <path d="M12.8 13.2L21 8.6v8.8L12.8 22z" fill="var(--cube-right)" stroke="var(--cube-right)" />
    </svg>
  )
}

function Icon({ children }: { children: ReactNode }) {
  return (
    <svg className={styles.icon} width="20" height="20" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.25" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      {children}
    </svg>
  )
}

const ICONS = {
  overview: <Icon><path d="M2.5 2.5h4.5v4.5H2.5zM9 2.5h4.5v4.5H9zM2.5 9h4.5v4.5H2.5zM9 9h4.5v4.5H9z" /></Icon>,
  core: <Icon><rect x="2.5" y="2.5" width="11" height="4" rx="1" /><rect x="2.5" y="9.5" width="11" height="4" rx="1" /><path d="M5 4.5h.01M5 11.5h.01" /></Icon>,
  ran: <Icon><path d="M8 7v7M5.5 14h5M8 7l-2.5 7M8 7l2.5 7" /><path d="M5.2 4.2a4 4 0 0 1 5.6 0M3.4 2.4a6.5 6.5 0 0 1 9.2 0" /></Icon>,
  ue: <Icon><rect x="4.5" y="1.5" width="7" height="13" rx="1.5" /><path d="M7 12h2" /></Icon>,
  logs: <Icon><path d="M3 4h10M3 8h10M3 12h6" /></Icon>,
  images: <Icon><path d="M8 2.5l5.5 3-5.5 3-5.5-3zM2.5 8.5l5.5 3 5.5-3M2.5 11l5.5 3 5.5-3" /></Icon>,
  sun: <Icon><circle cx="8" cy="8" r="2.75" /><path d="M8 1.5v1.5M8 13v1.5M1.5 8H3M13 8h1.5M3.4 3.4l1.06 1.06M11.54 11.54l1.06 1.06M3.4 12.6l1.06-1.06M11.54 4.46l1.06-1.06" /></Icon>,
  moon: <Icon><path d="M13 9.5A5.5 5.5 0 0 1 6.5 3a5.5 5.5 0 1 0 6.5 6.5z" /></Icon>,
  logout: <Icon><path d="M6 2.5H3v11h3M10 5l3 3-3 3M13 8H6" /></Icon>,
}

const ITEMS: { to: string; label: string; icon: ReactNode; end?: boolean }[] = [
  { to: '/', label: 'Overview', icon: ICONS.overview, end: true },
  { to: '/core-network', label: 'Core Network', icon: ICONS.core },
  { to: '/free-ran', label: 'free-ran (gNB)', icon: ICONS.ran },
  { to: '/free-ue', label: 'free-ue (UE)', icon: ICONS.ue },
]

const TOOLS: typeof ITEMS = [
  { to: '/logs', label: 'Logs', icon: ICONS.logs },
  { to: '/images', label: 'Images', icon: ICONS.images },
]

const cls = ({ isActive }: { isActive: boolean }) => `${styles.item} ${isActive ? styles.active : ''}`

interface SidebarProps {
  onLogout?: () => void
}

// Sidebar is the workbench's activity bar: one icon per view, the tools,
// and the theme and log-out buttons at the bottom. Labels show on hover.
export default function Sidebar({ onLogout }: SidebarProps) {
  const [theme, toggleTheme] = useTheme()
  return (
    <aside className={styles.bar} aria-label="Views">
      <Link to="/" className={styles.brand} aria-label="fru-lab overview"><LogoMark size={24} /></Link>
      <nav className={styles.group}>
        {ITEMS.map((i) => <NavLink key={i.to} to={i.to} end={i.end} className={cls} aria-label={i.label} data-tip={i.label}>{i.icon}</NavLink>)}
      </nav>
      <span className={styles.sep} />
      <nav className={styles.group}>
        {TOOLS.map((i) => <NavLink key={i.to} to={i.to} className={cls} aria-label={i.label} data-tip={i.label}>{i.icon}</NavLink>)}
      </nav>
      <div className={styles.foot}>
        <button type="button" className={styles.item} onClick={toggleTheme} aria-label={theme === 'dark' ? 'Light theme' : 'Dark theme'} data-tip={theme === 'dark' ? 'Light theme' : 'Dark theme'}>
          {theme === 'dark' ? ICONS.sun : ICONS.moon}
        </button>
        {onLogout && <button type="button" className={styles.item} onClick={onLogout} aria-label="Log out" data-tip="Log out">{ICONS.logout}</button>}
      </div>
    </aside>
  )
}
