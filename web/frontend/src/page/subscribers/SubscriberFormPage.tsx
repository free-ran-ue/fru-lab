import { useCallback, useEffect, useMemo, useState, type FormEvent } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import AppLayout from '../../components/layout/AppLayout'
import NotificationContainer from '../../components/notifications/NotificationContainer'
import { useNotifications } from '../../hooks/useNotifications'
import { webconsoleApi, extractWebconsoleErrorMessage } from '../../webconsoleApiClient'
import {
  CHARGING_METHODS,
  PDU_SESSION_TYPES,
  SSC_MODES,
  UP_SECURITY_OPTIONS,
  bulkUeIds,
  emptySubscriberForm,
  fromSubscription,
  makeDefaultUpSecurity,
  makeFlowRuleBundle,
  makeSessionRow,
  snssaiKey,
  toSubscription,
  type FlowRuleBundleRow,
  type SessionFormRow,
  type SubscriberFormState,
} from './subscriberForm'
import { BULK_CONCURRENCY, runLimited } from './bulk'
import styles from './webconsole-style.module.css'

interface BulkFailure {
  ueId: string
  message: string
}

function updateRow<T extends { id: string }>(rows: T[], id: string, patch: Partial<T>): T[] {
  return rows.map((row) => (row.id === id ? { ...row, ...patch } : row))
}

interface FieldProps {
  label: string
  required?: boolean
  children: React.ReactNode
}

function Field({ label, required, children }: FieldProps) {
  return (
    <div className={styles.fld}>
      <label>{label}{required && <span className={styles.required}> *</span>}</label>
      {children}
    </div>
  )
}

