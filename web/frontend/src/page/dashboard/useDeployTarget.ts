import { useCallback, useEffect, useState } from 'react'
import type { AxiosPromise } from 'axios'
import type { DeployStatusResponse, DeployLogsResponse, MessageResponse } from '../../api'
import type { DeploymentNode, NetworkFunction, NodeId, NodeStatus } from './types'

const POLL_INTERVAL_MS = 4000

interface DeployApi {
  // variant is ignored by targets that only ever have one template (gnb) -
  // only free5gc's `up` actually reads it.
  up: (variant?: string) => AxiosPromise<MessageResponse>
  down: () => AxiosPromise<MessageResponse>
  status: () => AxiosPromise<DeployStatusResponse>
  logs: () => AxiosPromise<DeployLogsResponse>
}

// Shared by every target that has a real deploy API (currently free5gc and
// gnb): polls status, and exposes deploy/stop/fetchLogs actions. Each
// target's hook is just this with its own id/label/api methods plugged in.
export function useDeployTarget(
  id: NodeId,
  label: string,
  formatSublabel: (serviceCount: number) => string,
  deployApi: DeployApi,
) {
  const [node, setNode] = useState<DeploymentNode>({
    id,
    label,
    sublabel: 'Checking status…',
    status: 'stopped',
    template: 'basic (built-in template)',
    lastDeployed: '—',
  })
  const [networkFunctions, setNetworkFunctions] = useState<NetworkFunction[]>([])
  const [isActionPending, setIsActionPending] = useState(false)
  const [logLines, setLogLines] = useState<string[]>([])
  const [isLoadingLogs, setIsLoadingLogs] = useState(false)

  const refresh = useCallback(async () => {
    const response = await deployApi.status()
    const services = response.data.services ?? []

    setNetworkFunctions(services.map((service) => ({
      name: service.name.toUpperCase(),
      status: service.status as NodeStatus,
    })))
    setNode({
      id,
      label,
      sublabel: formatSublabel(services.length),
      status: response.data.status as NodeStatus,
      template: 'basic (built-in template)',
      lastDeployed: response.data.lastDeployed
        ? new Date(response.data.lastDeployed).toLocaleString()
        : '—',
    })
  }, [deployApi, id, label, formatSublabel])

  useEffect(() => {
    refresh().catch(() => {
      // transient poll failures shouldn't blank out the last known state
    })
    const timer = setInterval(() => {
      refresh().catch(() => {})
    }, POLL_INTERVAL_MS)
    return () => clearInterval(timer)
  }, [refresh])

  const deploy = useCallback(async (variant?: string) => {
    setIsActionPending(true)
    try {
      await deployApi.up(variant)
      await refresh()
    } finally {
      setIsActionPending(false)
    }
  }, [deployApi, refresh])

  const stop = useCallback(async () => {
    setIsActionPending(true)
    try {
      await deployApi.down()
      await refresh()
    } finally {
      setIsActionPending(false)
    }
  }, [deployApi, refresh])

  const fetchLogs = useCallback(async () => {
    setIsLoadingLogs(true)
    try {
      const response = await deployApi.logs()
      setLogLines(response.data.lines ?? [])
    } finally {
      setIsLoadingLogs(false)
    }
  }, [deployApi])

  return { node, networkFunctions, isActionPending, deploy, stop, logLines, isLoadingLogs, fetchLogs }
}
