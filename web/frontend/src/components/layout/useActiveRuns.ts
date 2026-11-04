import { useEffect, useState } from 'react'
import { api } from '../../apiClient'

// the run states that count as "in progress" (the Runs list shows these)
const ACTIVE = ['configuring', 'n2', 'running', 'stopping']
const POLL_MS = 5000

export interface ActiveRuns {
  // runs in progress across every tester; null until the first answer
  count: number | null
  // whether fru-lab's API answered the last poll
  live: boolean
}

// useActiveRuns counts the runs going on across all testers, for the
// sidebar's Runs badge and the header's Live indicator.
export function useActiveRuns(): ActiveRuns {
  const [state, setState] = useState<ActiveRuns>({ count: null, live: true })

  useEffect(() => {
    let cancelled = false
    const read = async () => {
      if (document.hidden) return
      try {
        const testers = (await api.testersList()).data
        const runs = await Promise.allSettled(testers.map((t) => api.testerByIdRun(t.id)))
        const count = runs.filter((r) => r.status === 'fulfilled' && ACTIVE.includes(r.value.data.state)).length
        if (!cancelled) setState({ count, live: true })
      } catch {
        if (!cancelled) setState((cur) => ({ ...cur, live: false }))
      }
    }
    read()
    const timer = window.setInterval(read, POLL_MS)
    return () => { cancelled = true; window.clearInterval(timer) }
  }, [])

  return state
}
