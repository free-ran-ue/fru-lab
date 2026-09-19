import type {
  ChargingData,
  Dnn,
  DnnConfiguration,
  FlowRules,
  QosFlows,
  SessionManagementSubscriptionData,
  SmPolicySnssai,
  SubscribedSnssaiInfo,
  Subscription,
} from '../../webconsoleApi'

let nextRowId = 0
function makeRowId(): string {
  nextRowId += 1
  return `row-${nextRowId}`
}

// A "Flow Rule" bundle mirrors free5GC webconsole's own create-subscriber form,
// which merges FlowRule + QosFlow + ChargingData into one nested block per
// session (they all share the same snssai/dnn/qosRef, derived automatically
// from the parent session rather than typed by hand).
export interface FlowRuleBundleRow {
  id: string
  filter: string
  precedence: string
  fiveQi: string
  uplinkGbr: string
  downlinkGbr: string
  uplinkMbr: string
  downlinkMbr: string
  chargingMethod: string
  quota: string
  unitCost: string
}

export interface SessionFormRow {
  id: string
  sst: string
  sd: string
  isDefaultSlice: boolean
  dnn: string
  pduSessionType: string
  sscMode: string
  fiveQi: string
  arpPriority: string
  ambrUplink: string
  ambrDownlink: string
  upIntegr: string
  upConfid: string
  showUpSecurity: boolean
  flowRules: FlowRuleBundleRow[]
}

export interface SubscriberFormState {
  ueId: string
  plmnMcc: string
  plmnMnc: string
  gpsi: string
  permanentKey: string
  opValue: string
  sequenceNumber: string
  amf: string
  ambrUplink: string
  ambrDownlink: string
  sessions: SessionFormRow[]
}

export const PDU_SESSION_TYPES = ['IPV4', 'IPV6', 'IPV4V6']
export const SSC_MODES = ['SSC_MODE_1', 'SSC_MODE_2', 'SSC_MODE_3']
export const UP_SECURITY_OPTIONS = ['', 'REQUIRED', 'PREFERRED', 'NOT_NEEDED']
export const CHARGING_METHODS = ['', 'Offline', 'Online']

// Mirrors free5GC webconsole's own defaultFlowRule() (lib/dtos/subscription.ts),
// used both as the seed for a brand new session and for the "+ Flow Rule" button.
export function makeFlowRuleBundle(overrides: Partial<FlowRuleBundleRow> = {}): FlowRuleBundleRow {
  return {
    id: makeRowId(),
    filter: '1.1.1.1/32',
    precedence: '128',
    fiveQi: '9',
    uplinkGbr: '208 Mbps',
    downlinkGbr: '208 Mbps',
    uplinkMbr: '108 Mbps',
    downlinkMbr: '108 Mbps',
    chargingMethod: 'Online',
    quota: '10000',
    unitCost: '1',
    ...overrides,
  }
}

// Mirrors free5GC webconsole's defaultSnssaiConfiguration() + defaultDnnConfig(),
// used for the "+ S-NSSAI" button (a non-default slice with a blank SD).
export function makeSessionRow(overrides: Partial<SessionFormRow> = {}): SessionFormRow {
  return {
    id: makeRowId(),
    sst: '1',
    sd: '',
    isDefaultSlice: false,
    dnn: 'internet',
    pduSessionType: 'IPV4',
    sscMode: 'SSC_MODE_1',
    fiveQi: '9',
    arpPriority: '8',
    ambrUplink: '1000 Mbps',
    ambrDownlink: '1000 Mbps',
    upIntegr: '',
    upConfid: '',
    showUpSecurity: false,
    flowRules: [makeFlowRuleBundle()],
    ...overrides,
  }
}

// Mirrors free5GC webconsole's defaultUpSecurity().
export function makeDefaultUpSecurity(): { upIntegr: string; upConfid: string } {
  return { upIntegr: 'NOT_NEEDED', upConfid: 'NOT_NEEDED' }
}

