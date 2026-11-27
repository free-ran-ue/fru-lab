import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { api } from '../apiClient'
import { webconsoleApi, extractWebconsoleErrorMessage } from '../webconsoleApiClient'
import type { Subscriber } from '../webconsoleApi'
import type { DeployUeRequest, UeProfile } from '../api'
import { msinAt } from './useProfiles'
import { subscriptionToUeDeployRequest } from './ueDeployFromSubscription'
import { fromSubscription } from '../page/subscribers/subscriberForm'
import type { NodeStatus } from './types'

const POLL_INTERVAL_MS = 4000

interface UeInstanceStatus {
  status: NodeStatus
  lastDeployed: string
}

// One row per UE that could run: every UE of every UE profile, every
// free5GC subscriber (when the fleet uses them), and any other instance
// still deployed. A UE never deployed shows as "stopped" with no logs yet.
export interface UeRow {
  // where its config comes from
  source: 'profile' | 'subscriber' | 'other'
  profileId?: string
  profileName?: string
  // a profile UE's deploy request (subscriber UEs fetch theirs on deploy)
  request?: DeployUeRequest
  ueId: string
  plmnId: string
  gpsi: string
  status: NodeStatus
  hasBeenDeployed: boolean
  lastDeployed: string
  // this subscriber's first S-NSSAI session's sd - only used to pick which
  // gNB slice's box the detailed topology draws this UE's Uu line from
  // under ulcl-2slice; '' until its detail fetch resolves.
  sd: string
}

export interface UeSources {
  profiles: UeProfile[]
  // include the free5GC subscribers in the fleet
  useSubscribers: boolean
}

