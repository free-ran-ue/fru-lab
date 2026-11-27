import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import AppLayout from '../../components/layout/AppLayout'
import NotificationContainer from '../../components/notifications/NotificationContainer'
import { useNotifications } from '../../hooks/useNotifications'
import { extractErrorMessage } from '../../apiClient'
import CoreTabs from '../../lab/CoreTabs'
import SelectedNodePanel, { type DetailRow } from '../../lab/SelectedNodePanel'
import TopologySchematic from '../../lab/TopologySchematic'
import { StatusSwatch, StatusTag } from '../../lab/StatusTag'
import { getStatusMeta } from '../../lab/statusMeta'
import { FREE5GC_TEMPLATE_OPTIONS, type Free5gcTemplate } from '../../lab/free5gcTemplate'
import { templateLabel, useLab } from '../../lab/useLab'
import styles from '../../lab/lab.module.css'

// CoreNetworkPage deploys and stops the free5GC core and shows its network
// functions. Its Subscribers live in the second tab.
export default function CoreNetworkPage() {
  const navigate = useNavigate()
  const lab = useLab()
  const { free5gc, nodes } = lab
  const { errors, successes, addError, addSuccess, removeNotification } = useNotifications()
  // which template to deploy next; once running, the live one is shown
  const [selectedTemplate, setSelectedTemplate] = useState<Free5gcTemplate>('basic')
  const [selectedNf, setSelectedNf] = useState<string | null>(null)

  const core = nodes.core
  const coreStopped = core.status === 'stopped'
  const template = lab.runningTemplate ?? selectedTemplate
  const nfs = free5gc.networkFunctions
  const nf = nfs.find((n) => n.name === selectedNf) ?? null
  const blockedReason = coreStopped ? undefined : lab.blocked.coreStop

  async function handlePrimary() {
    try {
      if (!coreStopped) {
        await free5gc.stop()
        addSuccess('Core network stopped')
      } else {
        await free5gc.deploy(selectedTemplate)
        addSuccess('Core network deploy started')
      }
    } catch (error) {
      addError(extractErrorMessage(error, 'Core network action failed'))
    }
  }

  const primaryLabel = free5gc.isActionPending
    ? (coreStopped ? 'Deploying…' : 'Stopping…')
    : (coreStopped ? 'Deploy core' : 'Stop core')

  const rows: DetailRow[] = nf
    ? [
      { k: 'Service', v: nf.name.toLowerCase(), mono: true },
      { k: 'Part of', v: `free5GC · ${templateLabel(template)}` },
      { k: 'Core status', v: getStatusMeta(core.status).text },
      { k: 'Last deployed', v: core.lastDeployed, mono: true },
    ]
    : [
      { k: 'Stack', v: 'free5GC' },
      { k: 'Template', v: templateLabel(template) },
      { k: 'Network functions', v: nfs.length ? `${lab.healthyNfs} / ${nfs.length} healthy` : '—', mono: true },
      { k: 'Last deployed', v: core.lastDeployed, mono: true },
    ]

  return (
    <AppLayout
      crumbs={[{ label: 'Network functions' }]}
      overlay={<NotificationContainer errors={errors} successes={successes} onClose={removeNotification} />}
    >
      <CoreTabs />

      <section className={styles.pageHead}>
        <h1 className={styles.title}>Core Network</h1>
        <span className={styles.subtitle}>free5GC 5G core</span>
        <div className={styles.headActions}>
          {coreStopped && (
            <label className={styles.templatePicker}>
              <span>Template</span>
              <select
                className={styles.select}
                value={selectedTemplate}
                onChange={(event) => setSelectedTemplate(event.target.value as Free5gcTemplate)}
                disabled={free5gc.isActionPending}
              >
                {FREE5GC_TEMPLATE_OPTIONS.map((option) => (
                  <option key={option.value} value={option.value}>{option.label}</option>
                ))}
              </select>
            </label>
          )}
          <button type="button" className={styles.btn} onClick={() => navigate('/logs?target=core')}>View logs</button>
          <button
            type="button"
            className={`${styles.btn} ${coreStopped ? styles.btnPrimary : styles.btnDanger}`}
            onClick={handlePrimary}
            disabled={free5gc.isActionPending || Boolean(blockedReason)}
            title={blockedReason}
          >
            {primaryLabel}
          </button>
        </div>
      </section>

      <section className={styles.meta} aria-label="Core network status">
        <div><span>Status</span><b><StatusTag status={core.status} /></b></div>
        <div><span>Template</span><b>{templateLabel(template)}{coreStopped && <em>next deploy</em>}</b></div>
        <div><span>Healthy NFs</span><b><span className="num">{nfs.length ? `${lab.healthyNfs} / ${nfs.length}` : '—'}</span></b></div>
        <div><span>Subscribers</span><b><span className="num">{lab.ueCounts.total}</span></b></div>
        <div><span>Last deployed</span><b><span className="num">{core.lastDeployed}</span></b></div>
      </section>

      <section className={styles.panel} aria-label="Core network topology">
        <div className={styles.ph}>
          <h2>Core network topology</h2>
          {(['running', 'deploying', 'stopped', 'unhealthy'] as const).map((status) => (
            <span key={status} className={styles.legend}><StatusSwatch status={status} />{getStatusMeta(status).text}</span>
          ))}
          <span className={styles.aux}>{coreStopped ? `${templateLabel(template)} template preview` : `selected: ${nf?.name ?? 'free5GC core'}`}</span>
        </div>
        <TopologySchematic
          coreOnly
          template={template}
          templateLabel={templateLabel(template)}
          coreStatus={core.status}
          coreNfs={nfs}
          gnbStatus={lab.nodes.gnb.status}
          gnbTemplate=""
          gnbSlices={null}
          ueStatus={lab.nodes.ue.status}
          ueRows={[]}
          selection={{ node: 'core', nf: selectedNf, ue: null }}
          onSelect={(selection) => setSelectedNf(selection.nf)}
        />
      </section>

      <div className={styles.dash}>
        <div className={styles.dashMain}>
          <section className={styles.panel} aria-label="Network functions">
            <div className={styles.ph}>
              <h2>Network functions</h2>
              <span className={styles.aux}>{nfs.length ? 'select one for details' : ''}</span>
            </div>
            {nfs.length === 0 ? (
              <p className={styles.more}>The core network is not deployed. Pick a template and deploy it.</p>
            ) : (
              <div className={styles.nfGrid}>
                {nfs.map((n) => (
                  <button
                    key={n.name}
                    type="button"
                    className={styles.nfChip}
                    aria-pressed={n.name === selectedNf}
                    onClick={() => setSelectedNf(n.name === selectedNf ? null : n.name)}
                  >
                    <b>{n.name}</b>
                    <StatusTag status={n.status} />
                  </button>
                ))}
              </div>
            )}
          </section>
        </div>

        <SelectedNodePanel
          name={nf?.name ?? 'free5GC core'}
          status={nf?.status ?? core.status}
          rows={rows}
          hint={blockedReason}
          actions={nf ? <button type="button" className={styles.btn} onClick={() => setSelectedNf(null)}>Back to core</button> : null}
        />
      </div>
    </AppLayout>
  )
}
