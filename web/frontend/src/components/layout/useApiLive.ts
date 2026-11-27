import { useEffect, useState } from 'react'
import { api } from '../../apiClient'

const POLL_MS = 5000

// useApiLive reports whether fru-lab's API answered the last poll, for the
// header's Live indicator and the status bar.
export function useApiLive(): boolean {
  const [live, setLive] = useState(true)

  useEffect(() => {
    let cancelled = false
    const read = async () => {
      if (document.hidden) return
      try {
        await api.deployFree5gcStatus()
        if (!cancelled) setLive(true)
      } catch {
        if (!cancelled) setLive(false)
      }
    }
    read()
    const timer = window.setInterval(read, POLL_MS)
    return () => { cancelled = true; window.clearInterval(timer) }
  }, [])

  return live
}
