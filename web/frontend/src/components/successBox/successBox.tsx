import { useEffect } from 'react'
import styles from './successBox.module.css'

interface SuccessMessage {
  id: string
  message: string
  timestamp: number
}

interface SuccessBoxProps {
  successes: SuccessMessage[]
  onClose: (id: string) => void
  duration?: number
}

function SingleSuccess({ 
  success, 
  onClose, 
  duration = 3000,
  index
}: { 
  success: SuccessMessage
  onClose: (id: string) => void
  duration: number
  index: number
}) {
  useEffect(() => {
    const timer = setTimeout(() => {
      onClose(success.id)
    }, duration)
    return () => clearTimeout(timer)
  }, [success.id, duration, onClose])

  return (
    <div 
      className={styles.successBox} 
      style={{ 
        top: `${(index * 4.5) + 1}rem`,
        animationDelay: `${index * 0.05}s`
      }}
    >
      <svg className={styles.icon} width="16" height="16" viewBox="0 0 16 16" aria-hidden="true">
        <circle cx="8" cy="8" r="7" fill="currentColor" />
        <path d="M5 8.2l2 2 4-4.2" fill="none" stroke="var(--surface)" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" />
      </svg>
      <span className={styles.message}>{success.message}</span>
      <button 
        type="button" 
        className={styles.closeButton}
        onClick={() => onClose(success.id)}
        aria-label="Close success message"
      >
        <svg width="12" height="12" viewBox="0 0 12 12" aria-hidden="true">
          <path d="M2.5 2.5l7 7M9.5 2.5l-7 7" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
        </svg>
      </button>
    </div>
  )
}

export default function SuccessBox({ successes, onClose, duration = 3000 }: SuccessBoxProps) {
  return (
    <div className={styles.successBoxContainer}>
      {successes.map((success, index) => (
        <SingleSuccess
          key={success.id}
          success={success}
          onClose={onClose}
          duration={duration}
          index={index}
        />
      ))}
    </div>
  )
}
