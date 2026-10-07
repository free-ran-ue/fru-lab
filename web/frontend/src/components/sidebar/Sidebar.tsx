import { NavLink, useNavigate } from 'react-router-dom'
import Button from '../button/button'
import styles from './sidebar.module.css'

function navItemClassName({ isActive }: { isActive: boolean }): string {
  return `${styles.navItem} ${isActive ? styles.navItemActive : ''}`
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
        <p className={styles.badge}>FRU-LAB</p>
        <h1 className={styles.brand}>5G Lab Console</h1>

        <nav className={styles.nav}>
          <NavLink to="/" end className={navItemClassName}>Dashboard</NavLink>
          <NavLink to="/subscribers" className={navItemClassName}>5G Subscriber</NavLink>
          <NavLink to="/logs" className={navItemClassName}>Logs</NavLink>
          <NavLink to="/images" className={navItemClassName}>Images</NavLink>
        </nav>

        <p className={styles.navGroup}>Throughput Tester</p>
        <nav className={styles.nav}>
          <NavLink to="/tester" end className={navItemClassName}>Setup</NavLink>
          <NavLink to="/tester/profiles" className={navItemClassName}>Profiles</NavLink>
          <NavLink to="/tester/run" className={navItemClassName}>Run</NavLink>
          <NavLink to="/tester/history" className={navItemClassName}>History</NavLink>
          <NavLink to="/tester/bench" className={navItemClassName}>Bench</NavLink>
        </nav>
      </div>

      <div>
        <Button variant="secondary" onClick={handleLogout}>Logout</Button>
      </div>
    </aside>
  )
}
