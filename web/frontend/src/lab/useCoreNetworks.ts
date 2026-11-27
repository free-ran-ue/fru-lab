import { useCallback, useEffect, useState } from 'react'
import { api } from '../apiClient'
import type { CoreNetwork } from '../api'

// The lab free5GC: its name and its one data network. Its addresses come
// from the backend's address plan (lab.labCore, from useIpam).
export const LAB_CORE = {
  name: 'free5GC',
  dns: [{ name: 'internet' }],
}

// useCoreNetworks holds the saved core networks (the cores outside the
// lab) for the Core Network pages and the sidebar.
export function useCoreNetworks() {
  const [networks, setNetworks] = useState<CoreNetwork[]>([])
  const [loaded, setLoaded] = useState(false)

  const refresh = useCallback(async () => {
    try {
      const r = await api.coreNetworkList()
      setNetworks([...r.data].sort((a, b) => a.name.localeCompare(b.name, undefined, { numeric: true, sensitivity: 'base' })))
    } finally {
      setLoaded(true)
    }
  }, [])

  useEffect(() => { refresh().catch(() => {}) }, [refresh])

  const save = useCallback(async (n: CoreNetwork) => {
    const r = n.id ? await api.coreNetworkUpdate(n.id, n) : await api.coreNetworkCreate(n)
    await refresh()
    return r.data
  }, [refresh])

  const remove = useCallback(async (id: string) => {
    await api.coreNetworkDelete(id)
    await refresh()
  }, [refresh])

  return { networks, loaded, refresh, save, remove }
}
