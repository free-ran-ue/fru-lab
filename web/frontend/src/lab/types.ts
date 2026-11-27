export type NodeId = 'core' | 'gnb' | 'ue'
export type NodeStatus = 'running' | 'stopped' | 'unhealthy' | 'deploying'

export interface NetworkFunction {
  name: string
  status: NodeStatus
}

export interface DeploymentNode {
  // not constrained to NodeId - some deploy targets (gnb-slice1/gnb-slice2)
  // have their own hook/status but aren't a selectable canvas node in their
  // own right, since the gNB card represents both at once.
  id: string
  label: string
  sublabel: string
  status: NodeStatus
  template: string
  lastDeployed: string
  // overrides the status pill's text (e.g. "2 / 3" for a multi-instance
  // target like ue); falls back to the generic status word when unset.
  statusLabel?: string
}
