import { useCallback, useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import AppLayout from '../../components/layout/AppLayout'
import { webconsoleApi, extractWebconsoleErrorMessage } from '../../webconsoleApiClient'
import { fromSubscription, snssaiKey, type SubscriberFormState } from './subscriberForm'
import styles from './webconsole-style.module.css'

function KeyValueRows({ rows }: { rows: Array<[string, string]> }) {
  return (
    <table className={styles.table}>
      <tbody>
        {rows.map(([label, value]) => (
          <tr key={label} className={styles.row}>
            <td className={`${styles.td} ${styles.kvLabel}`}>{label}</td>
            <td className={`${styles.td} ${styles.kvValue}`}>{value || '—'}</td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}

function Panel({ title, meta, children }: { title: string; meta?: string; children: React.ReactNode }) {
  return (
    <section className={styles.panel}>
      <div className={styles.panelHead}>
        <h2>{title}</h2>
        {meta && <span className={`num ${styles.panelMeta}`}>{meta}</span>}
      </div>
      {children}
    </section>
  )
}

const dash = (v: string) => v || '—'

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

  const flowRules = form ? form.sessions.flatMap((session) => session.flowRules.map((rule) => ({ session, rule }))) : []

  return (
    <AppLayout crumbs={[{ label: 'Subscribers', to: '/subscribers' }, { label: <span className="num">{ueId}</span> }]}>
      <section className={styles.pageHeader}>
        <div className={styles.titleBlock}>
          <h1 className={`${styles.title} num`}>{ueId}</h1>
          <p className={styles.subtitle}>
            PLMN <span className="num">{plmnId}</span>
            {form?.gpsi && <> · GPSI <span className="num">{form.gpsi}</span></>}
          </p>
        </div>
        <div className={styles.headerActions}>
          <button type="button" className={styles.btn} onClick={() => navigate('/subscribers')}>Back</button>
          {!isLoading && !loadError && form && (
            <button
              type="button"
              className={`${styles.btn} ${styles.btnPrimary}`}
              onClick={() => navigate(`/subscribers/${encodeURIComponent(ueId!)}/${encodeURIComponent(plmnId!)}/edit`)}
            >
              Edit
            </button>
          )}
        </div>
      </section>

      {isLoading ? (
        <section className={styles.panel}><p className={styles.emptyState}>Loading subscriber…</p></section>
      ) : loadError ? (
        <section className={styles.panel}><p className={styles.emptyState}>{loadError}</p></section>
      ) : form ? (
        <>
          <Panel title="Identity and authentication" meta="5G-AKA · OPc">
            <KeyValueRows rows={[
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
          </Panel>

          <Panel title="Subscribed UE AMBR">
            <KeyValueRows rows={[
              ['Uplink', form.ambrUplink],
              ['Downlink', form.ambrDownlink],
            ]} />
          </Panel>

          <Panel title="S-NSSAI and DNN configurations" meta={`${form.sessions.length} slice${form.sessions.length === 1 ? '' : 's'}`}>
            <div className={styles.tableWrap}>
              <table className={styles.table}>
                <thead>
                  <tr>
                    <th className={styles.th}>S-NSSAI</th>
                    <th className={styles.th}>SST</th>
                    <th className={styles.th}>SD</th>
                    <th className={styles.th}>Default</th>
                    <th className={styles.th}>DNN</th>
                    <th className={`${styles.th} ${styles.right}`}>UL AMBR</th>
                    <th className={`${styles.th} ${styles.right}`}>DL AMBR</th>
                    <th className={`${styles.th} ${styles.right}`}>5QI</th>
                    <th className={`${styles.th} ${styles.right}`}>ARP</th>
                    <th className={styles.th}>PDU type</th>
                    <th className={styles.th}>SSC mode</th>
                    <th className={styles.th}>UP integrity</th>
                    <th className={styles.th}>UP confidentiality</th>
                  </tr>
                </thead>
                <tbody>
                  {form.sessions.map((session) => (
                    <tr key={session.id} className={styles.row}>
                      <td className={styles.td}><span className={styles.tag}>{snssaiKey(session.sst, session.sd)}</span></td>
                      <td className={`${styles.td} num`}>{dash(session.sst)}</td>
                      <td className={`${styles.td} num`}>{dash(session.sd)}</td>
                      <td className={styles.td}>{session.isDefaultSlice ? 'Yes' : 'No'}</td>
                      <td className={styles.td}>{dash(session.dnn)}</td>
                      <td className={`${styles.td} num ${styles.right}`}>{dash(session.ambrUplink)}</td>
                      <td className={`${styles.td} num ${styles.right}`}>{dash(session.ambrDownlink)}</td>
                      <td className={`${styles.td} num ${styles.right}`}>{dash(session.fiveQi)}</td>
                      <td className={`${styles.td} num ${styles.right}`}>{dash(session.arpPriority)}</td>
                      <td className={`${styles.td} num`}>{dash(session.pduSessionType)}</td>
                      <td className={`${styles.td} num`}>{dash(session.sscMode)}</td>
                      <td className={`${styles.td} num ${styles.mutedCell}`}>{session.showUpSecurity ? dash(session.upIntegr) : '—'}</td>
                      <td className={`${styles.td} num ${styles.mutedCell}`}>{session.showUpSecurity ? dash(session.upConfid) : '—'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </Panel>

          <Panel title="Flow rules" meta={`${flowRules.length} rule${flowRules.length === 1 ? '' : 's'}`}>
            {flowRules.length === 0 ? (
              <p className={styles.emptyState}>No flow rules.</p>
            ) : (
              <div className={styles.tableWrap}>
                <table className={styles.table}>
                  <thead>
                    <tr>
                      <th className={styles.th}>S-NSSAI</th>
                      <th className={styles.th}>DNN</th>
                      <th className={styles.th}>IP filter</th>
                      <th className={`${styles.th} ${styles.right}`}>Precedence</th>
                      <th className={`${styles.th} ${styles.right}`}>5QI</th>
                      <th className={`${styles.th} ${styles.right}`}>UL GBR</th>
                      <th className={`${styles.th} ${styles.right}`}>DL GBR</th>
                      <th className={`${styles.th} ${styles.right}`}>UL MBR</th>
                      <th className={`${styles.th} ${styles.right}`}>DL MBR</th>
                      <th className={styles.th}>Charging</th>
                      <th className={`${styles.th} ${styles.right}`}>Quota</th>
                      <th className={`${styles.th} ${styles.right}`}>Unit cost</th>
                    </tr>
                  </thead>
                  <tbody>
                    {flowRules.map(({ session, rule }) => (
                      <tr key={rule.id} className={styles.row}>
                        <td className={styles.td}><span className={styles.tag}>{snssaiKey(session.sst, session.sd)}</span></td>
                        <td className={styles.td}>{dash(session.dnn)}</td>
                        <td className={`${styles.td} num`}>{dash(rule.filter)}</td>
                        <td className={`${styles.td} num ${styles.right}`}>{dash(rule.precedence)}</td>
                        <td className={`${styles.td} num ${styles.right}`}>{dash(rule.fiveQi)}</td>
                        <td className={`${styles.td} num ${styles.right}`}>{dash(rule.uplinkGbr)}</td>
                        <td className={`${styles.td} num ${styles.right}`}>{dash(rule.downlinkGbr)}</td>
                        <td className={`${styles.td} num ${styles.right}`}>{dash(rule.uplinkMbr)}</td>
                        <td className={`${styles.td} num ${styles.right}`}>{dash(rule.downlinkMbr)}</td>
                        <td className={styles.td}>{dash(rule.chargingMethod)}</td>
                        <td className={`${styles.td} num ${styles.right}`}>{dash(rule.quota)}</td>
                        <td className={`${styles.td} num ${styles.right}`}>{dash(rule.unitCost)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </Panel>
        </>
      ) : null}
    </AppLayout>
  )
}
