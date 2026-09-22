import { useCallback, useMemo } from 'react'
import { api } from '../../apiClient'
import { useDeployTarget } from './useDeployTarget'

// gnb-slice1/gnb-slice2 are independent deploy targets (see
// backend/constant's DEPLOY_TARGET_GNB_SLICE1/2) - unlike the singleton
// gnb, both can be deployed at once, so each gets its own hook instance
// instead of sharing useGnbStatus. Neither is a selectable canvas node on
// its own though - the Dashboard's single "gNB" card fans deploy/stop out
// to both of these at once under the ulcl-2slice template (see
// DashboardPage's handlePrimaryAction).
export function useGnbSliceStatus(slice: 'slice1' | 'slice2') {
  const deployApi = useMemo(() => ({
    up: () => api.deployGnbSliceUp(slice),
    down: () => api.deployGnbSliceDown(slice),
    status: () => api.deployGnbSliceStatus(slice),
    logs: () => api.deployGnbSliceLogs(slice),
  }), [slice])

  const formatSublabel = useCallback(
    () => `free-ran-ue · ${slice === 'slice1' ? 'SD 010203' : 'SD 112233'}`,
    [slice],
  )

  const label = slice === 'slice1' ? 'gNB Slice 1' : 'gNB Slice 2'

  return useDeployTarget(`gnb-${slice}`, label, formatSublabel, deployApi)
}
