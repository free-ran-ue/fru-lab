import { Fragment, useEffect, useState, type ReactNode } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import Sidebar from '../sidebar/Sidebar'
import { useApiLive } from './useApiLive'
import { useLab } from '../../lab/LabContext'
import type { NodeStatus } from '../../lab/types'
import styles from './layout.module.css'

export interface Crumb {
  label: ReactNode
  to?: string
}

interface AppLayoutProps {
  // the page's breadcrumb, outermost first; the view (from the URL) is
  // added in front
  crumbs: Crumb[]
  // tags after the breadcrumb, e.g. "UNSAVED"
  crumbExtra?: ReactNode
  // the header's right side before the lab status
  headerActions?: ReactNode
  // above everything (notifications)
  overlay?: ReactNode
  // the workbench views fill the window instead of scrolling a column
  fill?: boolean
  children: ReactNode
}

function sectionOf(pathname: string): Crumb | null {
  if (pathname.startsWith('/core-network')) return { label: 'Core Network', to: '/core-network' }
  if (pathname.startsWith('/free-ran')) return { label: 'free-ran', to: '/free-ran' }
  if (pathname.startsWith('/free-ue')) return { label: 'free-ue', to: '/free-ue' }
  if (pathname.startsWith('/logs') || pathname.startsWith('/images')) return { label: 'Tools' }
  return null
}

function useUtcClock(): string {
  const [now, setNow] = useState(() => new Date())
  useEffect(() => {
    const timer = window.setInterval(() => setNow(new Date()), 1000)
    return () => window.clearInterval(timer)
  }, [])
  return now.toISOString().slice(11, 19)
}

function Pill({ to, status, label, value }: { to: string; status: NodeStatus; label: string; value: string }) {
  return (
    <Link to={to} className={styles.pill} title={`${label}: ${status}`}>
      <i className={styles[`st_${status}`]} aria-hidden="true" />
      <span>{label}</span>
      <b>{value}</b>
    </Link>
  )
}

// AppLayout is the workbench frame: the activity bar, a top bar with the
// breadcrumb and the lab's live status, the view, and a status bar.
export default function AppLayout({ crumbs, crumbExtra, headerActions, overlay, fill, children }: AppLayoutProps) {
  const navigate = useNavigate()
  const live = useApiLive()
  const lab = useLab()
  const clock = useUtcClock()
  const user = localStorage.getItem('username') || 'admin'
  const section = sectionOf(useLocation().pathname)
  const trail: Crumb[] = section ? [section, ...crumbs] : crumbs

  function handleLogout() {
    localStorage.removeItem('token')
    localStorage.removeItem('username')
    navigate('/login', { replace: true })
  }

  const core = lab.gnbExternal && !lab.coreUp ? 'external' : lab.runningTemplate ?? 'off'
  const gnb = lab.nodes.gnb.status === 'stopped' ? 'off' : lab.isTwoSlice ? '2 slices' : (lab.gnb.gnbDeployment?.profileName ?? 'on')

  return (
    <div className={styles.shell}>
      {overlay}
      <Sidebar onLogout={handleLogout} />
      <main className={`${styles.main} ${fill ? styles.mainFill : ''}`}>
        <header className={styles.bar}>
          <nav aria-label="Breadcrumb" className={styles.crumbs}>
            <span className={styles.product}>fru-lab</span>
            {trail.map((c, i) => {
              const last = i === trail.length - 1
              return (
                <Fragment key={i}>
                  <span aria-hidden="true" className={styles.slash}>/</span>
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
            <div className={styles.pills} aria-label="Lab status">
              <Pill to="/core-network" status={lab.gnbExternal && !lab.coreUp ? 'running' : lab.nodes.core.status} label="core" value={core} />
              <Pill to="/free-ran" status={lab.nodes.gnb.status} label="gnb" value={gnb} />
              <Pill to="/free-ue" status={lab.nodes.ue.status} label="ue" value={`${lab.ueCounts.running}/${lab.ueCounts.total}`} />
            </div>
            <span className={`num ${styles.dim}`}>{clock} UTC</span>
            <span className={styles.live} title={live ? 'API connected' : 'API not answering'}>
              <i className={live ? styles.st_running : styles.st_unhealthy} aria-hidden="true" />
              {live ? 'api' : 'api down'}
            </span>
            <span className={styles.dim}>{user}</span>
          </div>
        </header>
        <div className={fill ? styles.fill : styles.content}>{children}</div>
      </main>
    </div>
  )
}
