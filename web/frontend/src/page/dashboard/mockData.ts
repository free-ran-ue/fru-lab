import type { DeploymentNode, NetworkFunction, NodeId } from './types'

export const MOCK_NODES: Record<NodeId, DeploymentNode> = {
  core: {
    id: 'core',
    label: 'free5GC Core',
    sublabel: 'Core Network · 9 network functions',
    status: 'running',
    template: 'basic (built-in template)',
    lastDeployed: '10 minutes ago',
  },
  gnb: {
    id: 'gnb',
    label: 'gNB',
    sublabel: 'free-ran-ue · gNB simulator',
    status: 'stopped',
    template: 'basic (built-in template)',
    lastDeployed: '—',
  },
  ue: {
    id: 'ue',
    label: 'UE',
    sublabel: 'free-ran-ue · UE simulator',
    status: 'stopped',
    template: 'basic (built-in template)',
    lastDeployed: '—',
  },
}

export const MOCK_NETWORK_FUNCTIONS: NetworkFunction[] = [
  { name: 'NRF', status: 'running' },
  { name: 'AMF', status: 'running' },
  { name: 'SMF', status: 'running' },
  { name: 'UPF', status: 'unhealthy' },
  { name: 'AUSF', status: 'running' },
  { name: 'UDM', status: 'running' },
  { name: 'UDR', status: 'running' },
  { name: 'PCF', status: 'running' },
  { name: 'NSSF', status: 'running' },
]

export function getMockLogLines(nodeId: NodeId): string[] {
  const composeDir = nodeId === 'core' ? 'free5gc' : 'free-ran-ue'
  const header = `$ docker compose -f fru-compose/${composeDir}/docker-compose.yaml logs -f`

  if (nodeId === 'core') {
    return [
      header,
      'amf   | [INFO] AMF started, PLMN 208/93, listening on N2',
      'smf   | [INFO] SMF registered to NRF',
      'upf   | [WARN] health check failed: N4 association lost',
      'nrf   | [INFO] AMF profile registered',
      'ausf  | [INFO] AUSF ready',
      '...',
    ]
  }

  return [
    header,
    'gnb   | [INFO] gNB simulator connecting to AMF via N2',
    'gnb   | [INFO] NGSetupRequest sent',
    'ue    | [INFO] waiting for gNB broadcast',
    'gnb   | [INFO] cell broadcast started, PCI 1',
    'ue    | [INFO] RRC connection established',
    '...',
  ]
}
