import { NavLink, useNavigate } from 'react-router-dom'
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

export default function Sidebar() {
  const navigate = useNavigate()

  function handleLogout() {
    localStorage.removeItem('token')
    navigate('/login', { replace: true })
  }

  return (
    <aside className={styles.sidebar}>
      <div>
        <div className={styles.brand}>
          <LogoMark />
          <div className={styles.brandText}>
            <span className={styles.wordmark}>fru-<span className={styles.wordmarkLab}>lab</span></span>
            <span className={styles.tagline}>5G core test console</span>
          </div>
        </div>

        <p className={styles.navGroup}>Core network</p>
        <nav className={styles.nav} aria-label="Core network">
          <NavLink to="/" end className={navItemClassName}>Dashboard</NavLink>
          <NavLink to="/subscribers" className={navItemClassName}>Subscribers</NavLink>
          <NavLink to="/logs" className={navItemClassName}>Logs</NavLink>
          <NavLink to="/images" className={navItemClassName}>Images</NavLink>
        </nav>

        <p className={styles.navGroup}>Load testing</p>
        <nav className={styles.nav} aria-label="Load testing">
          <NavLink to="/tester/status" className={navItemClassName}>Testers</NavLink>
          <NavLink to="/tester" end className={navItemClassName}>Setup</NavLink>
          <NavLink to="/tester/profiles" className={navItemClassName}>Profiles</NavLink>
          <NavLink to="/tester/run" className={navItemClassName}>Runs</NavLink>
          <NavLink to="/tester/history" className={navItemClassName}>History</NavLink>
          <NavLink to="/tester/bench" className={navItemClassName}>Bench</NavLink>
        </nav>
      </div>

      <button type="button" className={styles.logout} onClick={handleLogout}>
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
          <path d="M6 2.5H3.5v11H6M10 5l3 3-3 3M13 8H6" />
        </svg>
        Log out
      </button>
    </aside>
  )
}
