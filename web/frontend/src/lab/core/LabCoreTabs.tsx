import { NavLink } from 'react-router-dom'
import styles from './lab-core-tabs.module.css'

const tabClass = ({ isActive }: { isActive: boolean }) => `${styles.tab} ${isActive ? styles.on : ''}`

// LabCoreTabs switches between the lab free5GC page's two parts: its
// network functions (deploy, topology) and its Subscribers.
export default function LabCoreTabs() {
  return (
    <nav className={styles.tabs} aria-label="free5GC">
      <NavLink to="/core-network/lab" end className={tabClass}>Network functions</NavLink>
      <NavLink to="/core-network/lab/subscribers" className={tabClass}>Subscribers</NavLink>
    </nav>
  )
}
