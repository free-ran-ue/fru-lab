import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from 'react'
import styles from './feedback.module.css'

// ---------- toasts ----------

type ToastKind = 'success' | 'error' | 'info'
interface Toast { id: number; kind: ToastKind; message: string }

const TOAST_MS = 5000

interface FeedbackApi {
  toast: (kind: ToastKind, message: string) => void
  confirm: (opts: ConfirmOptions) => Promise<boolean>
}

export interface ConfirmOptions {
  title: string
  message?: ReactNode
  // the confirm button's words, e.g. "Tear down"
  confirmLabel?: string
  // a destructive action: the confirm button is red
  danger?: boolean
}

const FeedbackContext = createContext<FeedbackApi | null>(null)

function ToastItem({ t, onClose }: { t: Toast; onClose: (id: number) => void }) {
  useEffect(() => {
    const timer = window.setTimeout(() => onClose(t.id), TOAST_MS)
    return () => window.clearTimeout(timer)
  }, [t.id, onClose])
  return (
    <div className={`${styles.toast} ${styles[`toast_${t.kind}`]}`} role={t.kind === 'error' ? 'alert' : 'status'}>
      <span className={styles.toastMark} aria-hidden="true" />
      <p className={styles.toastText}>{t.message}</p>
      <button type="button" className={styles.toastClose} onClick={() => onClose(t.id)} aria-label="Dismiss">×</button>
      <span className={styles.toastTimer} style={{ animationDuration: `${TOAST_MS}ms` }} aria-hidden="true" />
    </div>
  )
}

// ---------- confirm dialog ----------

interface Pending extends ConfirmOptions { resolve: (ok: boolean) => void }

function ConfirmDialog({ p, onDone }: { p: Pending; onDone: (ok: boolean) => void }) {
  const ok = useRef<HTMLButtonElement>(null)
  useEffect(() => {
    ok.current?.focus()
    const key = (e: KeyboardEvent) => {
      if (e.key === 'Escape') { e.preventDefault(); onDone(false) }
      if (e.key === 'Enter') { e.preventDefault(); onDone(true) }
    }
    window.addEventListener('keydown', key)
    return () => window.removeEventListener('keydown', key)
  }, [onDone])
  return (
    <div className={styles.backdrop} onMouseDown={(e) => { if (e.target === e.currentTarget) onDone(false) }}>
      <div className={styles.dialog} role="alertdialog" aria-modal="true" aria-labelledby="confirm-title">
        <h2 id="confirm-title" className={styles.dialogTitle}>{p.title}</h2>
        {p.message && <div className={styles.dialogBody}>{p.message}</div>}
        <div className={styles.dialogActions}>
          <button type="button" className={styles.btn} onClick={() => onDone(false)}>Cancel</button>
          <button ref={ok} type="button" className={`${styles.btn} ${p.danger ? styles.btnDanger : styles.btnPrimary}`} onClick={() => onDone(true)}>{p.confirmLabel ?? 'Confirm'}</button>
        </div>
      </div>
    </div>
  )
}

// FeedbackProvider is the app's one place for messages: toasts stacked in
// the top-right corner (each gone after 5 s) and the confirm dialog that
// replaces the browser's own.
export function FeedbackProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([])
  const [pending, setPending] = useState<Pending | null>(null)
  const next = useRef(1)

  const close = useCallback((id: number) => setToasts((cur) => cur.filter((t) => t.id !== id)), [])
  const toast = useCallback((kind: ToastKind, message: string) => {
    const id = next.current++
    setToasts((cur) => [...cur.slice(-5), { id, kind, message }])
  }, [])
  const confirm = useCallback((opts: ConfirmOptions) => new Promise<boolean>((resolve) => setPending({ ...opts, resolve })), [])
  const done = useCallback((ok: boolean) => {
    setPending((p) => { p?.resolve(ok); return null })
  }, [])

  return (
    <FeedbackContext.Provider value={{ toast, confirm }}>
      {children}
      <div className={styles.stack} aria-live="polite">
        {toasts.map((t) => <ToastItem key={t.id} t={t} onClose={close} />)}
      </div>
      {pending && <ConfirmDialog p={pending} onDone={done} />}
    </FeedbackContext.Provider>
  )
}

function useFeedback(): FeedbackApi {
  const api = useContext(FeedbackContext)
  if (!api) throw new Error('useFeedback needs a FeedbackProvider')
  return api
}

// useConfirm asks before a destructive action: await confirm({...}) is
// true when the user confirmed.
export function useConfirm() {
  return useFeedback().confirm
}

export function useToast() {
  return useFeedback().toast
}