// Mirrors free5GC webconsole's defaultSubscriptionDTO() exactly, including its
// one pre-filled default slice ("010203") with its own default flow rule.
export function emptySubscriberForm(): SubscriberFormState {
  return {
    ueId: 'imsi-208930000000001',
    plmnMcc: '208',
    plmnMnc: '93',
    gpsi: '',
    permanentKey: '8baf473f2f8fd09487cccbd7097c6862',
    opValue: '8e27b6af0e692e750f32667a3b14605d',
    sequenceNumber: '000000000023',
    amf: '8000',
    ambrUplink: '1 Gbps',
    ambrDownlink: '2 Gbps',
    sessions: [
      makeSessionRow({
        sd: '010203',
        isDefaultSlice: true,
        ambrUplink: '1000 Mbps',
        ambrDownlink: '1000 Mbps',
        flowRules: [makeFlowRuleBundle({
          precedence: '128',
          fiveQi: '8',
          uplinkGbr: '108 Mbps',
          downlinkGbr: '108 Mbps',
          uplinkMbr: '208 Mbps',
          downlinkMbr: '208 Mbps',
          chargingMethod: 'Offline',
          quota: '100000',
          unitCost: '1',
        })],
      }),
    ],
  }
}

// Matches free5gc's own key convention for maps keyed by SNSSAI, e.g. "01010203"
// (2-digit hex SST + SD), used by SmfSelectionSubscriptionData/SmPolicyData.
export function snssaiKey(sst: string, sd: string): string {
  const sstNumber = Number(sst) || 0
  return sstNumber.toString(16).padStart(2, '0') + (sd || '')
}

function toInt(value: string): number {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : 0
}

function hasAnyQos(bundle: FlowRuleBundleRow): boolean {
  return Boolean(bundle.fiveQi || bundle.uplinkGbr || bundle.downlinkGbr || bundle.uplinkMbr || bundle.downlinkMbr)
}

function hasAnyCharging(bundle: FlowRuleBundleRow): boolean {
  return Boolean(bundle.chargingMethod || bundle.quota || bundle.unitCost)
}

