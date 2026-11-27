import { NavLink } from 'react-router-dom'
import styles from './core-tabs.module.css'

const tabClass = ({ isActive }: { isActive: boolean }) => `${styles.tab} ${isActive ? styles.tabOn : ''}`

// CoreTabs switches between the Core Network page's two parts: the free5GC
// deployment and its Subscribers.
export default function CoreTabs() {
  return (
    <nav className={styles.tabs} aria-label="Core Network">
      <NavLink to="/core-network" end className={tabClass}>Network functions</NavLink>
      <NavLink to="/core-network/subscribers" className={tabClass}>Subscribers</NavLink>
    </nav>
  )
}
