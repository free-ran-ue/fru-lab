import { useCallback, useEffect, useState } from 'react'
import { api } from '../apiClient'
import type { CoreDeployStatus, RequestDeployFree5gcTemplateEnum } from '../api'
import { detectFree5gcTemplate, type Free5gcTemplate } from './free5gcTemplate'
import type { NetworkFunction, NodeStatus } from './types'

const POLL_INTERVAL_MS = 4000

// CoreDeploy is how one free5GC core network (kind free5gc) runs right now.
export interface CoreDeploy {
  coreId: string
  status: NodeStatus
  networkFunctions: NetworkFunction[]
  // the template it runs; null while stopped
  template: Free5gcTemplate | null
  lastDeployed: string
}

const STOPPED = (coreId: string): CoreDeploy => ({ coreId, status: 'stopped', networkFunctions: [], template: null, lastDeployed: '—' })

function toDeploy(c: CoreDeployStatus): CoreDeploy {
  const networkFunctions = (c.services ?? []).map((s) => ({ name: s.name.toUpperCase(), status: s.status as NodeStatus }))
  const t = c.template === 'basic' || c.template === 'ulcl' ? c.template : null
  return {
    coreId: c.coreId,
    status: (c.status || 'stopped') as NodeStatus,
    networkFunctions,
    // the backend's word first; else read it back off the live NFs
    template: t ?? detectFree5gcTemplate(networkFunctions),
    lastDeployed: c.lastDeployed ? new Date(c.lastDeployed).toLocaleString() : '—',
  }
}

// useCoreDeploys polls the free5GC core networks (the extra free5GCs on this
// host) and deploys / stops them by core network id. The lab free5GC stays
// with useFree5gcStatus.
export function useCoreDeploys() {
  const [byId, setById] = useState<Record<string, CoreDeploy>>({})
  // false until the first answer, so pages can tell "stopped" from "not known yet"
  const [loaded, setLoaded] = useState(false)
  // by core id
  const [pending, setPending] = useState<Record<string, boolean>>({})

  const refresh = useCallback(async () => {
    try {
      const r = await api.deployCoreList()
      setById(Object.fromEntries((r.data.cores ?? []).map((c) => [c.coreId, toDeploy(c)])))
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

  const withPending = useCallback(async (id: string, action: () => Promise<unknown>) => {
    setPending((cur) => ({ ...cur, [id]: true }))
    try {
      await action()
      await refresh()
    } finally {
      setPending((cur) => ({ ...cur, [id]: false }))
    }
  }, [refresh])

  const deploy = useCallback((id: string, template: Free5gcTemplate) => withPending(id, () => api.deployCoreUp(id, { template: template as RequestDeployFree5gcTemplateEnum })), [withPending])
  const stop = useCallback((id: string) => withPending(id, () => api.deployCoreDown(id)), [withPending])
  const get = useCallback((id: string): CoreDeploy => byId[id] ?? STOPPED(id), [byId])

  return { byId, loaded, pending, refresh, deploy, stop, get }
}
