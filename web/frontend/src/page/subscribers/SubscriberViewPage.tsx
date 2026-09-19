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
        <div className={styles.heroBand}>
          <div className={styles.heroBlobPink} aria-hidden="true" />
          <div className={styles.heroBlobBlue} aria-hidden="true" />
          <div className={styles.heroInner}>
            <div>
              <p className={styles.kicker}>Fru-Lab Control Plane</p>
              <h2 className={styles.title}>View Subscriber</h2>
            </div>
          </div>
        </div>

        <div className={styles.body}>
          {isLoading ? (
            <p>Loading subscriber…</p>
          ) : loadError ? (
            <>
              <p>{loadError}</p>
              <Button variant="secondary" onClick={() => navigate('/subscribers')}>Back to subscribers</Button>
            </>
          ) : form ? (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '1.5rem' }}>
              <div className={styles.actionsRow}>
                <Button variant="secondary" onClick={() => navigate('/subscribers')}>Back</Button>
                <Button onClick={() => navigate(`/subscribers/${encodeURIComponent(ueId!)}/${encodeURIComponent(plmnId!)}/edit`)}>Edit</Button>
              </div>

              <section className={styles.card}>
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
              </section>

              <div>
                <div className={styles.sectionHeading}><h3>Subscribed UE AMBR</h3></div>
                <section className={styles.card}>
                  <KeyValueTable rows={[
                    ['Uplink', form.ambrUplink],
                    ['Downlink', form.ambrDownlink],
                  ]} />
                </section>
              </div>

              {form.sessions.map((session) => (
                <div key={session.id}>
                  <div className={styles.sectionHeading}>
                    <h3>S-NSSAI Configuration ({snssaiKey(session.sst, session.sd)})</h3>
                  </div>
                  <section className={styles.card}>
                    <KeyValueTable rows={[
                      ['SST', session.sst],
                      ['SD', session.sd],
                      ['Default S-NSSAI', session.isDefaultSlice ? 'Yes' : 'No'],
                    ]} />

                    <div className={styles.nestedCard} style={{ marginTop: '1.25rem' }}>
                      <div className={styles.sectionHeading}><h3 style={{ fontSize: '0.95rem' }}>DNN Configurations</h3></div>
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
                        <div style={{ marginTop: '1.25rem' }}>
                          <div className={styles.sectionHeading}><h3 style={{ fontSize: '0.95rem' }}>UP Security</h3></div>
                          <div className={styles.nestedCard}>
                            <KeyValueTable rows={[
                              ['UP Integrity', session.upIntegr],
                              ['UP Confidentiality', session.upConfid],
                            ]} />
                          </div>
                        </div>
                      )}

                      {session.flowRules.map((rule, ruleIndex) => (
                        <div key={rule.id} style={{ marginTop: '1.25rem' }}>
                          <div className={styles.sectionHeading}><h3 style={{ fontSize: '0.95rem' }}>Flow Rule {ruleIndex + 1}</h3></div>
                          <div className={styles.nestedCard}>
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
                  </section>
                </div>
              ))}
            </div>
          ) : null}
        </div>
      </main>
    </div>
  )
}
