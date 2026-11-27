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
  subscribers: <Icon><circle cx="8" cy="5.5" r="2.5" /><path d="M3 13.5c.8-2.4 2.7-3.5 5-3.5s4.2 1.1 5 3.5" /></Icon>,
  logs: <Icon><path d="M3 4h10M3 8h10M3 12h6" /></Icon>,
  images: <Icon><path d="M8 2.5l5.5 3-5.5 3-5.5-3zM2.5 8.5l5.5 3 5.5-3M2.5 11l5.5 3 5.5-3" /></Icon>,
  logout: <Icon><path d="M6 2.5H3v11h3M10 5l3 3-3 3M13 8H6" /></Icon>,
}

interface SidebarProps {
  onLogout?: () => void
}

// Sidebar is the v2 navigation: the brand row, the icon links, and Log out
// at the bottom.
export default function Sidebar({ onLogout }: SidebarProps) {
  return (
    <aside className={styles.sidebar}>
      <Link to="/" className={styles.brand} aria-label="fru-lab: go to the dashboard">
        <LogoMark size={38} />
        <span className={styles.wordmark}>fru-<span className={styles.wordmarkLab}>lab</span></span>
      </Link>

      <p className={styles.navGroup}>Core network</p>
      <nav className={styles.nav} aria-label="Core network">
        <NavLink to="/" end className={navItemClassName}>{ICONS.dashboard}Dashboard</NavLink>
        <NavLink to="/subscribers" className={navItemClassName}>{ICONS.subscribers}Subscribers</NavLink>
        <NavLink to="/logs" className={navItemClassName}>{ICONS.logs}Logs</NavLink>
        <NavLink to="/images" className={navItemClassName}>{ICONS.images}Images</NavLink>
      </nav>

      {onLogout && (
        <button type="button" className={styles.logout} onClick={onLogout}>{ICONS.logout}Log out</button>
      )}
    </aside>
  )
}
