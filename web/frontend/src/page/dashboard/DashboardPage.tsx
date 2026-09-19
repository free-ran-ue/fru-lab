import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import Button from '../../components/button/button'
import StatsCard from '../../components/stats/stats-card'
import TopologyCanvas from './TopologyCanvas'
import DetailPanel from './DetailPanel'
import LogsModal from './LogsModal'
import { MOCK_NODES, MOCK_NETWORK_FUNCTIONS, getMockLogLines } from './mockData'
import type { NodeId } from './types'
import styles from './dashboard-page.module.css'

export default function DashboardPage() {
  const navigate = useNavigate()
  const [selected, setSelected] = useState<NodeId>('core')
  const [showLogs, setShowLogs] = useState(false)

  function handleLogout() {
    localStorage.removeItem('token')
    navigate('/login', { replace: true })
  }

  const healthyCount = MOCK_NETWORK_FUNCTIONS.filter((nf) => nf.status === 'running').length
  const totalNfs = MOCK_NETWORK_FUNCTIONS.length
  const unhealthyCount = totalNfs - healthyCount
  const selectedNode = MOCK_NODES[selected]

  return (
    <div className={styles.layout}>
      <aside className={styles.sidebar}>
        <div>
          <p className={styles.badge}>FRU-LAB</p>
          <h1 className={styles.brand}>5G Lab Console</h1>

          <nav className={styles.nav}>
            <a className={`${styles.navItem} ${styles.navItemActive}`} href="#">Dashboard</a>
            <a className={styles.navItem} href="#">Settings</a>
          </nav>
        </div>

        <div>
          <Button variant="secondary" onClick={handleLogout}>Logout</Button>
        </div>
      </aside>

      <main className={styles.content}>
        <header>
          <h2 className={styles.title}>Lab Dashboard</h2>
          <p className={styles.subtitle}>
            Deploy and monitor the free5GC core network and the free-ran-ue gNB / UE simulator
            (built-in templates for now, custom parameters coming later)
          </p>
        </header>

        <section className={styles.statsGrid}>
          <StatsCard
            title="Core Network"
            value={getStatusLabel(MOCK_NODES.core.status)}
            description="free5GC · basic template"
          />
          <StatsCard
            title="gNB"
            value={getStatusLabel(MOCK_NODES.gnb.status)}
            description="free-ran-ue · basic template"
          />
          <StatsCard
            title="UE"
            value={getStatusLabel(MOCK_NODES.ue.status)}
            description="free-ran-ue · basic template"
          />
          <StatsCard
            title="Healthy NFs"
            value={`${healthyCount} / ${totalNfs}`}
            description={unhealthyCount > 0
              ? `${unhealthyCount} NF${unhealthyCount === 1 ? '' : 's'} failing health check`
              : 'All network functions healthy'}
          />
        </section>

        <section className={styles.topologySection}>
          <div className={styles.topologyHeader}>
            <div>
              <h3>Network Topology</h3>
              <p>Click a node to see its status and actions. Drag nodes to rearrange the layout.</p>
            </div>
            <div className={styles.legend}>
              <span className={styles.legendItem}>
                <span className={`${styles.legendDot} ${styles.legendDotRunning}`} />Running
              </span>
              <span className={styles.legendItem}>
                <span className={`${styles.legendDot} ${styles.legendDotStopped}`} />Stopped
              </span>
              <span className={styles.legendItem}>
                <span className={`${styles.legendDot} ${styles.legendDotUnhealthy}`} />Unhealthy
              </span>
            </div>
          </div>

          <div className={styles.topologyBody}>
            <div className={styles.canvasWrap}>
              <TopologyCanvas nodes={MOCK_NODES} selected={selected} onSelect={setSelected} />
            </div>

            <DetailPanel
              node={selectedNode}
              networkFunctions={MOCK_NETWORK_FUNCTIONS}
              onViewLogs={() => setShowLogs(true)}
            />
          </div>
        </section>
      </main>

      <LogsModal
        isOpen={showLogs}
        node={selectedNode}
        logLines={getMockLogLines(selected)}
        onClose={() => setShowLogs(false)}
      />
    </div>
  )
}

function getStatusLabel(status: string): string {
  return status.charAt(0).toUpperCase() + status.slice(1)
}
