import { NavLink } from 'react-router-dom'
import styles from './lab-core-tabs.module.css'

const tabClass = ({ isActive }: { isActive: boolean }) => `${styles.tab} ${isActive ? styles.on : ''}`

// LabCoreTabs switches between a free5GC page's two parts: its network
// functions (deploy, topology) and its Subscribers. `base` is the page's
// path: /core-network/lab for the lab free5GC, /core-network/n/<id> for a
// free5GC core network.
export default function LabCoreTabs({ base = '/core-network/lab', label = 'free5GC' }: { base?: string; label?: string }) {
  return (
    <nav className={styles.tabs} aria-label={label}>
      <NavLink to={base} end className={tabClass}>Network functions</NavLink>
      <NavLink to={`${base}/subscribers`} className={tabClass}>Subscribers</NavLink>
    </nav>
  )
}
