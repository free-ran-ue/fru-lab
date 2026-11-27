import { Fragment, useEffect, useState, type ReactNode } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import Sidebar from '../sidebar/Sidebar'
import { useApiLive } from './useApiLive'
import styles from './layout.module.css'

export interface Crumb {
  label: ReactNode
  to?: string
}

interface AppLayoutProps {
  // the page's breadcrumb, outermost first; the section (from the URL) is
  // added in front
  crumbs: Crumb[]
  // tags after the breadcrumb, e.g. "UNSAVED"
  crumbExtra?: ReactNode
  // the header's right side before the clock, e.g. Setup's actions
  headerActions?: ReactNode
  // above everything (notifications)
  overlay?: ReactNode
  children: ReactNode
}

// sectionOf is the breadcrumb's first entry for a URL: the sidebar item the
// page belongs to.
function sectionOf(pathname: string): Crumb | null {
  if (pathname.startsWith('/core-network')) return { label: 'Core Network', to: '/core-network' }
  if (pathname.startsWith('/free-ran')) return { label: 'free-ran', to: '/free-ran' }
  if (pathname.startsWith('/free-ue')) return { label: 'free-ue', to: '/free-ue' }
  if (pathname.startsWith('/logs') || pathname.startsWith('/images')) return { label: 'Tools' }
  return null
}

// utcClock is hh:mm:ss UTC, ticking every second.
function useUtcClock(): string {
  const [now, setNow] = useState(() => new Date())
  useEffect(() => {
    const timer = window.setInterval(() => setNow(new Date()), 1000)
    return () => window.clearInterval(timer)
  }, [])
  return now.toISOString().slice(11, 19)
}

// AppLayout is every signed-in page in the v2 frame: the sidebar, a 48px
// header bar (breadcrumb; UTC clock, API state and user on the right), the
// content column and a status bar at the bottom.
export default function AppLayout({ crumbs, crumbExtra, headerActions, overlay, children }: AppLayoutProps) {
  const navigate = useNavigate()
  const live = useApiLive()
  const clock = useUtcClock()
  const user = localStorage.getItem('username') || 'admin'
  const section = sectionOf(useLocation().pathname)
  const trail: Crumb[] = section ? [section, ...crumbs] : crumbs

  function handleLogout() {
    localStorage.removeItem('token')
    localStorage.removeItem('username')
    navigate('/login', { replace: true })
  }

  return (
    <div className={styles.shell}>
      {overlay}
      <Sidebar onLogout={handleLogout} />
      <main className={styles.main}>
        <header className={styles.bar}>
          <nav aria-label="Breadcrumb" className={styles.crumbs}>
            {trail.map((c, i) => {
              const last = i === trail.length - 1
              return (
                <Fragment key={i}>
                  {i > 0 && <span aria-hidden="true">/</span>}
                  {c.to && !last
                    ? <Link to={c.to} className={styles.crumbLink}>{c.label}</Link>
                    : <span className={last ? styles.crumbCurrent : undefined} aria-current={last ? 'page' : undefined}>{c.label}</span>}
                </Fragment>
              )
            })}
            {crumbExtra}
          </nav>
          <div className={styles.barRight}>
            {headerActions}
            <span className="num">{clock} UTC</span>
            <span className={styles.live}>
              <span className={live ? styles.liveDot : styles.offDot} aria-hidden="true" />
              {live ? 'API connected' : 'API not answering'}
            </span>
            <span>{user}</span>
          </div>
        </header>
        <div className={styles.content}>{children}</div>
        <footer className={styles.status}>
          <span>{live ? 'api: connected' : 'api: not answering'} · polled every 5 s</span>
          <span className={styles.statusRight}>fru-lab</span>
        </footer>
      </main>
    </div>
  )
}
