import { useCallback, useEffect, useState } from 'react'
import { api } from '../../apiClient'
import type { TesterEntry } from '../../api'

// TesterOption is a tester with the name it reports; reachable is null
// until it has answered or failed.
export interface TesterOption extends TesterEntry {
  name: string
  reachable: boolean | null
}

// useTesters lists the testers fru-lab can reach, each named by its own
// status (its URL until then, or when it does not answer). null while
// loading; [] when even the list failed.
export function useTesters(): { testers: TesterOption[] | null, reload: () => void } {
  const [testers, setTesters] = useState<TesterOption[] | null>(null)

  const reload = useCallback(() => {
    let cancelled = false
    api.testersList()
      .then((r) => {
        if (cancelled) return
        setTesters(r.data.map((t) => ({ ...t, name: t.url, reachable: null })))
        for (const t of r.data) {
          api.testerByIdStatus(t.id)
            .then((s) => { if (!cancelled) setTesters((cur) => cur?.map((x) => (x.id === t.id ? { ...x, name: s.data.name || x.url, reachable: true } : x)) ?? cur) })
            .catch(() => { if (!cancelled) setTesters((cur) => cur?.map((x) => (x.id === t.id ? { ...x, reachable: false } : x)) ?? cur) })
        }
      })
      .catch(() => { if (!cancelled) setTesters([]) })
    return () => { cancelled = true }
  }, [])

  useEffect(() => reload(), [reload])
  return { testers, reload }
}

// testerLabel names the tester a profile uses, or says it is gone.
export function testerLabel(testers: TesterOption[] | null, id: string | undefined): string {
  const t = testers?.find((x) => x.id === (id || 'default'))
  if (t) return t.name
  return testers === null ? '…' : 'removed tester'
}
