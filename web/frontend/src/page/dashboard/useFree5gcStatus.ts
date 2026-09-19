import { useCallback, useEffect, useState } from 'react'
import { api } from '../../apiClient'
import type { DeploymentNode, NetworkFunction, NodeStatus } from './types'

const POLL_INTERVAL_MS = 4000

const INITIAL_NODE: DeploymentNode = {
  id: 'core',
  label: 'free5GC Core',
  sublabel: 'Core Network · checking status…',
  status: 'stopped',
  template: 'basic (built-in template)',
  lastDeployed: '—',
}

export function useFree5gcStatus() {
  const [node, setNode] = useState<DeploymentNode>(INITIAL_NODE)
  const [networkFunctions, setNetworkFunctions] = useState<NetworkFunction[]>([])
  const [isActionPending, setIsActionPending] = useState(false)
  const [logLines, setLogLines] = useState<string[]>([])
  const [isLoadingLogs, setIsLoadingLogs] = useState(false)

  const refresh = useCallback(async () => {
    const response = await api.deployFree5gcStatus()
    const services = response.data.services ?? []

    setNetworkFunctions(services.map((service) => ({
      name: service.name.toUpperCase(),
      status: service.status as NodeStatus,
    })))
    setNode({
      id: 'core',
      label: 'free5GC Core',
      sublabel: `Core Network · ${services.length} network functions`,
      status: response.data.status as NodeStatus,
      template: 'basic (built-in template)',
      lastDeployed: response.data.lastDeployed
        ? new Date(response.data.lastDeployed).toLocaleString()
        : '—',
    })
  }, [])

  useEffect(() => {
    refresh().catch(() => {
      // transient poll failures shouldn't blank out the last known state
    })
    const timer = setInterval(() => {
      refresh().catch(() => {})
    }, POLL_INTERVAL_MS)
    return () => clearInterval(timer)
  }, [refresh])

  const deploy = useCallback(async () => {
    setIsActionPending(true)
    try {
      await api.deployFree5gcUp()
      await refresh()
    } finally {
      setIsActionPending(false)
    }
  }, [refresh])

  const stop = useCallback(async () => {
    setIsActionPending(true)
    try {
      await api.deployFree5gcDown()
      await refresh()
    } finally {
      setIsActionPending(false)
    }
  }, [refresh])

  const fetchLogs = useCallback(async () => {
    setIsLoadingLogs(true)
    try {
      const response = await api.deployFree5gcLogs()
      setLogLines(response.data.lines)
    } finally {
      setIsLoadingLogs(false)
    }
  }, [])

  return { node, networkFunctions, isActionPending, deploy, stop, logLines, isLoadingLogs, fetchLogs }
}
