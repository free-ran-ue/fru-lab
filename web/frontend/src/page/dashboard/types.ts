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
}
