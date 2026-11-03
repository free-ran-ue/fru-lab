import { useEffect } from 'react'
import styles from './errorBox.module.css'

interface ErrorMessage {
  id: string
  message: string
  timestamp: number
}

interface ErrorBoxProps {
  errors: ErrorMessage[]
  onClose: (id: string) => void
  duration?: number
}

function SingleError({ 
  error, 
  onClose, 
  duration = 5000,
  index
}: { 
  error: ErrorMessage
  onClose: (id: string) => void
  duration: number
  index: number
}) {
  useEffect(() => {
    const timer = setTimeout(() => {
      onClose(error.id)
    }, duration)
    return () => clearTimeout(timer)
  }, [error.id, duration, onClose])

  return (
    <div 
      className={styles.errorBox} 
      style={{ 
        top: `${(index * 4.5) + 1}rem`,
        animationDelay: `${index * 0.05}s`
      }}
    >
      <svg className={styles.icon} width="16" height="16" viewBox="0 0 16 16" aria-hidden="true">
        <circle cx="8" cy="8" r="7" fill="currentColor" />
        <path d="M8 4.5v4.2" stroke="var(--surface)" strokeWidth="1.6" strokeLinecap="round" />
        <circle cx="8" cy="11.2" r="0.9" fill="var(--surface)" />
      </svg>
      <span className={styles.message}>{error.message}</span>
      <button 
        type="button" 
        className={styles.closeButton}
        onClick={() => onClose(error.id)}
        aria-label="Close error message"
      >
        <svg width="12" height="12" viewBox="0 0 12 12" aria-hidden="true">
          <path d="M2.5 2.5l7 7M9.5 2.5l-7 7" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
        </svg>
      </button>
    </div>
  )
}

export default function ErrorBox({ errors, onClose, duration = 5000 }: ErrorBoxProps) {
  return (
    <div className={styles.errorBoxContainer}>
      {errors.map((error, index) => (
        <SingleError
          key={error.id}
          error={error}
          onClose={onClose}
          duration={duration}
          index={index}
        />
      ))}
    </div>
  )
}
