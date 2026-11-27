import { useCallback, useEffect, useMemo, useState } from 'react'
import { api } from '../apiClient'
import type { UeProfile } from '../api'
import { profileUeId } from './useProfiles'
import type { NodeStatus } from './types'

const POLL_INTERVAL_MS = 4000

interface UeInstanceStatus {
  status: NodeStatus
  lastDeployed: string
  // the gNB target it was deployed onto ('' when unknown)
  gnb: string
}

// One row per UE profile (each profile is one UE), plus any other instance
// still deployed that matches no profile. A UE never deployed shows as
// "stopped" with no logs yet.
export interface UeRow {
  // where its config comes from
  source: 'profile' | 'other'
  profileId?: string
  profileName?: string
  ueId: string
  plmnId: string
  status: NodeStatus
  hasBeenDeployed: boolean
  lastDeployed: string
  // the profile's sd ('' for other instances); only a fallback for
  // which gNB a UE is on when its status has no recorded gnb
  sd: string
  // the gNB target the backend recorded for it (gnb-<profileId>); ''
  // when unknown or never deployed
  gnb: string
}

export function useUeInstances({ profiles }: { profiles: UeProfile[] }) {
  const [instancesByUeId, setInstancesByUeId] = useState<Record<string, UeInstanceStatus>>({})
  const [isLoadingInstances, setIsLoadingInstances] = useState(true)
  const [pendingInstances, setPendingInstances] = useState<Record<string, boolean>>({})

  const refreshInstances = useCallback(async () => {
    try {
      const response = await api.deployUeList()
      const map: Record<string, UeInstanceStatus> = {}
      for (const item of response.data.instances ?? []) {
        map[item.instance] = {
          status: item.status as NodeStatus,
          lastDeployed: item.lastDeployed ? new Date(item.lastDeployed).toLocaleString() : '—',
          gnb: item.gnb ?? '',
        }
      }
      setInstancesByUeId(map)
    } finally {
      setIsLoadingInstances(false)
    }
  }, [])

  useEffect(() => {
    // transient poll failures shouldn't blank out the last known state
    refreshInstances().catch(() => {})
    const timer = setInterval(() => { refreshInstances().catch(() => {}) }, POLL_INTERVAL_MS)
    return () => clearInterval(timer)
  }, [refreshInstances])

  const rows: UeRow[] = useMemo(() => {
    const out: UeRow[] = []
    const seen = new Set<string>()
    const state = (ueId: string) => {
      const instance = instancesByUeId[ueId]
      return {
        status: instance?.status ?? 'stopped' as NodeStatus,
        hasBeenDeployed: Boolean(instance),
        lastDeployed: instance?.lastDeployed ?? '—',
        gnb: instance?.gnb ?? '',
      }
    }
    for (const p of profiles) {
      const ueId = profileUeId(p)
      if (seen.has(ueId)) continue
      seen.add(ueId)
      out.push({ source: 'profile', profileId: p.id, profileName: p.name, ueId, plmnId: `${p.mcc}${p.mnc}`, sd: p.sd ?? '', ...state(ueId) })
    }
    // deployed before, from no saved profile: can still be stopped
    for (const ueId of Object.keys(instancesByUeId)) {
      if (seen.has(ueId)) continue
      out.push({ source: 'other', ueId, plmnId: '—', sd: '', ...state(ueId) })
    }
    return out
  }, [profiles, instancesByUeId])

  const deployRow = useCallback(async (ueId: string, gnb?: string) => {
    const row = rows.find((r) => r.ueId === ueId)
    if (!row?.profileId) throw new Error(`${ueId} has no UE profile to deploy from`)
    await api.deployUeProfileUp(row.profileId, gnb ? { gnb } : undefined)
  }, [rows])

  const withPending = useCallback(async (ueId: string, action: () => Promise<unknown>) => {
    setPendingInstances((current) => ({ ...current, [ueId]: true }))
    try {
      await action()
      await refreshInstances()
    } finally {
      setPendingInstances((current) => ({ ...current, [ueId]: false }))
    }
  }, [refreshInstances])

  // deploy starts a profile's UE; gnb picks the gNB target (omitted: the
  // backend picks one by SD)
  const deploy = useCallback(
    (ueId: string, gnb?: string) => withPending(ueId, () => deployRow(ueId, gnb)),
    [withPending, deployRow],
  )

  // deployMany starts UEs one after another (each waits for its container)
  // and reports progress; it returns the ids that failed.
  const deployMany = useCallback(async (ueIds: string[], gnb?: string, onProgress?: (done: number) => void): Promise<string[]> => {
    const failed: string[] = []
    let done = 0
    for (const ueId of ueIds) {
      setPendingInstances((current) => ({ ...current, [ueId]: true }))
      try {
        await deployRow(ueId, gnb)
      } catch {
        failed.push(ueId)
      } finally {
        setPendingInstances((current) => ({ ...current, [ueId]: false }))
        onProgress?.(++done)
        refreshInstances().catch(() => {})
      }
    }
    return failed
  }, [deployRow, refreshInstances])

  // stopMany stops the given UEs in parallel.
  const stopMany = useCallback(async (ueIds: string[]) => {
    if (ueIds.length === 0) return
    setPendingInstances((current) => Object.fromEntries([...Object.entries(current), ...ueIds.map((id) => [id, true])]))
    try {
      const results = await Promise.allSettled(ueIds.map((ueId) => api.deployUeDown(ueId)))
      const failedCount = results.filter((r) => r.status === 'rejected').length
      if (failedCount > 0) throw new Error(`Failed to stop ${failedCount} of ${ueIds.length} UE instance(s)`)
    } finally {
      await refreshInstances()
      setPendingInstances((current) => Object.fromEntries([...Object.entries(current), ...ueIds.map((id) => [id, false])]))
    }
  }, [refreshInstances])

  const stop = useCallback(
    (ueId: string) => withPending(ueId, () => api.deployUeDown(ueId)),
    [withPending],
  )

  const stopAll = useCallback(
    () => stopMany(rows.filter((row) => row.status !== 'stopped').map((row) => row.ueId)),
    [rows, stopMany],
  )

  const fetchLogs = useCallback(async (ueId: string): Promise<string[]> => {
    const response = await api.deployUeLogs(ueId)
    return response.data.lines ?? []
  }, [])

  return {
    rows,
    isLoadingInstances,
    pendingInstances,
    deploy,
    deployMany,
    stop,
    stopMany,
    stopAll,
    fetchLogs,
  }
}
