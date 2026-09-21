import { useCallback, useMemo } from 'react'
import { api } from '../../apiClient'
import { useDeployTarget } from './useDeployTarget'
import type { RequestDeployFree5gcTemplateEnum } from '../../api'

export function useFree5gcStatus() {
  const deployApi = useMemo(() => ({
    up: (template?: string) => api.deployFree5gcUp(template ? { template: template as RequestDeployFree5gcTemplateEnum } : undefined),
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