export default function SubscriberFormPage() {
  const navigate = useNavigate()
  const { ueId: editUeId, plmnId: editPlmnId } = useParams<{ ueId: string; plmnId: string }>()
  const isEditMode = Boolean(editUeId && editPlmnId)
  const [searchParams] = useSearchParams()
  // count > 1 creates that many subscribers from this form, IMSI +1 each
  const count = isEditMode ? 1 : Math.max(1, Math.floor(Number(searchParams.get('count')) || 1))
  const isBulk = count > 1

  const [form, setForm] = useState<SubscriberFormState>(emptySubscriberForm())
  const [isLoading, setIsLoading] = useState(isEditMode)
  const [isSaving, setIsSaving] = useState(false)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [progress, setProgress] = useState(0)
  const [bulkFailures, setBulkFailures] = useState<BulkFailure[]>([])
  const bulkIds = useMemo(() => (isBulk ? bulkUeIds(form.ueId, count) : null), [isBulk, form.ueId, count])

  const { errors, successes, addError, addSuccess, removeNotification } = useNotifications()

  const loadForEdit = useCallback(async () => {
    if (!editUeId || !editPlmnId) return
    setIsLoading(true)
    setLoadError(null)
    try {
      const response = await webconsoleApi.getSubscriberByID(editUeId, editPlmnId)
      setForm(fromSubscription(response.data))
    } catch (error) {
      setLoadError(extractWebconsoleErrorMessage(error, 'Failed to load subscriber'))
    } finally {
      setIsLoading(false)
    }
  }, [editUeId, editPlmnId])

  useEffect(() => {
    if (isEditMode) {
      loadForEdit()
    }
  }, [isEditMode, loadForEdit])

  // createMany posts one subscription per IMSI: the form's subscription
  // with only ueId changed. It keeps going past failures and reports them.
  async function createMany(ids: string[]): Promise<BulkFailure[]> {
    const base = toSubscription(form)
    setProgress(0)
    setBulkFailures([])
    const failed = await runLimited(ids, BULK_CONCURRENCY,
      (ueId) => webconsoleApi.postSubscriberByID(ueId, base.plmnID, { ...base, ueId }), setProgress)
    return failed.map((f) => ({ ueId: f.item, message: extractWebconsoleErrorMessage(f.error, 'Failed to create') }))
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (isBulk) {
      if (typeof bulkIds === 'string' || bulkIds === null) {
        addError(bulkIds ?? 'Invalid SUPI')
        return
      }
      setIsSaving(true)
      try {
        const failures = await createMany(bulkIds)
        if (failures.length === 0) {
          navigate('/subscribers')
          return
        }
        setBulkFailures(failures)
        addError(`Created ${bulkIds.length - failures.length} of ${bulkIds.length}; ${failures.length} failed (listed below)`)
      } finally {
        setIsSaving(false)
      }
      return
    }
    setIsSaving(true)
    try {
      const subscription = toSubscription(form)
      if (isEditMode && editUeId && editPlmnId) {
        await webconsoleApi.putSubscriberByID(editUeId, editPlmnId, subscription)
        addSuccess(`Updated ${subscription.ueId}`)
      } else {
        await webconsoleApi.postSubscriberByID(subscription.ueId, subscription.plmnID, subscription)
        addSuccess(`Created ${subscription.ueId}`)
      }
      navigate('/subscribers')
    } catch (error) {
      addError(extractWebconsoleErrorMessage(error, 'Failed to save subscriber'))
    } finally {
      setIsSaving(false)
    }
  }

  function updateSession(id: string, patch: Partial<SessionFormRow>) {
    setForm((current) => ({ ...current, sessions: updateRow(current.sessions, id, patch) }))
  }

  function updateFlowRule(sessionId: string, ruleId: string, patch: Partial<FlowRuleBundleRow>) {
    setForm((current) => ({
      ...current,
      sessions: current.sessions.map((session) => (
        session.id === sessionId
          ? { ...session, flowRules: updateRow(session.flowRules, ruleId, patch) }
          : session
      )),
    }))
  }

  const title = isEditMode ? 'Edit subscriber' : isBulk ? `Add ${count} subscribers` : 'Add subscriber'
  const crumbs = [
    { label: 'Subscribers', to: '/subscribers' },
    ...(isEditMode
      ? [{ label: <span className="num">{editUeId}</span>, to: `/subscribers/${encodeURIComponent(editUeId!)}/${encodeURIComponent(editPlmnId!)}` }, { label: 'Edit' }]
      : [{ label: isBulk ? `New (${count})` : 'New' }]),
  ]
  const header = (
    <section className={styles.pageHeader}>
      <div className={styles.titleBlock}>
        <h1 className={styles.title}>{title}</h1>
        <p className={styles.subtitle}>
          {isBulk
            ? 'You fill in the first subscriber; the rest get the next IMSIs (+1 each) with every other field the same.'
            : 'A subscription in free5GC’s UDR: identity and keys, the UE AMBR, and one or more slices with their DNN.'}
        </p>
      </div>
    </section>
  )

  if (isEditMode && (isLoading || loadError)) {
    return (
      <AppLayout crumbs={crumbs}>
        {header}
        <section className={styles.panel}>
          <p className={styles.emptyState}>{isLoading ? 'Loading subscriber…' : loadError}</p>
        </section>
        {loadError && (
          <div>
            <button type="button" className={styles.btn} onClick={() => navigate('/subscribers')}>Back to subscribers</button>
          </div>
        )}
      </AppLayout>
    )
  }

  return (
    <AppLayout
      crumbs={crumbs}
      overlay={<NotificationContainer errors={errors} successes={successes} onClose={removeNotification} />}
    >
      {header}

        <form onSubmit={handleSubmit} className={styles.form}>
          <section className={styles.sec}>
            <div className={styles.secH}><h2>Identity and authentication</h2><span className={styles.hint}>SUPI, PLMN and 5G-AKA keys (OPc)</span></div>
            <div className={styles.secB}>
                <Field label="SUPI (IMSI)" required>
                  <input
                    className={styles.input}
                    placeholder="imsi-208930000000100"
                    value={form.ueId}
                    onChange={(event) => setForm((current) => ({ ...current, ueId: event.target.value }))}
                    disabled={isEditMode}
                    required
                  />
                  {isBulk && (
                    <p className={typeof bulkIds === 'string' ? styles.hintError : styles.hint}>
                      {typeof bulkIds === 'string'
                        ? bulkIds
                        : `Creates ${count} subscribers, ${bulkIds?.[0]} to ${bulkIds?.[bulkIds.length - 1]}; every other field is the same for all of them.`}
                    </p>
                  )}
                </Field>
                <Field label="PLMN ID" required>
                  <div className={styles.inputPair}>
                    <input
                      className={styles.input}
                      aria-label="MCC"
                      placeholder="MCC"
                      value={form.plmnMcc}
                      onChange={(event) => setForm((current) => ({ ...current, plmnMcc: event.target.value }))}
                      disabled={isEditMode}
                      required
                    />
                    <input
                      className={styles.input}
                      aria-label="MNC"
                      placeholder="MNC"
                      value={form.plmnMnc}
                      onChange={(event) => setForm((current) => ({ ...current, plmnMnc: event.target.value }))}
                      disabled={isEditMode}
                      required
                    />
                  </div>
                </Field>
                <Field label="GPSI (MSISDN)">
                  <input
                    className={styles.input}
                    placeholder="0900000000"
                    value={form.gpsi}
                    onChange={(event) => setForm((current) => ({ ...current, gpsi: event.target.value }))}
                  />
                </Field>
                <Field label="Authentication Management Field (AMF)">
                  <input
                    className={styles.input}
                    value={form.amf}
                    onChange={(event) => setForm((current) => ({ ...current, amf: event.target.value }))}
                    required
                  />
                </Field>
                <Field label="Operator Code Value (OPc)" required>
                  <input
                    className={styles.input}
                    value={form.opValue}
                    onChange={(event) => setForm((current) => ({ ...current, opValue: event.target.value }))}
                    required
                  />
                </Field>
                <Field label="SQN" required>
                  <input
                    className={styles.input}
                    value={form.sequenceNumber}
                    onChange={(event) => setForm((current) => ({ ...current, sequenceNumber: event.target.value }))}
                    required
                  />
                </Field>
                <Field label="Permanent Authentication Key" required>
                  <input
                    className={styles.input}
                    value={form.permanentKey}
                    onChange={(event) => setForm((current) => ({ ...current, permanentKey: event.target.value }))}
                    required
                  />
                </Field>
              </div>
          </section>

          <section className={styles.sec}>
            <div className={styles.secH}><h2>Subscribed UE AMBR</h2><span className={styles.hint}>aggregate bit rate across all sessions</span></div>
            <div className={styles.secB}>
                <Field label="Uplink" required>
                  <input
                    className={styles.input}
                    placeholder="1 Gbps"
                    value={form.ambrUplink}
                    onChange={(event) => setForm((current) => ({ ...current, ambrUplink: event.target.value }))}
                    required
                  />
                </Field>
                <Field label="Downlink" required>
                  <input
                    className={styles.input}
                    placeholder="2 Gbps"
                    value={form.ambrDownlink}
                    onChange={(event) => setForm((current) => ({ ...current, ambrDownlink: event.target.value }))}
                    required
                  />
                </Field>
              </div>
          </section>
          {form.sessions.map((session, sessionIndex) => (
            <section key={session.id} className={styles.sec}>
              <div className={styles.secH}>
                <h2>S-NSSAI <span className="num">{snssaiKey(session.sst, session.sd)}</span></h2>
                <span className={styles.hint}>slice {sessionIndex + 1} of {form.sessions.length}</span>
                {form.sessions.length > 1 && (
                  <button
                    type="button"
                    className={`${styles.linkDanger} ${styles.headAction}`}
                    onClick={() => setForm((current) => ({
                      ...current,
                      sessions: current.sessions.filter((item) => item.id !== session.id),
                    }))}
                  >
                    Delete slice
                  </button>
                )}
              </div>
              <div className={styles.secB}>
                <Field label="SST" required>
                  <input className={styles.input} value={session.sst} onChange={(event) => updateSession(session.id, { sst: event.target.value })} required />
                </Field>
                <Field label="SD">
                  <input className={styles.input} value={session.sd} onChange={(event) => updateSession(session.id, { sd: event.target.value })} />
                </Field>
                <div className={styles.fld}>
                  <label htmlFor={`default-slice-${session.id}`}>Default S-NSSAI</label>
                  <div className={styles.checkboxRow}>
                    <input
                      type="checkbox"
                      id={`default-slice-${session.id}`}
                      checked={session.isDefaultSlice}
                      onChange={(event) => updateSession(session.id, { isDefaultSlice: event.target.checked })}
                    />
                    <span>{session.isDefaultSlice ? 'Yes' : 'No'}</span>
                  </div>
                </div>
              </div>

              <div className={styles.subH}><h3>DNN configuration</h3><span className={styles.hint}>session AMBR, QoS and PDU session type</span></div>
              <div className={styles.secB}>
                <Field label="DNN" required>
                  <input className={styles.input} value={session.dnn} onChange={(event) => updateSession(session.id, { dnn: event.target.value })} required />
                </Field>
                <Field label="Uplink AMBR" required>
                  <input className={styles.input} value={session.ambrUplink} onChange={(event) => updateSession(session.id, { ambrUplink: event.target.value })} required />
                </Field>
                <Field label="Downlink AMBR" required>
                  <input className={styles.input} value={session.ambrDownlink} onChange={(event) => updateSession(session.id, { ambrDownlink: event.target.value })} required />
                </Field>
                <Field label="Default 5QI" required>
                  <input className={styles.input} value={session.fiveQi} onChange={(event) => updateSession(session.id, { fiveQi: event.target.value })} required />
                </Field>
                <Field label="ARP Priority">
                  <input className={styles.input} value={session.arpPriority} onChange={(event) => updateSession(session.id, { arpPriority: event.target.value })} />
                </Field>
                <Field label="PDU Session Type">
                  <select className={styles.input} value={session.pduSessionType} onChange={(event) => updateSession(session.id, { pduSessionType: event.target.value })}>
                    {PDU_SESSION_TYPES.map((option) => <option key={option} value={option}>{option}</option>)}
                  </select>
                </Field>
                <Field label="SSC Mode">
                  <select className={styles.input} value={session.sscMode} onChange={(event) => updateSession(session.id, { sscMode: event.target.value })}>
                    {SSC_MODES.map((option) => <option key={option} value={option}>{option}</option>)}
                  </select>
                </Field>
              </div>

              {session.showUpSecurity && (
                <>
                  <div className={styles.subH}>
                    <h3>UP security</h3>
                    <span className={styles.hint}>user-plane integrity and ciphering</span>
                    <button
                      type="button"
                      className={`${styles.linkDanger} ${styles.headAction}`}
                      onClick={() => updateSession(session.id, { showUpSecurity: false, upIntegr: '', upConfid: '' })}
                    >
                      Delete
                    </button>
                  </div>
                  <div className={styles.secB}>
                    <Field label="UP Integrity">
                      <select className={styles.input} value={session.upIntegr} onChange={(event) => updateSession(session.id, { upIntegr: event.target.value })}>
                        {UP_SECURITY_OPTIONS.map((option) => <option key={option || 'none'} value={option}>{option || '—'}</option>)}
                      </select>
                    </Field>
                    <Field label="UP Confidentiality">
                      <select className={styles.input} value={session.upConfid} onChange={(event) => updateSession(session.id, { upConfid: event.target.value })}>
                        {UP_SECURITY_OPTIONS.map((option) => <option key={option || 'none'} value={option}>{option || '—'}</option>)}
                      </select>
                    </Field>
                  </div>
                </>
              )}

              {session.flowRules.map((rule, ruleIndex) => (
                <div key={rule.id}>
                  <div className={styles.subH}>
                    <h3>Flow rule {ruleIndex + 1}</h3>
                    <span className={styles.hint}>IP filter, QoS and charging</span>
                    <button
                      type="button"
                      className={`${styles.linkDanger} ${styles.headAction}`}
                      onClick={() => updateSession(session.id, { flowRules: session.flowRules.filter((item) => item.id !== rule.id) })}
                    >
                      Delete
                    </button>
                  </div>
                  <div className={styles.secB}>
                    <Field label="IP Filter" required>
                      <input className={styles.input} placeholder="permit out ip from any to any" value={rule.filter} onChange={(event) => updateFlowRule(session.id, rule.id, { filter: event.target.value })} required />
                    </Field>
                    <Field label="Precedence" required>
                      <input className={styles.input} value={rule.precedence} onChange={(event) => updateFlowRule(session.id, rule.id, { precedence: event.target.value })} required />
                    </Field>
                    <Field label="5QI">
                      <input className={styles.input} value={rule.fiveQi} onChange={(event) => updateFlowRule(session.id, rule.id, { fiveQi: event.target.value })} />
                    </Field>
                    <Field label="Uplink GBR">
                      <input className={styles.input} value={rule.uplinkGbr} onChange={(event) => updateFlowRule(session.id, rule.id, { uplinkGbr: event.target.value })} />
                    </Field>
                    <Field label="Downlink GBR">
                      <input className={styles.input} value={rule.downlinkGbr} onChange={(event) => updateFlowRule(session.id, rule.id, { downlinkGbr: event.target.value })} />
                    </Field>
                    <Field label="Uplink MBR">
                      <input className={styles.input} value={rule.uplinkMbr} onChange={(event) => updateFlowRule(session.id, rule.id, { uplinkMbr: event.target.value })} />
                    </Field>
                    <Field label="Downlink MBR">
                      <input className={styles.input} value={rule.downlinkMbr} onChange={(event) => updateFlowRule(session.id, rule.id, { downlinkMbr: event.target.value })} />
                    </Field>
                    <Field label="Charging Method">
                      <select className={styles.input} value={rule.chargingMethod} onChange={(event) => updateFlowRule(session.id, rule.id, { chargingMethod: event.target.value })}>
                        {CHARGING_METHODS.map((option) => <option key={option || 'none'} value={option}>{option || '—'}</option>)}
                      </select>
                    </Field>
                    <Field label="Quota (monetary)">
                      <input className={styles.input} value={rule.quota} onChange={(event) => updateFlowRule(session.id, rule.id, { quota: event.target.value })} />
                    </Field>
                    <Field label="Unit Cost (money per byte)">
                      <input className={styles.input} value={rule.unitCost} onChange={(event) => updateFlowRule(session.id, rule.id, { unitCost: event.target.value })} />
                    </Field>
                  </div>
                </div>
              ))}

              <div className={styles.addRow}>
                {!session.showUpSecurity && (
                  <button
                    type="button"
                    className={styles.btnAdd}
                    onClick={() => updateSession(session.id, { showUpSecurity: true, ...makeDefaultUpSecurity() })}
                  >
                    + UP security
                  </button>
                )}
                <button
                  type="button"
                  className={styles.btnAdd}
                  onClick={() => updateSession(session.id, { flowRules: [...session.flowRules, makeFlowRuleBundle()] })}
                >
                  + Flow rule
                </button>
              </div>
            </section>
          ))}

          <div>
            <button
              type="button"
              className={styles.btnAdd}
              onClick={() => setForm((current) => ({ ...current, sessions: [...current.sessions, makeSessionRow()] }))}
            >
              + S-NSSAI
            </button>
          </div>

          {bulkFailures.length > 0 && (
            <section className={styles.sec}>
              <div className={styles.secH}>
                <h2>{bulkFailures.length} subscriber{bulkFailures.length === 1 ? '' : 's'} could not be created</h2>
              </div>
              <ul className={styles.failureList}>
                {bulkFailures.map((f) => <li key={f.ueId}><code>{f.ueId}</code>: {f.message}</li>)}
              </ul>
            </section>
          )}

          <div className={styles.actionsRow}>
            <button type="button" className={styles.btn} onClick={() => navigate('/subscribers')} disabled={isSaving}>
              {bulkFailures.length > 0 ? 'Back to subscribers' : 'Cancel'}
            </button>
            <button type="submit" className={`${styles.btn} ${styles.btnPrimary}`} disabled={isSaving || (isBulk && typeof bulkIds === 'string')}>
              {isSaving
                ? (isBulk ? `Creating ${progress}/${count}…` : 'Saving…')
                : isEditMode ? 'Save' : isBulk ? `Create ${count}` : 'Create'}
            </button>
          </div>
        </form>
    </AppLayout>
  )
}
