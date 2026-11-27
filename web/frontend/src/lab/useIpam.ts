import { useCallback, useEffect, useState } from 'react'
import { api } from '../apiClient'
import type { IpamPool, IpamView } from '../api'

const POLL_INTERVAL_MS = 10000

// useIpam holds the backend's address plan and leases (GET /api/ipam): the
// one place the UI learns any lab address from.
export function useIpam() {
  const [view, setView] = useState<IpamView | null>(null)
  const refresh = useCallback(async () => {
    const r = await api.ipamView()
    setView(r.data)
  }, [])
  useEffect(() => {
    refresh().catch(() => {})
    const timer = setInterval(() => { refresh().catch(() => {}) }, POLL_INTERVAL_MS)
    return () => clearInterval(timer)
  }, [refresh])
  return { view, pools: view?.pools ?? [], warnings: view?.warnings ?? [], refresh }
}

// the address a pool reserves for a role (amf, upf, ...)
export const reservedAddr = (pool: IpamPool | undefined, role: string) => pool?.reserved.find((r) => r.role === role)?.addr

// coreAddresses is a free5GC core's N2 (AMF) and N3 (UPF) side, read from
// its cn-ran pool: `cn-ran` for the lab free5GC, `cn-ran-<id>` for a core
// network.
export function coreAddresses(pools: IpamPool[], coreId?: string) {
  const pool = pools.find((p) => p.id === (coreId ? `cn-ran-${coreId}` : 'cn-ran'))
  return {
    n2Ip: reservedAddr(pool, 'amf') ?? '—',
    n3Ip: reservedAddr(pool, 'upf') ?? reservedAddr(pool, 'i-upf') ?? '—',
    n2Interface: pool?.network ?? '—',
    n3Interface: pool?.network ?? '—',
  }
}
