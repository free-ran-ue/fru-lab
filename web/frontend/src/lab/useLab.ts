import type { CoreNetwork, GnbDeployment } from '../api'
import { useFree5gcStatus } from './useFree5gcStatus'
import { useCoreDeploys } from './useCoreDeploys'
import { LAB_CORE, useCoreNetworks } from './useCoreNetworks'
import { coreAddresses, useIpam } from './useIpam'
import { useGnbInstances } from './useGnbInstances'
import { useUeInstances, type UeRow } from './useUeInstances'
import { useGnbProfiles, useUeProfiles } from './useProfiles'
import { FREE5GC_TEMPLATE_OPTIONS, detectFree5gcTemplate, type Free5gcTemplate } from './free5gcTemplate'
import type { DeploymentNode, NetworkFunction, NodeStatus } from './types'

// LabGnb is one gNB that is up: a gNB profile's gNB. Its id is the gNB
// target name, gnb-<profileId>, which is also its canvas node id, its log
// stream target and the `gnb` a UE's status records.
export interface LabGnb {
  id: string
  profileId: string
  name: string
  profileName: string
  status: NodeStatus
  networkFunctions: NetworkFunction[]
  lastDeployed: string
  // what it was deployed with (profile name, core, IDs and addresses)
  deployment: GnbDeployment | null
  core: 'lab' | 'external'
  // the core network it attached to (deployment.coreNetwork); '' for the
  // lab free5GC (or a gNB deployed before core networks recorded it)
  coreNetwork: string
  sd: string
  pending: boolean
  stop: () => Promise<void>
}

const STATUS_RANK: NodeStatus[] = ['unhealthy', 'deploying', 'running', 'stopped']
// the worst of several statuses; stopped when there are none
function worst(statuses: NodeStatus[]): NodeStatus {
  return STATUS_RANK.find((s) => statuses.includes(s)) ?? 'stopped'
}

const fmtTime = (t?: string) => (t ? new Date(t).toLocaleString() : '—')

// isFree5gcCore tells a free5GC core network fru-lab deploys on this host
// (kind free5gc) from a core outside fru-lab (kind external, the default).
export const isFree5gcCore = (n?: CoreNetwork) => n?.kind === 'free5gc'

// onLabCore: the gNB is attached to the lab free5GC (not to a free5GC core
// network, whose gNBs also have core lab)
export const onLabCore = (g: LabGnb) => g.core === 'lab' && !g.coreNetwork

// LabCore is one free5GC core network (kind free5gc) with how it runs.
export interface LabCore {
  id: string
  name: string
  network: CoreNetwork
  status: NodeStatus
  template: Free5gcTemplate | null
  networkFunctions: NetworkFunction[]
  healthyNfs: number
  lastDeployed: string
  pending: boolean
  // the running gNBs attached to it
  gnbs: LabGnb[]
  // why it cannot be stopped now, if it cannot
  stopBlocked?: string
  deploy: (template: Free5gcTemplate) => Promise<void>
  stop: () => Promise<void>
}

// The UE count is driven by the UE profiles (one UE each), not by who
// happens to be deployed right now - a profile never deployed still counts
// toward the total, just as "stopped".
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
    sublabel: total === 0 ? 'No UE profiles yet' : `${runningCount} / ${total} UE instance${total === 1 ? '' : 's'} running`,
    status,
    statusLabel: `${runningCount} / ${total}`,
    template: 'basic (built-in template)',
    lastDeployed: mostRecentDeploy ?? '—',
  }
}

export function templateLabel(template: Free5gcTemplate): string {
  return FREE5GC_TEMPLATE_OPTIONS.find((option) => option.value === template)?.label ?? 'Basic'
}

