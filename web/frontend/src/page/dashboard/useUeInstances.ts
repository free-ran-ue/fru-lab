import { useCallback, useEffect, useMemo, useState } from 'react'
import { api } from '../../apiClient'
import { webconsoleApi, extractWebconsoleErrorMessage } from '../../webconsoleApiClient'
import type { Subscriber } from '../../webconsoleApi'
import { subscriptionToUeDeployRequest } from './ueDeployFromSubscription'
import type { NodeStatus } from './types'

const POLL_INTERVAL_MS = 4000

interface UeInstanceStatus {
  status: NodeStatus
  lastDeployed: string
}

// One row per subscriber (not per deployed instance) - a subscriber that has
// never been deployed still shows up, just as "stopped" with no logs yet,
// since the UE fleet is defined by who *could* be deployed, not by who
// happens to be materialized on disk right now.
export interface UeRow {
  ueId: string
  plmnId: string
  gpsi: string
  status: NodeStatus
  hasBeenDeployed: boolean
  lastDeployed: string
}

export function useUeInstances() {
  const [instancesByUeId, setInstancesByUeId] = useState<Record<string, UeInstanceStatus>>({})
  const [isLoadingInstances, setIsLoadingInstances] = useState(true)
  const [subscribers, setSubscribers] = useState<Subscriber[]>([])
  const [isLoadingSubscribers, setIsLoadingSubscribers] = useState(true)
  const [subscribersError, setSubscribersError] = useState<string | null>(null)
  const [pendingInstances, setPendingInstances] = useState<Record<string, boolean>>({})

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

  const rows: UeRow[] = useMemo(() => subscribers.map((subscriber) => {
    const instance = instancesByUeId[subscriber.ueId]
    return {
      ueId: subscriber.ueId,
      plmnId: subscriber.plmnID,
      gpsi: subscriber.gpsi,
      status: instance?.status ?? 'stopped',
      hasBeenDeployed: Boolean(instance),
      lastDeployed: instance?.lastDeployed ?? '—',
    }
  }), [subscribers, instancesByUeId])

  const withPending = useCallback(async (ueId: string, action: () => Promise<void>) => {
    setPendingInstances((current) => ({ ...current, [ueId]: true }))
    try {
      await action()
      await refreshInstances()
    } finally {
      setPendingInstances((current) => ({ ...current, [ueId]: false }))
    }
  }, [refreshInstances])

  const deploy = useCallback(
    (ueId: string) => withPending(ueId, async () => {
      const subscriber = subscribers.find((item) => item.ueId === ueId)
      if (!subscriber) throw new Error(`Unknown subscriber ${ueId}`)
      const response = await webconsoleApi.getSubscriberByID(subscriber.ueId, subscriber.plmnID)
      const request = subscriptionToUeDeployRequest(response.data)
      await api.deployUeUp(ueId, request)
    }),
    [withPending, subscribers],
  )

  const stop = useCallback(
    (ueId: string) => withPending(ueId, async () => {
      await api.deployUeDown(ueId)
    }),
    [withPending],
  )

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
    stop,
    fetchLogs,
  }
}
