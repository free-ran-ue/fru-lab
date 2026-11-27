import { useFree5gcStatus } from './useFree5gcStatus'
import { useGnbStatus } from './useGnbStatus'
import { useGnbSliceStatus } from './useGnbSliceStatus'
import { useUeInstances, type UeRow } from './useUeInstances'
import { FREE5GC_TEMPLATE_OPTIONS, detectFree5gcTemplate, type Free5gcTemplate } from './free5gcTemplate'
import type { GnbSlice } from './TopologySchematic'
import type { DeploymentNode, NetworkFunction, NodeStatus } from './types'

// mirrors backend's constant.SLICE1_SD / SLICE2_SD - which gNB slice a
// UE attaches to under ulcl-2slice, from its subscriber's first session.
export const SLICE1_SD = '010203'
export const SLICE2_SD = '112233'

// UE's count is driven by subscribers, not by who happens to be deployed
// right now - a subscriber that's never been deployed still counts toward
// the total, just as "stopped".
function computeUeNode(rows: UeRow[]): DeploymentNode {
  const total = rows.length
  const runningCount = rows.filter((row) => row.status === 'running').length
  const status: NodeStatus = rows.some((row) => row.status === 'unhealthy')
    ? 'unhealthy'
    : rows.some((row) => row.status === 'deploying')
      ? 'deploying'
      : runningCount > 0 ? 'running' : 'stopped'
  const mostRecentDeploy = rows.map((row) => row.lastDeployed).find((d) => d !== '—')
  return {
    id: 'ue',
    label: 'UE',
    sublabel: total === 0 ? 'No subscribers yet' : `${runningCount} / ${total} UE instance${total === 1 ? '' : 's'} running`,
    status,
    statusLabel: `${runningCount} / ${total}`,
    template: 'basic (built-in template)',
    lastDeployed: mostRecentDeploy ?? '—',
  }
}

// Under ulcl-2slice the one gNB stands for both slice gNBs: this folds
// their two statuses into one.
function computeGnbAggregateNode(slice1: DeploymentNode, slice2: DeploymentNode): DeploymentNode {
  const statuses = [slice1.status, slice2.status]
  const status: NodeStatus = statuses.includes('unhealthy')
    ? 'unhealthy'
    : statuses.includes('deploying')
      ? 'deploying'
      : statuses.includes('running') ? 'running' : 'stopped'
  return {
    id: 'gnb',
    label: 'gNB',
    sublabel: 'free-ran-ue · slice1 + slice2',
    status,
    template: 'ulcl-2slice (two slices)',
    lastDeployed: slice1.lastDeployed !== '—' ? slice1.lastDeployed : slice2.lastDeployed,
  }
}

export function templateLabel(template: Free5gcTemplate): string {
  return FREE5GC_TEMPLATE_OPTIONS.find((option) => option.value === template)?.label ?? 'Basic'
}

// useLab polls every deploy target (core, gNB and its two slices, the UE
// fleet) and derives what the Dashboard and the Core Network / free-ran /
// free-ue pages all need: aggregate nodes, the running template, and the
// dependency rules the backend enforces (core → gNB → UE).
export function useLab() {
  const free5gc = useFree5gcStatus()
  const gnb = useGnbStatus()
  const gnbSlice1 = useGnbSliceStatus('slice1')
  const gnbSlice2 = useGnbSliceStatus('slice2')
  const ue = useUeInstances()

  // the template actually running, read back from the live NFs; null while
  // the core is stopped
  const runningTemplate = detectFree5gcTemplate(free5gc.networkFunctions)
  const isTwoSlice = runningTemplate === 'ulcl-2slice'

  const gnbNode = isTwoSlice ? computeGnbAggregateNode(gnbSlice1.node, gnbSlice2.node) : gnb.node
  const ueNode = computeUeNode(ue.rows)

  // when the gNB stands for both slices, list both slices' own services
  const gnbNetworkFunctions: NetworkFunction[] = isTwoSlice
    ? [
      ...gnbSlice1.networkFunctions.map((nf) => ({ ...nf, name: `SLICE1-${nf.name}` })),
      ...gnbSlice2.networkFunctions.map((nf) => ({ ...nf, name: `SLICE2-${nf.name}` })),
    ]
    : gnb.networkFunctions

  const coreUp = free5gc.node.status !== 'stopped'
  // any gNB target up, including one left over from another template
  const gnbUp = gnb.node.status !== 'stopped' || gnbSlice1.node.status !== 'stopped' || gnbSlice2.node.status !== 'stopped'
  const upUes = ue.rows.filter((row) => row.status !== 'stopped')
  const ueUp = upUes.length > 0

  const gnbSlices: GnbSlice[] | null = isTwoSlice
    ? [
      { name: 'slice1', status: gnbSlice1.node.status, ueCount: upUes.filter((row) => row.sd === SLICE1_SD).length },
      { name: 'slice2', status: gnbSlice2.node.status, ueCount: upUes.filter((row) => row.sd === SLICE2_SD).length },
    ]
    : null

  const healthyNfs = free5gc.networkFunctions.filter((nf) => nf.status === 'running').length
  const ueCounts = {
    total: ue.rows.length,
    running: ue.rows.filter((row) => row.status === 'running').length,
    starting: ue.rows.filter((row) => row.status === 'deploying').length,
    unhealthy: ue.rows.filter((row) => row.status === 'unhealthy').length,
  }

  // the same rules as the backend (Processor.checkStopDependents /
  // checkDeployPrerequisite), so buttons are disabled with a reason
  // instead of only failing after a click
  const blocked = {
    coreStop: gnbUp ? 'Stop gNB on the free-ran page before stopping the core network' : undefined,
    gnbDeploy: free5gc.node.status !== 'running' ? 'Deploy the core network before deploying gNB' : undefined,
    gnbStop: ueUp ? 'Stop all UE instances on the free-ue page before stopping gNB' : undefined,
    ueDeploy: gnbNode.status === 'stopped' ? 'Deploy gNB on the free-ran page before deploying a UE instance' : undefined,
  }

  return {
    free5gc, gnb, gnbSlice1, gnbSlice2, ue,
    runningTemplate, isTwoSlice,
    nodes: { core: free5gc.node, gnb: gnbNode, ue: ueNode },
    gnbNetworkFunctions, gnbSlices,
    coreUp, gnbUp, ueUp, upUes,
    healthyNfs, ueCounts, blocked,
  }
}

export type Lab = ReturnType<typeof useLab>