// useLab polls every deploy target (core, the profile gNBs, the UE fleet) and derives what the Dashboard and the Core Network /
// free-ran / free-ue pages all need: the running gNBs, aggregate nodes, the
// running template, and the dependency rules the backend enforces
// (core → gNB → UE).
function useLabState() {
  const free5gc = useFree5gcStatus()
  const gnbInstances = useGnbInstances()
  const gnbProfiles = useGnbProfiles()
  const coreNetworks = useCoreNetworks()
  const ipam = useIpam()
  // the lab free5GC with its addresses from the address plan
  const labCore = { ...LAB_CORE, ...coreAddresses(ipam.pools) }
  const coreDeploys = useCoreDeploys()
  const ueProfiles = useUeProfiles()
  const ue = useUeInstances({ profiles: ueProfiles.profiles })
  const coreRunning = free5gc.node.status === 'running'

  // the template actually running, read back from the live NFs; null while
  // the core is stopped
  const runningTemplate = detectFree5gcTemplate(free5gc.networkFunctions)

  // every gNB that is up, in profile order
  const { stop: stopGnb, pending: gnbPending } = gnbInstances
  const profileOrder = new Map(gnbProfiles.profiles.map((p, i) => [p.id, i]))
  const gnbs: LabGnb[] = [
    ...[...gnbInstances.instances]
      .filter((g) => g.status !== 'stopped')
      .sort((a, b) => (profileOrder.get(a.profileId) ?? 1e9) - (profileOrder.get(b.profileId) ?? 1e9))
      .map((g): LabGnb => ({
        id: g.instance,
        profileId: g.profileId,
        name: g.instance,
        profileName: g.gnb?.profileName || gnbProfiles.profiles.find((p) => p.id === g.profileId)?.name || g.profileId,
        status: g.status as NodeStatus,
        networkFunctions: (g.services ?? []).map((s) => ({ name: s.name.toUpperCase(), status: s.status as NodeStatus })),
        lastDeployed: fmtTime(g.lastDeployed),
        deployment: g.gnb ?? null,
        core: g.gnb?.core === 'external' ? 'external' : 'lab',
        coreNetwork: g.gnb?.coreNetwork ?? '',
        sd: g.gnb?.sd ?? '',
        pending: Boolean(gnbPending[g.profileId]),
        stop: () => stopGnb(g.profileId),
      })),
  ]

  const gnbNode: DeploymentNode = {
    id: 'gnb',
    label: 'gNB',
    sublabel: gnbs.length ? `${gnbs.length} gNB${gnbs.length === 1 ? '' : 's'} running` : 'free-ran-ue · gNB simulator',
    status: worst(gnbs.map((g) => g.status)),
    statusLabel: gnbs.length ? String(gnbs.length) : undefined,
    template: 'gNB profiles',
    lastDeployed: gnbs[0]?.lastDeployed ?? '—',
  }
  const ueNode = computeUeNode(ue.rows)

  const coreUp = free5gc.node.status !== 'stopped'
  const gnbUp = gnbs.length > 0
  // the running gNBs that dial a core outside the lab
  const externalGnbs = gnbs.filter((g) => g.core === 'external')
  const gnbExternal = externalGnbs.length > 0
  // gNBs that depend on the lab core
  const labGnbs = gnbs.filter(onLabCore)
  // the running gNBs on a core: the lab free5GC's without one, else those
  // recorded on that core network (an external one's from before that
  // record: those dialing its AMF)
  const gnbsOnCore = (n?: CoreNetwork): LabGnb[] => (!n
    ? labGnbs
    : gnbs.filter((g) => (g.coreNetwork
      ? g.coreNetwork === n.id
      : !isFree5gcCore(n) && g.core === 'external' && Boolean(n.n2Ip) && g.deployment?.amfIp === n.n2Ip)))

  // every free5GC core network with how it runs, in name order
  const { deploy: deployCore, stop: stopCore, get: coreDeploy, pending: corePending } = coreDeploys
  const free5gcCores: LabCore[] = coreNetworks.networks.filter((n) => isFree5gcCore(n) && n.id).map((n) => {
    const id = n.id!
    const d = coreDeploy(id)
    const on = gnbsOnCore(n)
    return {
      id,
      name: n.name,
      network: n,
      status: d.status,
      template: d.template,
      networkFunctions: d.networkFunctions,
      healthyNfs: d.networkFunctions.filter((nf) => nf.status === 'running').length,
      lastDeployed: d.lastDeployed,
      pending: Boolean(corePending[id]),
      gnbs: on,
      stopBlocked: on.length ? `Stop the ${on.length} gNB${on.length === 1 ? '' : 's'} on ${n.name} (free-ran) before stopping it` : undefined,
      deploy: (t: Free5gcTemplate) => deployCore(id, t),
      stop: () => stopCore(id),
    }
  })
  // the free5GC core networks that are up (deploying, running or unhealthy)
  const upCores = free5gcCores.filter((c) => c.status !== 'stopped')
  const loaded = free5gc.loaded && gnbInstances.loaded && !ue.isLoadingInstances && coreDeploys.loaded
  const upUes = ue.rows.filter((row) => row.status !== 'stopped')
  const ueUp = upUes.length > 0

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
    coreStop: labGnbs.length
      ? `Stop the ${labGnbs.length} gNB${labGnbs.length === 1 ? '' : 's'} on the lab core (free-ran) before stopping the core network`
      : undefined,
    // for a gNB on the lab core; one on an external core needs nothing
    gnbDeploy: coreRunning ? undefined : 'Deploy the core network first, or pick a gNB profile on an external core',
    ueDeploy: gnbUp ? undefined : 'Deploy a gNB on the free-ran page before deploying a UE instance',
  }
  // why stopping gNB `id` is not possible now: while a UE recorded on it runs
  const gnbStopBlocked = (id: string): string | undefined => {
    const on = upUes.filter((r) => r.gnb === id)
    if (on.length === 0) return undefined
    return `Stop the ${on.length} UE${on.length === 1 ? '' : 's'} on this gNB first (free-ue)`
  }

  return {
    free5gc, gnbInstances, ue, coreDeploys,
    gnbs, externalGnbs, labGnbs, gnbsOnCore, free5gcCores, upCores,
    runningTemplate,
    nodes: { core: free5gc.node, gnb: gnbNode, ue: ueNode },
    coreUp, gnbUp, ueUp, upUes, gnbExternal, loaded,
    gnbProfiles, ueProfiles, coreRunning, coreNetworks, ipam, labCore,
    healthyNfs, ueCounts, blocked, gnbStopBlocked,
  }
}

export type Lab = ReturnType<typeof useLabState>

export { useLabState }
