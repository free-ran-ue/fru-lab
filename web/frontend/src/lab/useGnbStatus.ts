import { useCallback, useMemo } from 'react'
import { api } from '../apiClient'
import { useDeployTarget } from './useDeployTarget'

export function useGnbStatus() {
  const deployApi = useMemo(() => ({
    // variant is the gNB profile id; empty deploys the lab default
    up: (profileId?: string) => api.deployGnbUp(profileId ? { profileId } : undefined),
    down: () => api.deployGnbDown(),
    status: () => api.deployGnbStatus(),
    logs: () => api.deployGnbLogs(),
  }), [])

  const formatSublabel = useCallback(() => 'free-ran-ue · gNB simulator', [])

  return useDeployTarget('gnb', 'gNB', formatSublabel, deployApi)
}
