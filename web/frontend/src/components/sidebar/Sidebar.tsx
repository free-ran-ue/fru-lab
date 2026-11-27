import type { ReactNode } from 'react'
import { Link, NavLink } from 'react-router-dom'
import styles from './sidebar.module.css'

function navItemClassName({ isActive }: { isActive: boolean }): string {
  return `${styles.navItem} ${isActive ? styles.navItemActive : ''}`
}

// The cube from the fru-lab logo.
export function LogoMark({ size = 28 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" strokeLinejoin="round" strokeWidth={1.2} aria-hidden="true">
      <path d="M12 2.4L20.6 7 12 11.6 3.4 7z" fill="var(--brand-1)" stroke="var(--brand-1)" />
      <path d="M3 8.6l8.2 4.6V22L3 17.4z" fill="var(--brand-2)" stroke="var(--brand-2)" />
      <path d="M12.8 13.2L21 8.6v8.8L12.8 22z" fill="var(--brand-3)" stroke="var(--brand-3)" />
    </svg>
  )
}

// Icon is a 16px line icon in the nav's text colour.
function Icon({ children }: { children: ReactNode }) {
  return (
    <svg className={styles.icon} width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      {children}
    </svg>
  )
}

const ICONS = {
  dashboard: <Icon><path d="M2.5 2.5h4.5v4.5H2.5zM9 2.5h4.5v4.5H9zM2.5 9h4.5v4.5H2.5zM9 9h4.5v4.5H9z" /></Icon>,
  core: <Icon><rect x="2.5" y="2.5" width="11" height="4" rx="1" /><rect x="2.5" y="9.5" width="11" height="4" rx="1" /><path d="M5 4.5h.01M5 11.5h.01" /></Icon>,
  ran: <Icon><path d="M8 7v7M5.5 14h5M8 7l-2.5 7M8 7l2.5 7" /><path d="M5.2 4.2a4 4 0 0 1 5.6 0M3.4 2.4a6.5 6.5 0 0 1 9.2 0" /></Icon>,
  ue: <Icon><rect x="4.5" y="1.5" width="7" height="13" rx="1.5" /><path d="M7 12h2" /></Icon>,
  logs: <Icon><path d="M3 4h10M3 8h10M3 12h6" /></Icon>,
  images: <Icon><path d="M8 2.5l5.5 3-5.5 3-5.5-3zM2.5 8.5l5.5 3 5.5-3M2.5 11l5.5 3 5.5-3" /></Icon>,
  logout: <Icon><path d="M6 2.5H3v11h3M10 5l3 3-3 3M13 8H6" /></Icon>,
}

interface SidebarProps {
  onLogout?: () => void
}

// Sidebar is the v2 navigation: the brand row, Dashboard, the three lab
// pages (Core Network, free-ran, free-ue), the tools, and Log out at the
// bottom.
export default function Sidebar({ onLogout }: SidebarProps) {
  return (
    <aside className={styles.sidebar}>
      <Link to="/" className={styles.brand} aria-label="fru-lab: go to the dashboard">
        <LogoMark size={38} />
        <span className={styles.wordmark}>fru-<span className={styles.wordmarkLab}>lab</span></span>
      </Link>

      <nav className={`${styles.nav} ${styles.navTop}`} aria-label="Overview">
        <NavLink to="/" end className={navItemClassName}>{ICONS.dashboard}Dashboard</NavLink>
      </nav>

      <p className={styles.navGroup}>Lab</p>
      <nav className={styles.nav} aria-label="Lab">
        <NavLink to="/core-network" className={navItemClassName}>{ICONS.core}Core Network</NavLink>
        <NavLink to="/free-ran" className={navItemClassName}>{ICONS.ran}free-ran</NavLink>
        <NavLink to="/free-ue" className={navItemClassName}>{ICONS.ue}free-ue</NavLink>
      </nav>

      <p className={styles.navGroup}>Tools</p>
      <nav className={styles.nav} aria-label="Tools">
        <NavLink to="/logs" className={navItemClassName}>{ICONS.logs}Logs</NavLink>
        <NavLink to="/images" className={navItemClassName}>{ICONS.images}Images</NavLink>
      </nav>

      {onLogout && (
        <button type="button" className={styles.logout} onClick={onLogout}>{ICONS.logout}Log out</button>
      )}
    </aside>
  )
}
