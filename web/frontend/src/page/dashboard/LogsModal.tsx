import type { DeploymentNode } from './types'
import styles from './logs-modal.module.css'

interface LogsModalProps {
  isOpen: boolean
  node: DeploymentNode
  logLines: string[]
  isLoading?: boolean
  onClose: () => void
}

export default function LogsModal({ isOpen, node, logLines, isLoading = false, onClose }: LogsModalProps) {
  if (!isOpen) return null

  return (
    <div className={styles.overlay}>
      <div className={styles.modal}>
        <div className={styles.header}>
          <span className={styles.title}>{`Logs — ${node.label}`}</span>
          <button className={styles.closeButton} onClick={onClose} aria-label="Close logs">✕</button>
        </div>
        <div className={styles.body}>
          {isLoading ? (
            <div className={styles.line}>Loading…</div>
          ) : logLines.length === 0 ? (
            <div className={styles.line}>No log output yet.</div>
          ) : (
            logLines.map((line, index) => (
              <div key={index} className={styles.line}>{line}</div>
            ))
          )}
        </div>
      </div>
    </div>
  )
}
