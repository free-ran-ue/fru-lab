import { useNavigate } from 'react-router-dom'
import AppLayout from '../../components/layout/AppLayout'
import NotificationContainer from '../../components/notifications/NotificationContainer'
import { useNotifications } from '../../hooks/useNotifications'
import { extractErrorMessage } from '../../apiClient'
import SelectedNodePanel, { type DetailRow } from '../../lab/SelectedNodePanel'
import { StatusTag } from '../../lab/StatusTag'
import { SLICE1_SD, SLICE2_SD, templateLabel, useLab } from '../../lab/useLab'
import type { DeploymentNode } from '../../lab/types'
import styles from '../../lab/lab.module.css'

interface GnbRow {
  name: string
  sd: string
  node: DeploymentNode
  ues: number
}

// FreeRanPage deploys and stops the free-ran-ue gNB. Its template follows
// the core's: under ULCL (Two Slice) there is one gNB per slice, and
// Deploy / Stop act on both.
export default function FreeRanPage() {
  const navigate = useNavigate()
  const lab = useLab()
  const { gnb, gnbSlice1, gnbSlice2, nodes, isTwoSlice } = lab
  const { errors, successes, addError, addSuccess, removeNotification } = useNotifications()

  const node = nodes.gnb
  const gnbStopped = node.status === 'stopped'
  const isPending = isTwoSlice ? gnbSlice1.isActionPending || gnbSlice2.isActionPending : gnb.isActionPending
  const blockedReason = gnbStopped ? lab.blocked.gnbDeploy : lab.blocked.gnbStop

  const rows: GnbRow[] = isTwoSlice
    ? [
      { name: 'gNB slice 1', sd: SLICE1_SD, node: gnbSlice1.node, ues: lab.upUes.filter((r) => r.sd === SLICE1_SD).length },
      { name: 'gNB slice 2', sd: SLICE2_SD, node: gnbSlice2.node, ues: lab.upUes.filter((r) => r.sd === SLICE2_SD).length },
    ]
    : [{ name: 'gNB', sd: '—', node: gnb.node, ues: lab.upUes.length }]

  async function handlePrimary() {
    try {
      if (isTwoSlice) {
        if (!gnbStopped) {
          await Promise.allSettled([gnbSlice1.stop(), gnbSlice2.stop()])
          addSuccess('gNB stopped')
        } else {
          await Promise.allSettled([gnbSlice1.deploy(), gnbSlice2.deploy()])
          addSuccess('gNB deploy started')
        }
      } else if (!gnbStopped) {
        await gnb.stop()
        addSuccess('gNB stopped')
      } else {
        await gnb.deploy()
        addSuccess('gNB deploy started')
      }
    } catch (error) {
      addError(extractErrorMessage(error, 'gNB action failed'))
    }
  }

  const primaryLabel = isPending
    ? (gnbStopped ? 'Deploying…' : 'Stopping…')
    : (gnbStopped ? 'Deploy gNB' : 'Stop gNB')

  const template = isTwoSlice ? 'ulcl-2slice (two slices)' : 'basic'
  const details: DetailRow[] = [
    { k: 'Simulator', v: 'free-ran-ue' },
    { k: 'Template', v: template },
    { k: 'Core network', v: lab.runningTemplate ? `${templateLabel(lab.runningTemplate)} · ${nodes.core.status}` : 'not deployed' },
    { k: 'UEs attached', v: String(lab.upUes.length), mono: true },
    { k: 'Last deployed', v: node.lastDeployed, mono: true },
  ]

  return (
    <AppLayout
      crumbs={[{ label: 'gNB' }]}
      overlay={<NotificationContainer errors={errors} successes={successes} onClose={removeNotification} />}
    >
      <section className={styles.pageHead}>
        <h1 className={styles.title}>free-ran</h1>
        <span className={styles.subtitle}>free-ran-ue gNB simulator</span>
        <div className={styles.headActions}>
          <button type="button" className={styles.btn} onClick={() => navigate(`/logs?target=${isTwoSlice ? 'gnb-slice1' : 'gnb'}`)}>View logs</button>
          <button
            type="button"
            className={`${styles.btn} ${gnbStopped ? styles.btnPrimary : styles.btnDanger}`}
            onClick={handlePrimary}
            disabled={isPending || Boolean(blockedReason)}
            title={blockedReason}
          >
            {primaryLabel}
          </button>
        </div>
      </section>

      <section className={styles.meta} aria-label="gNB status">
        <div><span>Status</span><b><StatusTag status={node.status} /></b></div>
        <div><span>Template</span><b>{template}<em>follows the core</em></b></div>
        <div><span>Core network</span><b><StatusTag status={nodes.core.status} /></b></div>
        <div><span>UEs attached</span><b><span className="num">{lab.upUes.length}</span></b></div>
      </section>

      <div className={styles.dash}>
        <div className={styles.dashMain}>
          <section className={styles.panel} aria-label="gNBs">
            <div className={styles.ph}>
              <h2>gNB</h2>
              <span className={styles.aux}>{isTwoSlice ? 'one gNB per slice' : 'one gNB'}</span>
            </div>
            <div className={styles.tableWrap}>
              <table className={styles.tbl}>
                <thead>
                  <tr><th>gNB</th><th>Slice SD</th><th>Status</th><th className={styles.r}>UEs</th><th className={styles.r}>Last deployed</th></tr>
                </thead>
                <tbody>
                  {rows.map((row) => (
                    <tr key={row.name}>
                      <td>{row.name}</td>
                      <td className="num">{row.sd}</td>
                      <td><StatusTag status={row.node.status} /></td>
                      <td className={`num ${styles.r}`}>{row.ues}</td>
                      <td className={styles.r}>{row.node.lastDeployed}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </section>
        </div>

        <SelectedNodePanel
          name="gNB"
          status={node.status}
          rows={details}
          services={lab.gnbNetworkFunctions}
          hint={blockedReason}
          actions={null}
        />
      </div>
    </AppLayout>
  )
}