export function toSubscription(form: SubscriberFormState): Subscription {
  const plmnID = `${form.plmnMcc.trim()}${form.plmnMnc.trim()}`
  const ueId = form.ueId.trim()

  interface SliceAccumulator {
    sst: string
    sd: string
    isDefaultSlice: boolean
    dnnConfigurations: Record<string, DnnConfiguration>
  }
  const sliceMap = new Map<string, SliceAccumulator>()

  const flowRules: FlowRules[] = []
  const qosFlows: QosFlows[] = []
  const chargingDatas: ChargingData[] = []

  for (const row of form.sessions) {
    const sliceMapKey = `${row.sst}-${row.sd}`
    let slice = sliceMap.get(sliceMapKey)
    if (!slice) {
      slice = { sst: row.sst, sd: row.sd, isDefaultSlice: row.isDefaultSlice, dnnConfigurations: {} }
      sliceMap.set(sliceMapKey, slice)
    }

    const dnnConfiguration: DnnConfiguration = {
      pduSessionTypes: { defaultSessionType: row.pduSessionType, allowedSessionTypes: [row.pduSessionType] },
      sscModes: { defaultSscMode: row.sscMode, allowedSscModes: [row.sscMode] },
      '5gQosProfile': {
        '5qi': toInt(row.fiveQi),
        arp: {
          priorityLevel: toInt(row.arpPriority),
          preemptCap: '',
          preemptVuln: '',
        },
        priorityLevel: toInt(row.arpPriority),
      },
      sessionAmbr: { uplink: row.ambrUplink, downlink: row.ambrDownlink },
    }
    if (row.upIntegr || row.upConfid) {
      dnnConfiguration.upSecurity = {
        upIntegr: row.upIntegr || 'NOT_NEEDED',
        upConfid: row.upConfid || 'NOT_NEEDED',
      }
    }
    slice.dnnConfigurations[row.dnn] = dnnConfiguration

    const snssai = snssaiKey(row.sst, row.sd)
    row.flowRules.forEach((bundle, index) => {
      const qosRef = index + 1
      flowRules.push({
        filter: bundle.filter,
        precedence: toInt(bundle.precedence),
        snssai,
        dnn: row.dnn,
        qosRef,
      })
      if (hasAnyQos(bundle)) {
        qosFlows.push({
          snssai,
          dnn: row.dnn,
          qosRef,
          '5qi': toInt(bundle.fiveQi),
          mbrUL: bundle.uplinkMbr,
          mbrDL: bundle.downlinkMbr,
          gbrUL: bundle.uplinkGbr,
          gbrDL: bundle.downlinkGbr,
        })
      }
      if (hasAnyCharging(bundle)) {
        chargingDatas.push({
          snssai,
          dnn: row.dnn,
          qosRef,
          filter: bundle.filter,
          chargingMethod: bundle.chargingMethod,
          quota: bundle.quota,
          unitCost: bundle.unitCost,
        })
      }
    })
  }

  const slices = Array.from(sliceMap.values())

  const sessionManagementSubscriptionData: SessionManagementSubscriptionData[] = slices.map((slice) => ({
    singleNssai: { sst: toInt(slice.sst), sd: slice.sd },
    dnnConfigurations: slice.dnnConfigurations,
  }))

  const subscribedSnssaiInfos: Record<string, SubscribedSnssaiInfo> = {}
  const smPolicySnssaiData: Record<string, SmPolicySnssai> = {}
  for (const slice of slices) {
    const key = snssaiKey(slice.sst, slice.sd)
    const dnns = Object.keys(slice.dnnConfigurations)
    subscribedSnssaiInfos[key] = { dnnInfos: dnns.map((dnn): Dnn => ({ dnn })) }

    const smPolicyDnnData: Record<string, Dnn> = {}
    for (const dnn of dnns) {
      smPolicyDnnData[dnn] = { dnn }
    }
    smPolicySnssaiData[key] = { snssai: { sst: toInt(slice.sst), sd: slice.sd }, smPolicyDnnData }
  }

  const gpsi = form.gpsi.trim()

  return {
    plmnID,
    ueId,
    AuthenticationSubscription: {
      authenticationMethod: '5G_AKA',
      authenticationManagementField: form.amf,
      sequenceNumber: form.sequenceNumber,
      permanentKey: { permanentKeyValue: form.permanentKey, encryptionKey: 0, encryptionAlgorithm: 0 },
      // Stored as OPc, not OP: free5GC's own default test subscriber uses
      // this exact key material ("8e27b6af0e692e750f32667a3b14605d") as OPc,
      // and free-ran-ue's uecfg.yaml field is literally named encOpcKey (it
      // uses the value as-is, it does not derive OPc from OP). Storing it as
      // OP instead makes the network derive a different actual OPc than what
      // the UE uses directly, which fails Milenage AUTN verification
      // (XMAC-A/MAC-A mismatch) even though every other field is correct.
      milenage: { op: { opValue: '', encryptionKey: 0, encryptionAlgorithm: 0 } },
      opc: { opcValue: form.opValue, encryptionKey: 0, encryptionAlgorithm: 0 },
    },
    AccessAndMobilitySubscriptionData: {
      // Always a one-element array, even when gpsi is blank (["msisdn-"]) - matches
      // webconsole's own frontend exactly. A genuinely empty array makes webconsole's
      // backend panic (getMsisdn does reflect.ValueOf(nil).Len() when the omitempty
      // bson tag drops a zero-length slice), so this is a correctness fix, not a
      // stylistic one.
      gpsis: [gpsi.startsWith('msisdn-') ? gpsi : `msisdn-${gpsi}`],
      subscribedUeAmbr: { uplink: form.ambrUplink, downlink: form.ambrDownlink },
      nssai: {
        defaultSingleNssais: slices.filter((s) => s.isDefaultSlice).map((slice) => ({ sst: toInt(slice.sst), sd: slice.sd })),
        singleNssais: slices.filter((s) => !s.isDefaultSlice).map((slice) => ({ sst: toInt(slice.sst), sd: slice.sd })),
      },
    },
    SessionManagementSubscriptionData: sessionManagementSubscriptionData,
    SmfSelectionSubscriptionData: { subscribedSnssaiInfos },
    AmPolicyData: { subscCats: ['free5gc'] },
    SmPolicyData: { smPolicySnssaiData },
    FlowRules: flowRules,
    QosFlows: qosFlows,
    ChargingDatas: chargingDatas,
  }
}

