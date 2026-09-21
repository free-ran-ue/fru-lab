import Button from '../../components/button/button'
import { getStatusMeta } from './statusMeta'
import type { DeploymentNode, NetworkFunction } from './types'
import styles from './detail-panel.module.css'

interface DetailPanelProps {
  node: DeploymentNode
  networkFunctions: NetworkFunction[]
  onViewLogs: () => void
  onPrimaryAction?: () => void
  isActionPending?: boolean
  // set when the currently-shown primary action (deploy or stop) would
  // violate the network's dependency order - e.g. stopping a gNB a UE is
  // still dialing, or deploying a gNB before the core is up. Disables the
  // button and explains why instead of only erroring after the click.
  actionBlockedReason?: string
  // only free5gc has more than one deploy template right now - every other
  // target leaves these unset, and the picker doesn't render at all.
  templateOptions?: { value: string; label: string }[]
  selectedTemplate?: string
  onTemplateChange?: (value: string) => void
}

export default function DetailPanel({ node, networkFunctions, onViewLogs, onPrimaryAction, isActionPending = false, actionBlockedReason, templateOptions, selectedTemplate, onTemplateChange }: DetailPanelProps) {
  const meta = getStatusMeta(node.status)
  const isRunning = node.status === 'running'
  const isActionBlocked = Boolean(actionBlockedReason)

  const primaryActionLabel = isActionPending
    ? (isRunning ? 'Stopping…' : 'Deploying…')
    : (isRunning ? 'Stop Service' : 'Deploy Service')

  return (
    <div className={styles.panel}>
      <div>
        <div className={styles.eyebrow}>Selected node</div>
        <h4 className={styles.nodeLabel}>{node.label}</h4>
        <p className={styles.nodeSublabel}>{node.sublabel}</p>
        <span className={styles.statusPill} style={{ background: meta.bg, color: meta.color, borderColor: meta.border }}>
          <span className={styles.statusDot} style={{ background: meta.dot }} />
          {meta.text}
        </span>
      </div>

      <div className={styles.divider} />

      <div>
        <div className={styles.sectionTitle}>Services</div>
        <div className={styles.nfGrid}>
          {networkFunctions.map((nf) => {
            const nfMeta = getStatusMeta(nf.status)
            return (
              <div key={nf.name} className={styles.nfChip} style={{ background: nfMeta.bg, color: nfMeta.color, borderColor: nfMeta.border }}>
                <span className={styles.nfDot} style={{ background: nfMeta.dot }} />
                {nf.name}
              </div>
            )
          })}
        </div>
      </div>

      <div className={styles.divider} />

      <div className={styles.metaList}>
        <div className={styles.metaRow}>
          <span>Last deployed</span>
          <span className={styles.metaValue}>{node.lastDeployed}</span>
        </div>
      </div>

      <div className={styles.actions}>
        {templateOptions && !isRunning && (
          <label className={styles.templateSelector}>
            <span>Template</span>
            <select
              className={styles.templateSelect}
              value={selectedTemplate}
              onChange={(event) => onTemplateChange?.(event.target.value)}
              disabled={isActionPending}
            >
              {templateOptions.map((option) => (
                <option key={option.value} value={option.value}>{option.label}</option>
              ))}
            </select>
          </label>
        )}
        {isActionBlocked && (
          <p className={styles.blockedHint}>{actionBlockedReason}</p>
        )}
        <Button variant="primary" onClick={onPrimaryAction} disabled={!onPrimaryAction || isActionPending || isActionBlocked}>
          {primaryActionLabel}
        </Button>
        <Button variant="secondary" onClick={onViewLogs}>View Logs</Button>
      </div>
    </div>
  )
}
