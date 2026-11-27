import { useCallback } from 'react'
import { useToast } from '../components/feedback/Feedback'

// useNotifications posts the page's messages to the app's toasts (top
// right, gone after 5 s), so they outlive a page change.
export function useNotifications() {
  const toast = useToast()
  const addError = useCallback((message: string) => toast('error', message), [toast])
  const addSuccess = useCallback((message: string) => toast('success', message), [toast])
  const addInfo = useCallback((message: string) => toast('info', message), [toast])
  return { addError, addSuccess, addInfo }
}
