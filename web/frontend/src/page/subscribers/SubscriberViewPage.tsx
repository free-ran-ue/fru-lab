import { useCallback, useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import AppLayout from '../../components/layout/AppLayout'
import { webconsoleApi, extractWebconsoleErrorMessage } from '../../webconsoleApiClient'
import { fromSubscription, snssaiKey, type SubscriberFormState } from './subscriberForm'
import styles from './webconsole-style.module.css'
import { SortTh, useSort } from '../../lab/sort'

// a numeric field as a number, so 10 sorts after 9
const num = (v: unknown) => (v === '' || v === undefined || v === null || Number.isNaN(Number(v)) ? String(v ?? '') : Number(v))

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
  const sessionSort = useSort(form?.sessions ?? [], {
    snssai: (x) => snssaiKey(x.sst, x.sd), sst: (x) => num(x.sst), sd: (x) => x.sd, def: (x) => (x.isDefaultSlice ? 0 : 1), dnn: (x) => x.dnn,
    ul: (x) => x.ambrUplink, dl: (x) => x.ambrDownlink, fiveQi: (x) => num(x.fiveQi), arp: (x) => num(x.arpPriority),
    pdu: (x) => x.pduSessionType, ssc: (x) => x.sscMode, integ: (x) => (x.showUpSecurity ? x.upIntegr : ''), confid: (x) => (x.showUpSecurity ? x.upConfid : ''),
  })
  const ruleSort = useSort(flowRules, {
    snssai: ({ session }) => snssaiKey(session.sst, session.sd), dnn: ({ session }) => session.dnn, filter: ({ rule }) => rule.filter,
    prec: ({ rule }) => num(rule.precedence), fiveQi: ({ rule }) => num(rule.fiveQi), ulGbr: ({ rule }) => rule.uplinkGbr, dlGbr: ({ rule }) => rule.downlinkGbr,
    ulMbr: ({ rule }) => rule.uplinkMbr, dlMbr: ({ rule }) => rule.downlinkMbr, charging: ({ rule }) => rule.chargingMethod, quota: ({ rule }) => num(rule.quota), cost: ({ rule }) => num(rule.unitCost),
  })

  return (
    <AppLayout crumbs={[{ label: 'Subscribers', to: '/core-network/subscribers' }, { label: <span className="num">{ueId}</span> }]}>
      <section className={styles.pageHeader}>
        <div className={styles.titleBlock}>
          <h1 className={`${styles.title} num`}>{ueId}</h1>
          <p className={styles.subtitle}>
            PLMN <span className="num">{plmnId}</span>
            {form?.gpsi && <> · GPSI <span className="num">{form.gpsi}</span></>}
          </p>
        </div>
        <div className={styles.headerActions}>
          <button type="button" className={styles.btn} onClick={() => navigate('/core-network/subscribers')}>Back</button>
          {!isLoading && !loadError && form && (
            <button
              type="button"
              className={`${styles.btn} ${styles.btnPrimary}`}
              onClick={() => navigate(`/core-network/subscribers/${encodeURIComponent(ueId!)}/${encodeURIComponent(plmnId!)}/edit`)}
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
                    <SortTh k="snssai" sort={sessionSort.sort} onSort={sessionSort.toggle} className={styles.th}>S-NSSAI</SortTh>
                    <SortTh k="sst" sort={sessionSort.sort} onSort={sessionSort.toggle} className={styles.th}>SST</SortTh>
                    <SortTh k="sd" sort={sessionSort.sort} onSort={sessionSort.toggle} className={styles.th}>SD</SortTh>
                    <SortTh k="def" sort={sessionSort.sort} onSort={sessionSort.toggle} className={styles.th}>Default</SortTh>
                    <SortTh k="dnn" sort={sessionSort.sort} onSort={sessionSort.toggle} className={styles.th}>DNN</SortTh>
                    <SortTh k="ul" sort={sessionSort.sort} onSort={sessionSort.toggle} className={`${styles.th} ${styles.right}`}>UL AMBR</SortTh>
                    <SortTh k="dl" sort={sessionSort.sort} onSort={sessionSort.toggle} className={`${styles.th} ${styles.right}`}>DL AMBR</SortTh>
                    <SortTh k="fiveQi" sort={sessionSort.sort} onSort={sessionSort.toggle} className={`${styles.th} ${styles.right}`}>5QI</SortTh>
                    <SortTh k="arp" sort={sessionSort.sort} onSort={sessionSort.toggle} className={`${styles.th} ${styles.right}`}>ARP</SortTh>
                    <SortTh k="pdu" sort={sessionSort.sort} onSort={sessionSort.toggle} className={styles.th}>PDU type</SortTh>
                    <SortTh k="ssc" sort={sessionSort.sort} onSort={sessionSort.toggle} className={styles.th}>SSC mode</SortTh>
                    <SortTh k="integ" sort={sessionSort.sort} onSort={sessionSort.toggle} className={styles.th}>UP integrity</SortTh>
                    <SortTh k="confid" sort={sessionSort.sort} onSort={sessionSort.toggle} className={styles.th}>UP confidentiality</SortTh>
                  </tr>
                </thead>
                <tbody>
                  {sessionSort.sorted.map((session) => (
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
                      <SortTh k="snssai" sort={ruleSort.sort} onSort={ruleSort.toggle} className={styles.th}>S-NSSAI</SortTh>
                      <SortTh k="dnn" sort={ruleSort.sort} onSort={ruleSort.toggle} className={styles.th}>DNN</SortTh>
                      <SortTh k="filter" sort={ruleSort.sort} onSort={ruleSort.toggle} className={styles.th}>IP filter</SortTh>
                      <SortTh k="prec" sort={ruleSort.sort} onSort={ruleSort.toggle} className={`${styles.th} ${styles.right}`}>Precedence</SortTh>
                      <SortTh k="fiveQi" sort={ruleSort.sort} onSort={ruleSort.toggle} className={`${styles.th} ${styles.right}`}>5QI</SortTh>
                      <SortTh k="ulGbr" sort={ruleSort.sort} onSort={ruleSort.toggle} className={`${styles.th} ${styles.right}`}>UL GBR</SortTh>
                      <SortTh k="dlGbr" sort={ruleSort.sort} onSort={ruleSort.toggle} className={`${styles.th} ${styles.right}`}>DL GBR</SortTh>
                      <SortTh k="ulMbr" sort={ruleSort.sort} onSort={ruleSort.toggle} className={`${styles.th} ${styles.right}`}>UL MBR</SortTh>
                      <SortTh k="dlMbr" sort={ruleSort.sort} onSort={ruleSort.toggle} className={`${styles.th} ${styles.right}`}>DL MBR</SortTh>
                      <SortTh k="charging" sort={ruleSort.sort} onSort={ruleSort.toggle} className={styles.th}>Charging</SortTh>
                      <SortTh k="quota" sort={ruleSort.sort} onSort={ruleSort.toggle} className={`${styles.th} ${styles.right}`}>Quota</SortTh>
                      <SortTh k="cost" sort={ruleSort.sort} onSort={ruleSort.toggle} className={`${styles.th} ${styles.right}`}>Unit cost</SortTh>
                    </tr>
                  </thead>
                  <tbody>
                    {ruleSort.sorted.map(({ session, rule }) => (
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
