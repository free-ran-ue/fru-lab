import type { ReactNode } from 'react'
import { Link, NavLink, useLocation } from 'react-router-dom'
import { useTheme } from '../../theme'
import { useLab } from '../../lab/LabContext'
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
    <svg className={styles.icon} width="18" height="18" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.25" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      {children}
    </svg>
  )
}

const ICONS = {
  overview: <Icon><path d="M2.5 2.5h4.5v4.5H2.5zM9 2.5h4.5v4.5H9zM2.5 9h4.5v4.5H2.5zM9 9h4.5v4.5H9z" /></Icon>,
  core: <Icon><rect x="2.5" y="2.5" width="11" height="4" rx="1" /><rect x="2.5" y="9.5" width="11" height="4" rx="1" /><path d="M5 4.5h.01M5 11.5h.01" /></Icon>,
  ran: <Icon><path d="M8 7v7M5.5 14h5M8 7l-2.5 7M8 7l2.5 7" /><path d="M5.2 4.2a4 4 0 0 1 5.6 0M3.4 2.4a6.5 6.5 0 0 1 9.2 0" /></Icon>,
  ue: <Icon><rect x="4.5" y="1.5" width="7" height="13" rx="1.5" /><path d="M7 12h2" /></Icon>,
  images: <Icon><path d="M8 2.5l5.5 3-5.5 3-5.5-3zM2.5 8.5l5.5 3 5.5-3M2.5 11l5.5 3 5.5-3" /></Icon>,
  sun: <Icon><circle cx="8" cy="8" r="2.75" /><path d="M8 1.5v1.5M8 13v1.5M1.5 8H3M13 8h1.5M3.4 3.4l1.06 1.06M11.54 11.54l1.06 1.06M3.4 12.6l1.06-1.06M11.54 4.46l1.06-1.06" /></Icon>,
  moon: <Icon><path d="M13 9.5A5.5 5.5 0 0 1 6.5 3a5.5 5.5 0 1 0 6.5 6.5z" /></Icon>,
  logout: <Icon><path d="M6 2.5H3v11h3M10 5l3 3-3 3M13 8H6" /></Icon>,
}

interface NavItem { to: string; label: string; icon: ReactNode; end?: boolean; children?: { to: string; label: string; end?: boolean }[] }

const ITEMS: NavItem[] = [
  {
    to: '/', label: 'Overview', icon: ICONS.overview, end: true,
    children: [{ to: '/events', label: 'Events' }, { to: '/logs', label: 'Logs' }],
  },
  // its sub-pages are filled in with the saved core networks (coreItem)
  { to: '/core-network', label: 'Core Network', icon: ICONS.core },
  { to: '/free-ran', label: 'free-ran (gNB)', icon: ICONS.ran },
  { to: '/free-ue', label: 'free-ue (UE)', icon: ICONS.ue },
]

// coreItem is Core Network (itself the list of cores) with a sub-page per
// core: the lab free5GC (its Subscribers are a tab of its page), then
// every saved core outside the lab.
function coreItem(base: NavItem, cores: { id?: string; name: string }[]): NavItem {
  return {
    ...base,
    children: [
      { to: '/core-network/lab', label: 'free5GC' },
      ...cores.filter((c) => c.id).map((c) => ({ to: `/core-network/n/${c.id}`, label: c.name })),
    ],
  }
}

const TOOLS: NavItem[] = [
  { to: '/images', label: 'Images', icon: ICONS.images },
]

const cls = ({ isActive }: { isActive: boolean }) => `${styles.item} ${isActive ? styles.active : ''}`
const subCls = ({ isActive }: { isActive: boolean }) => `${styles.item} ${styles.sub} ${isActive ? styles.active : ''}`

interface SidebarProps {
  onLogout?: () => void
}

// Item is one view; a view with sub-pages lists them under it. The view's
// own row opens its own page (lit while it is open) and stays half-lit
// while one of its sub-pages is.
function Item({ to, label, icon, end, children }: NavItem) {
  const { pathname } = useLocation()
  const here = end ? pathname === to : pathname === to || pathname === `${to}/`
  const inGroup = !here && children?.some((c) => (c.end ? pathname === c.to : pathname.startsWith(c.to)))
  return (
    <>
      <NavLink to={to} end className={() => (children ? `${styles.item} ${here ? styles.active : inGroup ? styles.parentActive : ''}` : cls({ isActive: here || (!end && pathname.startsWith(`${to}/`)) }))} title={label}>
        {icon}
        <span className={styles.label}>{label}</span>
      </NavLink>
      {children?.map((c) => (
        <NavLink key={c.to} to={c.to} end={c.end} className={subCls} title={`${label} · ${c.label}`}>
          <span className={styles.label}>{c.label}</span>
        </NavLink>
      ))}
    </>
  )
}

// Sidebar is the workbench's navigation: the views, the tools, and the
// theme and log-out buttons at the bottom, each an icon with its label.
// Below 700px it collapses to icons only.
export default function Sidebar({ onLogout }: SidebarProps) {
  const lab = useLab()
  const [theme, toggleTheme] = useTheme()
  const themeLabel = theme === 'dark' ? 'Light theme' : 'Dark theme'
  return (
    <aside className={styles.bar} aria-label="Views">
      <Link to="/" className={styles.brand} aria-label="fru-lab overview">
        <LogoMark size={27} />
        <span className={styles.brandName}>fru-lab</span>
      </Link>
      <nav className={styles.group} aria-label="Lab">
        <span className={styles.heading}>Lab</span>
        {ITEMS.map((i) => <Item key={i.to} {...(i.to === '/core-network' ? coreItem(i, lab.coreNetworks.networks) : i)} />)}
      </nav>
      <nav className={styles.group} aria-label="Tools">
        <span className={styles.heading}>Tools</span>
        {TOOLS.map((i) => <Item key={i.to} {...i} />)}
      </nav>
      <div className={styles.foot}>
        <button type="button" className={styles.item} onClick={toggleTheme} title={themeLabel}>
          {theme === 'dark' ? ICONS.sun : ICONS.moon}
          <span className={styles.label}>{themeLabel}</span>
        </button>
        {onLogout && (
          <button type="button" className={styles.item} onClick={onLogout} title="Log out">
            {ICONS.logout}
            <span className={styles.label}>Log out</span>
          </button>
        )}
      </div>
    </aside>
  )
}
