import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import Button from '../../components/button/button'
import StatsCard from '../../components/stats/stats-card'
import NotificationContainer from '../../components/notifications/NotificationContainer'
import { useNotifications } from '../../hooks/useNotifications'
import { extractErrorMessage } from '../../apiClient'
import TopologyCanvas from './TopologyCanvas'
import DetailPanel from './DetailPanel'
import LogsModal from './LogsModal'
import { useFree5gcStatus } from './useFree5gcStatus'
import { getStatusMeta } from './statusMeta'
import { MOCK_NODES, getMockLogLines } from './mockData'
import type { DeploymentNode, NodeId } from './types'
import styles from './dashboard-page.module.css'

export default function DashboardPage() {
  const navigate = useNavigate()
  const [selected, setSelected] = useState<NodeId>('core')
  const [showLogs, setShowLogs] = useState(false)

  const { errors, successes, addError, addSuccess, removeNotification } = useNotifications()
  const free5gc = useFree5gcStatus()

  function handleLogout() {
    localStorage.removeItem('token')
    navigate('/login', { replace: true })
  }

  async function handlePrimaryAction() {
    try {
      if (free5gc.node.status === 'running') {
        await free5gc.stop()
        addSuccess('free5GC stopped')
      } else {
        await free5gc.deploy()
        addSuccess('free5GC deploy started')
      }
    } catch (error) {
      addError(extractErrorMessage(error, 'free5GC action failed'))
    }
  }

  async function handleViewLogs() {
    setShowLogs(true)
    if (selected !== 'core') return

    try {
      await free5gc.fetchLogs()
    } catch (error) {
      addError(extractErrorMessage(error, 'Failed to load logs'))
    }
  }

  const nodes: Record<NodeId, DeploymentNode> = {
    core: free5gc.node,
    gnb: MOCK_NODES.gnb,
    ue: MOCK_NODES.ue,
  }
  const selectedNode = nodes[selected]

  const healthyCount = free5gc.networkFunctions.filter((nf) => nf.status === 'running').length
  const totalNfs = free5gc.networkFunctions.length
  const unhealthyCount = totalNfs - healthyCount

  return (
    <div className={styles.layout}>
      <NotificationContainer
        errors={errors}
        successes={successes}
        onClose={removeNotification}
      />

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
            value={getStatusLabel(nodes.core.status)}
            description="free5GC · basic template"
            valueColor={getStatusMeta(nodes.core.status).color}
          />
          <StatsCard
            title="gNB"
            value={getStatusLabel(nodes.gnb.status)}
            description="free-ran-ue · basic template"
            valueColor={getStatusMeta(nodes.gnb.status).color}
          />
          <StatsCard
            title="UE"
            value={getStatusLabel(nodes.ue.status)}
            description="free-ran-ue · basic template"
            valueColor={getStatusMeta(nodes.ue.status).color}
          />
          <StatsCard
            title="Healthy NFs"
            value={totalNfs > 0 ? `${healthyCount} / ${totalNfs}` : '—'}
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
              <TopologyCanvas nodes={nodes} selected={selected} onSelect={setSelected} />
            </div>

            <DetailPanel
              node={selectedNode}
              networkFunctions={free5gc.networkFunctions}
              onViewLogs={handleViewLogs}
              onPrimaryAction={selected === 'core' ? handlePrimaryAction : undefined}
              isActionPending={selected === 'core' && free5gc.isActionPending}
            />
          </div>
        </section>
      </main>

      <LogsModal
        isOpen={showLogs}
        node={selectedNode}
        logLines={selected === 'core' ? free5gc.logLines : getMockLogLines(selected)}
        isLoading={selected === 'core' && free5gc.isLoadingLogs}
        onClose={() => setShowLogs(false)}
      />
    </div>
  )
}

function getStatusLabel(status: string): string {
  return status.charAt(0).toUpperCase() + status.slice(1)
}
