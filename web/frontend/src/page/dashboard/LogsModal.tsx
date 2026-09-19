import type { DeploymentNode } from './types'
import styles from './logs-modal.module.css'

interface LogsModalProps {
  isOpen: boolean
  node: DeploymentNode
  logLines: string[]
  onClose: () => void
}

export default function LogsModal({ isOpen, node, logLines, onClose }: LogsModalProps) {
  if (!isOpen) return null

  return (
    <div className={styles.overlay}>
      <div className={styles.modal}>
        <div className={styles.header}>
          <span className={styles.title}>{`Logs — ${node.label}`}</span>
          <button className={styles.closeButton} onClick={onClose} aria-label="Close logs">✕</button>
        </div>
        <div className={styles.body}>
          {logLines.map((line, index) => (
            <div key={index} className={styles.line}>{line}</div>
          ))}
        </div>
      </div>
    </div>
  )
}
