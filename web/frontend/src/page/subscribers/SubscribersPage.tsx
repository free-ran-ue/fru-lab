import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import Sidebar from '../../components/sidebar/Sidebar'
import Button from '../../components/button/button'
import Modal from '../../components/modal/modal'
import NotificationContainer from '../../components/notifications/NotificationContainer'
import { useNotifications } from '../../hooks/useNotifications'
import { webconsoleApi, extractWebconsoleErrorMessage } from '../../webconsoleApiClient'
import type { Subscriber } from '../../webconsoleApi'
import { MAX_BULK_SUBSCRIBERS } from './subscriberForm'
import styles from './webconsole-style.module.css'

export default function SubscribersPage() {
  const navigate = useNavigate()
  const [subscribers, setSubscribers] = useState<Subscriber[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [search, setSearch] = useState('')
  const [deletingUeId, setDeletingUeId] = useState<string | null>(null)
  const [confirmTarget, setConfirmTarget] = useState<Subscriber | null>(null)
  const [isAskingCount, setIsAskingCount] = useState(false)
  const [ueCount, setUeCount] = useState('1')
  const parsedCount = Number(ueCount)
  const isCountValid = Number.isInteger(parsedCount) && parsedCount >= 1 && parsedCount <= MAX_BULK_SUBSCRIBERS

  // Add asks how many UEs first; the form then creates that many,
  // IMSI +1 each, with every other field the same.
  function startAdd() {
    if (!isCountValid) return
    setIsAskingCount(false)
    navigate(`/subscribers/new?count=${parsedCount}`)
  }

  const { errors, successes, addError, addSuccess, removeNotification } = useNotifications()

  const refresh = useCallback(async () => {
    setIsLoading(true)
    setLoadError(null)
    try {
      const response = await webconsoleApi.getSubscribers()
      setSubscribers(response.data)
    } catch (error) {
      setLoadError(extractWebconsoleErrorMessage(
        error,
        'Failed to reach the free5GC webconsole. Is free5GC deployed?',
      ))
    } finally {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => {
    refresh()
  }, [refresh])

  const filteredSubscribers = useMemo(() => {
    const query = search.trim().toLowerCase()
    if (!query) return subscribers
    return subscribers.filter((subscriber) =>
      subscriber.ueId.toLowerCase().includes(query) ||
      subscriber.plmnID.toLowerCase().includes(query) ||
      (subscriber.gpsi || '').toLowerCase().includes(query))
  }, [subscribers, search])

  async function handleConfirmDelete() {
    if (!confirmTarget) return
    const subscriber = confirmTarget
    setConfirmTarget(null)
    setDeletingUeId(subscriber.ueId)

    try {
      await webconsoleApi.deleteSubscriberByID(subscriber.ueId, subscriber.plmnID)
      addSuccess(`Deleted ${subscriber.ueId}`)
      await refresh()
    } catch (error) {
      addError(extractWebconsoleErrorMessage(error, 'Failed to delete subscriber'))
    } finally {
      setDeletingUeId(null)
    }
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
              <h2 className={styles.title}>5G Subscribers</h2>
            </div>
          </div>
        </div>

        <div className={styles.body}>
          <section className={styles.card}>
            <input
              className={styles.input}
              style={{ marginBottom: '1.25rem' }}
              placeholder={`Search Subscriber (${filteredSubscribers.length} / ${subscribers.length})`}
              value={search}
              onChange={(event) => setSearch(event.target.value)}
            />

            {isLoading ? (
              <p className={styles.emptyState}>Loading subscribers…</p>
            ) : loadError ? (
              <p className={styles.emptyState}>{loadError}</p>
            ) : filteredSubscribers.length === 0 ? (
              <p className={styles.emptyState}>No Subscription</p>
            ) : (
              <table className={styles.table} style={{ marginBottom: '1.25rem' }}>
                <thead>
                  <tr>
                    <th>PLMN</th>
                    <th>UE ID</th>
                    <th>GPSI</th>
                    <th>Delete</th>
                    <th>View</th>
                    <th>Edit</th>
                  </tr>
                </thead>
                <tbody>
                  {filteredSubscribers.map((subscriber) => (
                    <tr key={`${subscriber.ueId}-${subscriber.plmnID}`}>
                      <td>{subscriber.plmnID}</td>
                      <td>{subscriber.ueId}</td>
                      <td>{subscriber.gpsi || '—'}</td>
                      <td>
                        <Button
                          variant="danger"
                          onClick={() => setConfirmTarget(subscriber)}
                          disabled={deletingUeId === subscriber.ueId}
                        >
                          {deletingUeId === subscriber.ueId ? 'Deleting…' : 'Delete'}
                        </Button>
                      </td>
                      <td>
                        <Button
                          variant="secondary"
                          onClick={() => navigate(`/subscribers/${encodeURIComponent(subscriber.ueId)}/${encodeURIComponent(subscriber.plmnID)}`)}
                        >
                          View
                        </Button>
                      </td>
                      <td>
                        <Button
                          variant="secondary"
                          onClick={() => navigate(`/subscribers/${encodeURIComponent(subscriber.ueId)}/${encodeURIComponent(subscriber.plmnID)}/edit`)}
                        >
                          Edit
                        </Button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}

            <Button onClick={() => { setUeCount('1'); setIsAskingCount(true) }}>+ Add Subscriber</Button>
          </section>
        </div>
      </main>

      <Modal
        isOpen={confirmTarget !== null}
        onClose={() => setConfirmTarget(null)}
        title="Delete subscriber"
        onSubmit={handleConfirmDelete}
      >
        <p>Delete subscriber <strong>{confirmTarget?.ueId}</strong>? This cannot be undone.</p>
      </Modal>

      <Modal
        isOpen={isAskingCount}
        onClose={() => setIsAskingCount(false)}
        title="Add subscribers"
        onSubmit={startAdd}
      >
        <form onSubmit={(event) => { event.preventDefault(); startAdd() }}>
          <div className={styles.field}>
            <label htmlFor="ue-count">How many UEs?</label>
            <input
              id="ue-count"
              className={styles.input}
              type="number"
              min={1}
              max={MAX_BULK_SUBSCRIBERS}
              value={ueCount}
              onChange={(event) => setUeCount(event.target.value)}
              autoFocus
            />
          </div>
          <p style={{ margin: '0.75rem 0 0', fontSize: '0.85rem', color: '#64748b' }}>
            {isCountValid
              ? 'You fill in the first subscriber; the rest get the next IMSIs (+1 each) with every other field the same.'
              : `Enter a whole number from 1 to ${MAX_BULK_SUBSCRIBERS}.`}
          </p>
        </form>
      </Modal>
    </div>
  )
}
