export type NodeId = 'core' | 'gnb' | 'ue'
export type NodeStatus = 'running' | 'stopped' | 'unhealthy' | 'deploying'

export interface NetworkFunction {
  name: string
  status: NodeStatus
}

export interface DeploymentNode {
  id: NodeId
  label: string
  sublabel: string
  status: NodeStatus
  template: string
  lastDeployed: string
  // overrides the status pill's text (e.g. "2 / 3" for a multi-instance
  // target like ue); falls back to the generic status word when unset.
  statusLabel?: string
}
