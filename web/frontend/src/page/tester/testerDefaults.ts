import type { TesterProfile } from '../../api'

// Starting point for a first-time setup page; values match the basic
// free5GC template fru-lab deploys (AMF 10.0.1.3, UPF 10.0.1.5 on the
// frulab-cn-ran bridge, host side docker-cn-ran) so it runs as-is there.
export const DEFAULT_TESTER_PROFILE: TesterProfile = {
  name: 'basic',
  scale: { gnbCount: 10, ueCount: 1000 },
  gnb: {
    gnbIdStart: '000314',
    namePattern: 'gNB-{i}',
    mcc: '208',
    mnc: '93',
    tac: '000001',
    sst: 1,
    sd: '010203',
  },
  network: {
    n2: { interface: 'docker-cn-ran', cidr: '10.0.1.0/24', startIp: '10.0.1.100', amfIp: '10.0.1.3', amfPort: 38412 },
    n3: { interface: 'docker-cn-ran', cidr: '10.0.1.0/24', startIp: '10.0.1.100', upfIp: '10.0.1.5', upfPort: 2152 },
  },
  rates: { n2: { timeoutMs: 5000, retries: 1 } },
}

// normalizeProfile turns whatever was saved into a complete profile:
// every field missing or of the wrong type takes its default, and keys
// this version doesn't know are dropped (fru-tester rejects unknown
// fields). Saved profiles can predate fields added in later phases.
export function normalizeProfile(saved: unknown): TesterProfile {
  return mergeKnown(DEFAULT_TESTER_PROFILE, saved) as TesterProfile
}

function mergeKnown(defaults: unknown, saved: unknown): unknown {
  if (typeof defaults === 'object' && defaults !== null) {
    const src = typeof saved === 'object' && saved !== null ? saved as Record<string, unknown> : {}
    return Object.fromEntries(Object.entries(defaults).map(([key, value]) => [key, mergeKnown(value, src[key])]))
  }
  return typeof saved === typeof defaults ? saved : defaults
}