export function useUeInstances({ profiles, useSubscribers }: UeSources) {
  const [instancesByUeId, setInstancesByUeId] = useState<Record<string, UeInstanceStatus>>({})
  const [isLoadingInstances, setIsLoadingInstances] = useState(true)
  const [subscribers, setSubscribers] = useState<Subscriber[]>([])
  const [isLoadingSubscribers, setIsLoadingSubscribers] = useState(true)
  const [subscribersError, setSubscribersError] = useState<string | null>(null)
  const [pendingInstances, setPendingInstances] = useState<Record<string, boolean>>({})
  const [sdByUeId, setSdByUeId] = useState<Record<string, string>>({})
  const fetchedSdRef = useRef<Set<string>>(new Set())

  const refreshInstances = useCallback(async () => {
    try {
      const response = await api.deployUeList()
      const map: Record<string, UeInstanceStatus> = {}
      for (const item of response.data.instances ?? []) {
        map[item.instance] = {
          status: item.status as NodeStatus,
          lastDeployed: item.lastDeployed ? new Date(item.lastDeployed).toLocaleString() : '—',
        }
      }
      setInstancesByUeId(map)
    } finally {
      setIsLoadingInstances(false)
    }
  }, [])

  const refreshSubscribers = useCallback(async () => {
    // isLoadingSubscribers only ever needs to gate the *first* load - once
    // it flips false it stays false, so background poll ticks update
    // `subscribers`/`subscribersError` silently instead of flashing
    // "Loading…" (or briefly clearing a real error) every 4s.
    try {
      const response = await webconsoleApi.getSubscribers()
      setSubscribers(response.data)
      setSubscribersError(null)
    } catch (error) {
      setSubscribersError(extractWebconsoleErrorMessage(
        error,
        'Failed to reach the free5GC webconsole. Is free5GC deployed?',
      ))
    } finally {
      setIsLoadingSubscribers(false)
    }
  }, [])

  useEffect(() => {
    // transient poll failures shouldn't blank out the last known state
    refreshInstances().catch(() => {})
    refreshSubscribers().catch(() => {})
    const timer = setInterval(() => {
      refreshInstances().catch(() => {})
      // webconsole isn't reachable until the core is deployed, so this must
      // keep retrying too - otherwise the UE panel never notices new
      // subscribers (or webconsole coming up) without a manual page refresh.
      refreshSubscribers().catch(() => {})
    }, POLL_INTERVAL_MS)
    return () => clearInterval(timer)
  }, [refreshInstances, refreshSubscribers])

  // fetches each subscriber's sd once (fetchedSdRef tracks who's already
  // been fetched, so this doesn't re-fetch every 4s poll tick) - only the
  // detailed topology view needs this, to know which gNB slice's box a UE's
  // Uu line should be drawn from under ulcl-2slice.
  useEffect(() => {
    const missing = subscribers.filter((subscriber) => !fetchedSdRef.current.has(subscriber.ueId))
    if (missing.length === 0) return
    for (const subscriber of missing) fetchedSdRef.current.add(subscriber.ueId)

    Promise.allSettled(missing.map(async (subscriber) => {
      const response = await webconsoleApi.getSubscriberByID(subscriber.ueId, subscriber.plmnID)
      return { ueId: subscriber.ueId, sd: fromSubscription(response.data).sessions[0]?.sd || '' }
    })).then((results) => {
      setSdByUeId((current) => {
        const next = { ...current }
        for (const result of results) {
          if (result.status === 'fulfilled') next[result.value.ueId] = result.value.sd
        }
        return next
      })
    })
  }, [subscribers])

  const rows: UeRow[] = useMemo(() => {
    const out: UeRow[] = []
    const seen = new Set<string>()
    const state = (ueId: string) => {
      const instance = instancesByUeId[ueId]
      return {
        status: instance?.status ?? 'stopped' as NodeStatus,
        hasBeenDeployed: Boolean(instance),
        lastDeployed: instance?.lastDeployed ?? '—',
      }
    }
    for (const p of profiles) {
      for (let k = 0; k < p.count; k++) {
        const msin = msinAt(p.msinStart, k)
        const ueId = `imsi-${p.mcc}${p.mnc}${msin}`
        if (seen.has(ueId)) continue
        seen.add(ueId)
        out.push({
          source: 'profile', profileId: p.id, profileName: p.name,
          request: { mcc: p.mcc, mnc: p.mnc, msin, permanentKey: p.permanentKey, opValue: p.opValue, amf: p.amf, sqn: p.sqn, dnn: p.dnn || 'internet', sst: p.sst, sd: p.sd ?? '' },
          ueId, plmnId: `${p.mcc}${p.mnc}`, gpsi: '', sd: p.sd ?? '', ...state(ueId),
        })
      }
    }
    if (useSubscribers) {
      for (const subscriber of subscribers) {
        if (seen.has(subscriber.ueId)) continue
        seen.add(subscriber.ueId)
        out.push({
          source: 'subscriber', ueId: subscriber.ueId, plmnId: subscriber.plmnID, gpsi: subscriber.gpsi,
          sd: sdByUeId[subscriber.ueId] ?? '', ...state(subscriber.ueId),
        })
      }
    }
    // deployed before, from a source not shown now: can still be stopped
    for (const ueId of Object.keys(instancesByUeId)) {
      if (seen.has(ueId)) continue
      out.push({ source: 'other', ueId, plmnId: '—', gpsi: '', sd: '', ...state(ueId) })
    }
    return out
  }, [profiles, useSubscribers, subscribers, instancesByUeId, sdByUeId])

  const withPending = useCallback(async (ueId: string, action: () => Promise<void>) => {
    setPendingInstances((current) => ({ ...current, [ueId]: true }))
    try {
      await action()
      await refreshInstances()
    } finally {
      setPendingInstances((current) => ({ ...current, [ueId]: false }))
    }
  }, [refreshInstances])

  const requestFor = useCallback(async (ueId: string): Promise<DeployUeRequest> => {
    const row = rows.find((r) => r.ueId === ueId)
    if (row?.request) return row.request
    const subscriber = subscribers.find((item) => item.ueId === ueId)
    if (!subscriber) throw new Error(`No config for ${ueId}: it is not in a UE profile or the free5GC subscribers`)
    const response = await webconsoleApi.getSubscriberByID(subscriber.ueId, subscriber.plmnID)
    return subscriptionToUeDeployRequest(response.data)
  }, [rows, subscribers])

  const deploy = useCallback(
    (ueId: string) => withPending(ueId, async () => {
      await api.deployUeUp(ueId, await requestFor(ueId))
    }),
    [withPending, requestFor],
  )

  // deployMany starts UEs one after another (each waits for its container)
  // and reports progress; it returns the ids that failed.
  const deployMany = useCallback(async (ueIds: string[], onProgress?: (done: number) => void): Promise<string[]> => {
    const failed: string[] = []
    let done = 0
    for (const ueId of ueIds) {
      setPendingInstances((current) => ({ ...current, [ueId]: true }))
      try {
        await api.deployUeUp(ueId, await requestFor(ueId))
      } catch {
        failed.push(ueId)
      } finally {
        setPendingInstances((current) => ({ ...current, [ueId]: false }))
        onProgress?.(++done)
        refreshInstances().catch(() => {})
      }
    }
    return failed
  }, [requestFor, refreshInstances])

  // stopMany stops the given UEs in parallel.
  const stopMany = useCallback(async (ueIds: string[]) => {
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
    (ueId: string) => withPending(ueId, async () => {
      await api.deployUeDown(ueId)
    }),
    [withPending],
  )

  const stopAll = useCallback(async () => {
    const runningIds = rows.filter((row) => row.status !== 'stopped').map((row) => row.ueId)
    if (runningIds.length === 0) return

    setPendingInstances((current) => {
      const next = { ...current }
      for (const ueId of runningIds) next[ueId] = true
      return next
    })
    try {
      const results = await Promise.allSettled(runningIds.map((ueId) => api.deployUeDown(ueId)))
      const failedCount = results.filter((result) => result.status === 'rejected').length
      if (failedCount > 0) {
        throw new Error(`Failed to stop ${failedCount} of ${runningIds.length} UE instance(s)`)
      }
    } finally {
      await refreshInstances()
      setPendingInstances((current) => {
        const next = { ...current }
        for (const ueId of runningIds) next[ueId] = false
        return next
      })
    }
  }, [rows, refreshInstances])

  const fetchLogs = useCallback(async (ueId: string): Promise<string[]> => {
    const response = await api.deployUeLogs(ueId)
    return response.data.lines ?? []
  }, [])

  return {
    rows,
    isLoadingSubscribers,
    isLoadingInstances,
    subscribersError,
    pendingInstances,
    deploy,
    deployMany,
    stop,
    stopMany,
    stopAll,
    fetchLogs,
  }
}
