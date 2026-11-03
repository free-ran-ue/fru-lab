import { Fragment, type ReactNode } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import Sidebar from '../sidebar/Sidebar'
import { useActiveRuns } from './useActiveRuns'
import styles from './layout.module.css'

export interface Crumb {
  label: ReactNode
  to?: string
}

interface AppLayoutProps {
  // the header's breadcrumb, outermost first; the last one is the page
  crumbs: Crumb[]
  // tags after the breadcrumb, e.g. "UNSAVED"
  crumbExtra?: ReactNode
  // the header's right side before Live / user, e.g. Setup's actions
  headerActions?: ReactNode
  // above everything (notifications)
  overlay?: ReactNode
  children: ReactNode
}

// AppLayout is every signed-in page: the sidebar, the white header bar
// (breadcrumb on the left; Live, the user and Log out on the right) and
// the content column.
export default function AppLayout({ crumbs, crumbExtra, headerActions, overlay, children }: AppLayoutProps) {
  const navigate = useNavigate()
  const { count, live } = useActiveRuns()
  const user = localStorage.getItem('username') || 'admin'

  function handleLogout() {
    localStorage.removeItem('token')
    localStorage.removeItem('username')
    navigate('/login', { replace: true })
  }

  return (
    <div className={styles.shell}>
      {overlay}
      <Sidebar activeRuns={count} />
      <main className={styles.main}>
        <header className={styles.bar}>
          <nav aria-label="Breadcrumb" className={styles.crumbs}>
            {crumbs.map((c, i) => {
              const last = i === crumbs.length - 1
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
            <span className={styles.live}>
              <span className={live ? styles.liveDot : styles.offDot} aria-hidden="true" />
              {live ? 'Live' : 'Offline'}
            </span>
            <span>{user}</span>
            <button type="button" className={styles.logout} onClick={handleLogout}>Log out</button>
          </div>
        </header>
        <div className={styles.content}>{children}</div>
      </main>
    </div>
  )
}
