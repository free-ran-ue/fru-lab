import type { DeploymentNode, NodeId } from './types'

// gNB and UE don't have a backend deploy target yet, so they stay mock data
// until free-ran-ue gets its own composeContext (see the design doc).
export const MOCK_NODES: Record<Exclude<NodeId, 'core'>, DeploymentNode> = {
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

// core's logs come from the real API now; this only backs gNB/UE until
// free-ran-ue has a deploy target too.
export function getMockLogLines(_nodeId: Exclude<NodeId, 'core'>): string[] {
  return [
    '$ docker compose -f fru-compose/free-ran-ue/docker-compose.yaml logs -f',
    'gnb   | [INFO] gNB simulator connecting to AMF via N2',
    'gnb   | [INFO] NGSetupRequest sent',
    'ue    | [INFO] waiting for gNB broadcast',
    'gnb   | [INFO] cell broadcast started, PCI 1',
    'ue    | [INFO] RRC connection established',
    '...',
  ]
}
