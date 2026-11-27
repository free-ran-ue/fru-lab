import { useCallback, useEffect, useState } from 'react'
import { api } from '../apiClient'
import type { CoreNetwork } from '../api'

// The lab free5GC's fixed addresses on the lab network (see the backend's
// free5gc templates): AMF for N2, UPF for N3, and its one data network.
export const LAB_CORE = {
  name: 'free5GC',
  n2Ip: '10.0.1.3',
  n3Ip: '10.0.1.5',
  n2Interface: 'frulab-cn-ran',
  n3Interface: 'frulab-cn-ran',
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
