import { useCallback, useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import Sidebar from '../../components/sidebar/Sidebar'
import Button from '../../components/button/button'
import { webconsoleApi, extractWebconsoleErrorMessage } from '../../webconsoleApiClient'
import { fromSubscription, snssaiKey, type SubscriberFormState } from './subscriberForm'
import styles from './webconsole-style.module.css'

function KeyValueTable({ rows }: { rows: Array<[string, string]> }) {
  return (
    <table className={styles.kvTable}>
      <tbody>
        {rows.map(([label, value]) => (
          <tr key={label}>
            <th>{label}</th>
            <td>{value || '—'}</td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}

export default function SubscriberViewPage() {
  const navigate = useNavigate()
  const { ueId, plmnId } = useParams<{ ueId: string; plmnId: string }>()

  const [form, setForm] = useState<SubscriberFormState | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [loadError, setLoadError] = useState<string | null>(null)

  const load = useCallback(async () => {
    if (!ueId || !plmnId) return
    setIsLoading(true)
    setLoadError(null)
    try {
      const response = await webconsoleApi.getSubscriberByID(ueId, plmnId)
      setForm(fromSubscription(response.data))
    } catch (error) {
      setLoadError(extractWebconsoleErrorMessage(error, 'Failed to load subscriber'))
    } finally {
      setIsLoading(false)
    }
  }, [ueId, plmnId])

  useEffect(() => {
    load()
  }, [load])

  return (
    <div className={styles.layout}>
      <Sidebar />

      <main className={styles.content}>
        <header className={styles.pageHeader}>
          <div className={styles.titleBlock}>
            <h1 className={styles.title}>View subscriber</h1>
          </div>
          {!isLoading && !loadError && form && (
            <div className={styles.headerActions}>
              <Button variant="secondary" onClick={() => navigate('/subscribers')}>Back</Button>
              <Button onClick={() => navigate(`/subscribers/${encodeURIComponent(ueId!)}/${encodeURIComponent(plmnId!)}/edit`)}>Edit</Button>
            </div>
          )}
        </header>

        {isLoading ? (
          <p className={styles.bodyText}>Loading subscriber…</p>
        ) : loadError ? (
          <>
            <p className={styles.bodyText}>{loadError}</p>
            <div><Button variant="secondary" onClick={() => navigate('/subscribers')}>Back to subscribers</Button></div>
          </>
        ) : form ? (
          <>
            <section className={styles.panel}>
              <div className={styles.panelHead}><h2>Identity and authentication</h2></div>
              <div className={styles.panelBody}>
                <KeyValueTable rows={[
                  ['PLMN ID', `${form.plmnMcc}${form.plmnMnc}`],
                  ['SUPI (IMSI)', form.ueId],
                  ['GPSI (MSISDN)', form.gpsi],
                  ['Authentication Management Field (AMF)', form.amf],
                  ['Authentication Method', '5G_AKA'],
                  ['K', form.permanentKey],
                  ['Operator Code Type', 'OPc'],
                  ['Operator Code Value (OPc)', form.opValue],
                  ['SQN', form.sequenceNumber],
                ]} />
              </div>
            </section>

            <section className={styles.panel}>
              <div className={styles.panelHead}><h2>Subscribed UE AMBR</h2></div>
              <div className={styles.panelBody}>
                <KeyValueTable rows={[
                  ['Uplink', form.ambrUplink],
                  ['Downlink', form.ambrDownlink],
                ]} />
              </div>
            </section>

            {form.sessions.map((session) => (
              <section key={session.id} className={styles.panel}>
                <div className={styles.panelHead}>
                  <h2>S-NSSAI Configuration ({snssaiKey(session.sst, session.sd)})</h2>
                </div>
                <div className={styles.panelBody}>
                  <KeyValueTable rows={[
                    ['SST', session.sst],
                    ['SD', session.sd],
                    ['Default S-NSSAI', session.isDefaultSlice ? 'Yes' : 'No'],
                  ]} />

                  <div className={styles.subpanel}>
                    <div className={styles.subpanelHead}><h3>DNN Configurations</h3></div>
                    <div className={styles.subpanelBody}>
                      <KeyValueTable rows={[
                        ['Data Network Name', session.dnn],
                        ['Uplink AMBR', session.ambrUplink],
                        ['Downlink AMBR', session.ambrDownlink],
                        ['Default 5QI', session.fiveQi],
                        ['ARP Priority', session.arpPriority],
                        ['PDU Session Type', session.pduSessionType],
                        ['SSC Mode', session.sscMode],
                      ]} />

                      {session.showUpSecurity && (
                        <div className={styles.subpanel}>
                          <div className={styles.subpanelHead}><h3>UP Security</h3></div>
                          <div className={styles.subpanelBody}>
                            <KeyValueTable rows={[
                              ['UP Integrity', session.upIntegr],
                              ['UP Confidentiality', session.upConfid],
                            ]} />
                          </div>
                        </div>
                      )}

                      {session.flowRules.map((rule, ruleIndex) => (
                        <div key={rule.id} className={styles.subpanel}>
                          <div className={styles.subpanelHead}><h3>Flow Rule {ruleIndex + 1}</h3></div>
                          <div className={styles.subpanelBody}>
                            <KeyValueTable rows={[
                              ['IP Filter', rule.filter],
                              ['Precedence', rule.precedence],
                              ['5QI', rule.fiveQi],
                              ['Uplink GBR', rule.uplinkGbr],
                              ['Downlink GBR', rule.downlinkGbr],
                              ['Uplink MBR', rule.uplinkMbr],
                              ['Downlink MBR', rule.downlinkMbr],
                              ['Charging Method', rule.chargingMethod],
                              ['Quota (monetary)', rule.quota],
                              ['Unit Cost (money per byte)', rule.unitCost],
                            ]} />
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                </div>
              </section>
            ))}
          </>
        ) : null}
      </main>
    </div>
  )
}
