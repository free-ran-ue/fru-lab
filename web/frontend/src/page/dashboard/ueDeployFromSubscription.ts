import type { DeployUeRequest } from '../../api'
import type { Subscription } from '../../webconsoleApi'
import { fromSubscription } from '../subscribers/subscriberForm'

// Derives the fields a UE instance's config needs straight from a webconsole
// Subscription - the fru-lab backend never talks to webconsole itself, so
// this conversion always happens here, in the browser.
export function subscriptionToUeDeployRequest(subscription: Subscription): DeployUeRequest {
  const form = fromSubscription(subscription)
  const plmnId = `${form.plmnMcc}${form.plmnMnc}`
  const rawImsi = form.ueId.replace(/^imsi-/, '')
  const msin = rawImsi.startsWith(plmnId) ? rawImsi.slice(plmnId.length) : rawImsi

  const session = form.sessions[0]

  return {
    mcc: form.plmnMcc,
    mnc: form.plmnMnc,
    msin,
    permanentKey: form.permanentKey,
    opValue: form.opValue,
    amf: form.amf,
    sqn: form.sequenceNumber,
    dnn: session?.dnn || 'internet',
    sst: session?.sst || '1',
    sd: session?.sd || '',
  }
}
