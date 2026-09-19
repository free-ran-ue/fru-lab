import { useCallback, useMemo } from 'react'
import { api } from '../../apiClient'
import { useDeployTarget } from './useDeployTarget'

export function useFree5gcStatus() {
  const deployApi = useMemo(() => ({
    up: () => api.deployFree5gcUp(),
    down: () => api.deployFree5gcDown(),
    status: () => api.deployFree5gcStatus(),
    logs: () => api.deployFree5gcLogs(),
  }), [])

  const formatSublabel = useCallback(
    (serviceCount: number) => `Core Network · ${serviceCount} network functions`,
    [],
  )

  return useDeployTarget('core', 'free5GC Core', formatSublabel, deployApi)
}