export function fromSubscription(sub: Subscription): SubscriberFormState {
  const plmnID = sub.plmnID || ''
  const defaultSnssaiKeys = new Set((sub.AccessAndMobilitySubscriptionData?.nssai?.defaultSingleNssais || [])
    .map((n) => snssaiKey(String(n.sst), n.sd || '')))

  const sessions: SessionFormRow[] = []
  for (const slice of sub.SessionManagementSubscriptionData || []) {
    const sst = String(slice.singleNssai?.sst ?? '')
    const sd = slice.singleNssai?.sd || ''
    const sliceSnssaiKey = snssaiKey(sst, sd)
    const dnnConfigurations = slice.dnnConfigurations || {}
    for (const [dnn, config] of Object.entries(dnnConfigurations)) {
      const matchingFlowRules = (sub.FlowRules || []).filter((r) => r.snssai === sliceSnssaiKey && r.dnn === dnn)
      const flowRules = matchingFlowRules.map((rule) => {
        const qos = (sub.QosFlows || []).find((q) => q.snssai === sliceSnssaiKey && q.dnn === dnn && q.qosRef === rule.qosRef)
        const charging = (sub.ChargingDatas || []).find((c) => c.snssai === sliceSnssaiKey && c.dnn === dnn && c.qosRef === rule.qosRef)
        return makeFlowRuleBundle({
          filter: rule.filter || '',
          precedence: rule.precedence !== undefined ? String(rule.precedence) : '',
          fiveQi: qos?.['5qi'] !== undefined ? String(qos['5qi']) : '',
          uplinkGbr: qos?.gbrUL || '',
          downlinkGbr: qos?.gbrDL || '',
          uplinkMbr: qos?.mbrUL || '',
          downlinkMbr: qos?.mbrDL || '',
          chargingMethod: charging?.chargingMethod || '',
          quota: charging?.quota || '',
          unitCost: charging?.unitCost || '',
        })
      })

      sessions.push(makeSessionRow({
        sst,
        sd,
        isDefaultSlice: defaultSnssaiKeys.size === 0 || defaultSnssaiKeys.has(sliceSnssaiKey),
        dnn,
        pduSessionType: config.pduSessionTypes?.defaultSessionType || 'IPV4',
        sscMode: config.sscModes?.defaultSscMode || 'SSC_MODE_1',
        fiveQi: String(config['5gQosProfile']?.['5qi'] ?? ''),
        arpPriority: String(config['5gQosProfile']?.arp?.priorityLevel ?? ''),
        ambrUplink: config.sessionAmbr?.uplink || '',
        ambrDownlink: config.sessionAmbr?.downlink || '',
        upIntegr: config.upSecurity?.upIntegr || '',
        upConfid: config.upSecurity?.upConfid || '',
        showUpSecurity: Boolean(config.upSecurity?.upIntegr || config.upSecurity?.upConfid),
        flowRules,
      }))
    }
  }

  return {
    ueId: sub.ueId || '',
    plmnMcc: plmnID.slice(0, 3),
    plmnMnc: plmnID.slice(3),
    gpsi: (sub.AccessAndMobilitySubscriptionData?.gpsis?.[0] || '').replace(/^msisdn-/, ''),
    permanentKey: sub.AuthenticationSubscription?.permanentKey?.permanentKeyValue || '',
    // Mirrors webconsole's own read-side precedence (SubscriberRead.tsx
    // operationCodeValue()): prefer OP if a subscriber genuinely has one set,
    // otherwise fall back to OPc - covers subscribers created either way.
    opValue: sub.AuthenticationSubscription?.milenage?.op?.opValue
      || sub.AuthenticationSubscription?.opc?.opcValue
      || '',
    sequenceNumber: sub.AuthenticationSubscription?.sequenceNumber || '',
    amf: sub.AuthenticationSubscription?.authenticationManagementField || '',
    ambrUplink: sub.AccessAndMobilitySubscriptionData?.subscribedUeAmbr?.uplink || '',
    ambrDownlink: sub.AccessAndMobilitySubscriptionData?.subscribedUeAmbr?.downlink || '',
    sessions: sessions.length > 0 ? sessions : [makeSessionRow()],
  }
}
