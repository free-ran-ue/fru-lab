import type { TesterProfile } from '../../api'

// Starting point for a first-time setup page; values mirror free-ran-ue's
// sample gnb.yaml so a lab already running fru-lab's free5GC gets close.
export const DEFAULT_TESTER_PROFILE: TesterProfile = {
  name: 'N2 baseline',
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
    n2: { interface: '', cidr: '10.0.1.0/24', startIp: '10.0.1.100', amfIp: '10.0.1.1', amfPort: 38412 },
    n3: { interface: '', cidr: '10.0.2.0/24', startIp: '10.0.2.100', upfIp: '10.0.2.1', upfPort: 2152 },
  },
  rates: { n2: { timeoutMs: 5000, retries: 1 } },
}
