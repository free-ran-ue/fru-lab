import { useCallback, useEffect, useState } from 'react'
import { api } from '../apiClient'
import type { GnbInstance } from '../api'

const POLL_INTERVAL_MS = 4000

// useGnbInstances polls the gNBs deployed from gNB profiles (one per
// profile, several at once) and deploys / stops them by profile id.
export function useGnbInstances() {
  const [instances, setInstances] = useState<GnbInstance[]>([])
  // false until the first answer, so pages can tell "none" from "not known yet"
  const [loaded, setLoaded] = useState(false)
  // by profile id
  const [pending, setPending] = useState<Record<string, boolean>>({})

  const refresh = useCallback(async () => {
    try {
      const response = await api.deployGnbList()
      setInstances(response.data.instances ?? [])
    } finally {
      setLoaded(true)
    }
  }, [])

  useEffect(() => {
    // transient poll failures shouldn't blank out the last known state
    refresh().catch(() => {})
    const timer = setInterval(() => { refresh().catch(() => {}) }, POLL_INTERVAL_MS)
    return () => clearInterval(timer)
  }, [refresh])

  const withPending = useCallback(async (profileId: string, action: () => Promise<unknown>) => {
    setPending((cur) => ({ ...cur, [profileId]: true }))
    try {
      await action()
      await refresh()
    } finally {
      setPending((cur) => ({ ...cur, [profileId]: false }))
    }
  }, [refresh])

  // core: '' keeps the profile's, 'lab' is free5GC, else a core network id
  const deploy = useCallback((profileId: string, core = '') => withPending(profileId, () => api.deployGnbUp(profileId, core ? { core } : undefined)), [withPending])
  const stop = useCallback((profileId: string) => withPending(profileId, () => api.deployGnbDown(profileId)), [withPending])

  return { instances, loaded, pending, refresh, deploy, stop }
}
