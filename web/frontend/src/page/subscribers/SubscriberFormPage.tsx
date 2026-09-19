import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import Sidebar from '../../components/sidebar/Sidebar'
import Button from '../../components/button/button'
import NotificationContainer from '../../components/notifications/NotificationContainer'
import { useNotifications } from '../../hooks/useNotifications'
import { webconsoleApi, extractWebconsoleErrorMessage } from '../../webconsoleApiClient'
import {
  CHARGING_METHODS,
  PDU_SESSION_TYPES,
  SSC_MODES,
  UP_SECURITY_OPTIONS,
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
import styles from './webconsole-style.module.css'

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
    <div className={styles.field}>
      <label>{label}{required && <span className={styles.required}> *</span>}</label>
      {children}
    </div>
  )
}

export default function SubscriberFormPage() {
  const navigate = useNavigate()
  const { ueId: editUeId, plmnId: editPlmnId } = useParams<{ ueId: string; plmnId: string }>()
  const isEditMode = Boolean(editUeId && editPlmnId)

  const [form, setForm] = useState<SubscriberFormState>(emptySubscriberForm())
  const [isLoading, setIsLoading] = useState(isEditMode)
  const [isSaving, setIsSaving] = useState(false)
  const [loadError, setLoadError] = useState<string | null>(null)

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

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
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

  if (isEditMode && isLoading) {
    return (
      <div className={styles.layout}>
        <Sidebar />
        <main className={styles.content}>
          <div className={styles.body}><p>Loading subscriber…</p></div>
        </main>
      </div>
    )
  }

  if (isEditMode && loadError) {
    return (
      <div className={styles.layout}>
        <Sidebar />
        <main className={styles.content}>
          <div className={styles.body}>
            <p>{loadError}</p>
            <Button variant="secondary" onClick={() => navigate('/subscribers')}>Back to subscribers</Button>
          </div>
        </main>
      </div>
    )
  }

  return (
    <div className={styles.layout}>
      <NotificationContainer errors={errors} successes={successes} onClose={removeNotification} />
      <Sidebar />

      <main className={styles.content}>
        <div className={styles.heroBand}>
          <div className={styles.heroBlobPink} aria-hidden="true" />
          <div className={styles.heroBlobBlue} aria-hidden="true" />
          <div className={styles.heroInner}>
            <div>
              <p className={styles.kicker}>Fru-Lab Control Plane</p>
              <h2 className={styles.title}>{isEditMode ? 'Edit Subscriber' : 'Add Subscriber'}</h2>
            </div>
          </div>
        </div>

        <div className={styles.body}>
          <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: '1.5rem' }}>
            <section className={styles.card}>
              <div className={styles.fieldGrid}>
                <Field label="SUPI (IMSI)" required>
                  <input
                    className={styles.input}
                    placeholder="imsi-208930000000100"
                    value={form.ueId}
                    onChange={(event) => setForm((current) => ({ ...current, ueId: event.target.value }))}
                    disabled={isEditMode}
                    required
                  />
                </Field>
                <Field label="PLMN ID" required>
                  <div style={{ display: 'flex', gap: '0.5rem' }}>
                    <input
                      className={styles.input}
                      placeholder="MCC"
                      value={form.plmnMcc}
                      onChange={(event) => setForm((current) => ({ ...current, plmnMcc: event.target.value }))}
                      disabled={isEditMode}
                      required
                    />
                    <input
                      className={styles.input}
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

            <div>
              <div className={styles.sectionHeading}><h3>Subscribed UE AMBR</h3></div>
              <section className={styles.card}>
                <div className={styles.fieldGrid}>
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
            </div>

            <div className={styles.sliceList}>
              {form.sessions.map((session, sessionIndex) => (
                <div key={session.id}>
                  <div className={styles.sectionHeading}>
                    <h3>S-NSSAI Configuration ({snssaiKey(session.sst, session.sd)})</h3>
                    {form.sessions.length > 1 && (
                      <Button
                        variant="danger"
                        onClick={() => setForm((current) => ({
                          ...current,
                          sessions: current.sessions.filter((item) => item.id !== session.id),
                        }))}
                      >
                        Delete
                      </Button>
                    )}
                  </div>
                  <section className={styles.card}>
                    <div className={styles.fieldGrid}>
                      <Field label="SST" required>
                        <input className={styles.input} value={session.sst} onChange={(event) => updateSession(session.id, { sst: event.target.value })} required />
                      </Field>
                      <Field label="SD">
                        <input className={styles.input} value={session.sd} onChange={(event) => updateSession(session.id, { sd: event.target.value })} />
                      </Field>
                    </div>

                    <div className={styles.checkboxRow} style={{ margin: '1.25rem 0' }}>
                      <input
                        type="checkbox"
                        id={`default-slice-${session.id}`}
                        checked={session.isDefaultSlice}
                        onChange={(event) => updateSession(session.id, { isDefaultSlice: event.target.checked })}
                      />
                      <label htmlFor={`default-slice-${session.id}`}>Default S-NSSAI</label>
                    </div>

                    <div className={styles.nestedCard}>
                      <div className={styles.sectionHeading}><h3 style={{ fontSize: '0.95rem' }}>DNN Configurations</h3></div>

                      <Field label="DNN" required>
                        <input className={styles.input} value={session.dnn} onChange={(event) => updateSession(session.id, { dnn: event.target.value })} required />
                      </Field>

                      <div className={styles.fieldGrid} style={{ marginTop: '1.25rem' }}>
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
                          <select className={styles.select} value={session.pduSessionType} onChange={(event) => updateSession(session.id, { pduSessionType: event.target.value })}>
                            {PDU_SESSION_TYPES.map((option) => <option key={option} value={option}>{option}</option>)}
                          </select>
                        </Field>
                        <Field label="SSC Mode">
                          <select className={styles.select} value={session.sscMode} onChange={(event) => updateSession(session.id, { sscMode: event.target.value })}>
                            {SSC_MODES.map((option) => <option key={option} value={option}>{option}</option>)}
                          </select>
                        </Field>
                      </div>

                      {session.showUpSecurity ? (
                        <div style={{ marginTop: '1.25rem' }}>
                          <div className={styles.sectionHeading}>
                            <h3 style={{ fontSize: '0.95rem' }}>UP Security</h3>
                            <Button
                              variant="danger"
                              onClick={() => updateSession(session.id, { showUpSecurity: false, upIntegr: '', upConfid: '' })}
                            >
                              Delete
                            </Button>
                          </div>
                          <div className={styles.nestedCard}>
                            <div className={styles.fieldGrid}>
                              <Field label="UP Integrity">
                                <select className={styles.select} value={session.upIntegr} onChange={(event) => updateSession(session.id, { upIntegr: event.target.value })}>
                                  {UP_SECURITY_OPTIONS.map((option) => <option key={option || 'none'} value={option}>{option || '—'}</option>)}
                                </select>
                              </Field>
                              <Field label="UP Confidentiality">
                                <select className={styles.select} value={session.upConfid} onChange={(event) => updateSession(session.id, { upConfid: event.target.value })}>
                                  {UP_SECURITY_OPTIONS.map((option) => <option key={option || 'none'} value={option}>{option || '—'}</option>)}
                                </select>
                              </Field>
                            </div>
                          </div>
                        </div>
                      ) : (
                        <button
                          type="button"
                          className={styles.btnAdd}
                          style={{ marginTop: '1.25rem' }}
                          onClick={() => updateSession(session.id, { showUpSecurity: true, ...makeDefaultUpSecurity() })}
                        >
                          + UP Security
                        </button>
                      )}

                      {session.flowRules.map((rule, ruleIndex) => (
                        <div key={rule.id} style={{ marginTop: '1.5rem' }}>
                          <div className={styles.sectionHeading}>
                            <h3 style={{ fontSize: '0.95rem' }}>Flow Rule {ruleIndex + 1}</h3>
                            <Button
                              variant="danger"
                              onClick={() => updateSession(session.id, { flowRules: session.flowRules.filter((item) => item.id !== rule.id) })}
                            >
                              Delete
                            </Button>
                          </div>
                          <div className={styles.nestedCard}>
                            <div className={styles.fieldGrid}>
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
                                <select className={styles.select} value={rule.chargingMethod} onChange={(event) => updateFlowRule(session.id, rule.id, { chargingMethod: event.target.value })}>
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
                        </div>
                      ))}

                      <button
                        type="button"
                        className={styles.btnLink}
                        style={{ marginTop: '1.25rem', display: 'block' }}
                        onClick={() => updateSession(session.id, { flowRules: [...session.flowRules, makeFlowRuleBundle()] })}
                      >
                        + Flow Rule
                      </button>
                    </div>
                  </section>
                  {sessionIndex === form.sessions.length - 1 && (
                    <button
                      type="button"
                      className={styles.btnAdd}
                      style={{ marginTop: '1.25rem' }}
                      onClick={() => setForm((current) => ({ ...current, sessions: [...current.sessions, makeSessionRow()] }))}
                    >
                      + S-NSSAI
                    </button>
                  )}
                </div>
              ))}
            </div>

            <div className={styles.actionsRow}>
              <Button variant="secondary" type="button" onClick={() => navigate('/subscribers')} disabled={isSaving}>Cancel</Button>
              <Button type="submit" disabled={isSaving}>
                {isSaving ? 'Saving…' : isEditMode ? 'Save' : 'Create'}
              </Button>
            </div>
          </form>
        </div>
      </main>
    </div>
  )
}
