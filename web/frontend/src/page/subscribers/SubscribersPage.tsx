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
import { BULK_CONCURRENCY, runLimited } from './bulk'
import Pager from '../../components/pager/Pager'
import { pageOf } from '../../components/pager/paging'

const PAGE_SIZE = 10
import styles from './webconsole-style.module.css'

export default function SubscribersPage() {
  const navigate = useNavigate()
  const [subscribers, setSubscribers] = useState<Subscriber[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [search, setSearch] = useState('')
  const [deletingUeId, setDeletingUeId] = useState<string | null>(null)
  const [confirmTarget, setConfirmTarget] = useState<Subscriber | null>(null)
  const [page, setPage] = useState(1)
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [isConfirmingBulk, setIsConfirmingBulk] = useState(false)
  const [bulkProgress, setBulkProgress] = useState<number | null>(null) // deletions done, while running
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

  // sorted by IMSI: the webconsole lists them in no fixed order, which
  // would shuffle the pages
  const filteredSubscribers = useMemo(() => {
    const query = search.trim().toLowerCase()
    const matching = !query ? subscribers : subscribers.filter((subscriber) =>
      subscriber.ueId.toLowerCase().includes(query) ||
      subscriber.plmnID.toLowerCase().includes(query) ||
      (subscriber.gpsi || '').toLowerCase().includes(query))
    return [...matching].sort((a, b) => a.ueId.localeCompare(b.ueId) || a.plmnID.localeCompare(b.plmnID))
  }, [subscribers, search])

  const shown = pageOf(filteredSubscribers, page, PAGE_SIZE)
  const keyOf = (s: Subscriber) => `${s.ueId}|${s.plmnID}`
  const selectedSubscribers = subscribers.filter((s) => selected.has(keyOf(s)))
  const pageAllSelected = shown.items.length > 0 && shown.items.every((s) => selected.has(keyOf(s)))
  const allMatchingSelected = filteredSubscribers.length > 0 && filteredSubscribers.every((s) => selected.has(keyOf(s)))
  const isBulkDeleting = bulkProgress !== null

  function toggle(subscriber: Subscriber) {
    setSelected((current) => {
      const next = new Set(current)
      if (next.has(keyOf(subscriber))) next.delete(keyOf(subscriber))
      else next.add(keyOf(subscriber))
      return next
    })
  }

  function setMany(list: Subscriber[], on: boolean) {
    setSelected((current) => {
      const next = new Set(current)
      list.forEach((s) => (on ? next.add(keyOf(s)) : next.delete(keyOf(s))))
      return next
    })
  }

  async function handleBulkDelete() {
    const targets = selectedSubscribers
    setIsConfirmingBulk(false)
    setBulkProgress(0)
    const failed = await runLimited(targets, BULK_CONCURRENCY,
      (s) => webconsoleApi.deleteSubscriberByID(s.ueId, s.plmnID), setBulkProgress)
    // keep the ones that failed selected, so they can be retried
    setSelected(new Set(failed.map((f) => keyOf(f.item))))
    setBulkProgress(null)
    if (targets.length > failed.length) addSuccess(`Deleted ${targets.length - failed.length} subscriber${targets.length - failed.length === 1 ? '' : 's'}`)
    if (failed.length > 0) {
      const first = failed[0]
      addError(`${failed.length} could not be deleted (still selected), e.g. ${first.item.ueId}: ${extractWebconsoleErrorMessage(first.error, 'Failed to delete')}`)
    }
    await refresh()
  }

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
              onChange={(event) => { setSearch(event.target.value); setPage(1) }}
            />

            {selectedSubscribers.length > 0 && (
              <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', flexWrap: 'wrap', marginBottom: '1rem' }}>
                <Button variant="danger" onClick={() => setIsConfirmingBulk(true)} disabled={isBulkDeleting}>
                  {isBulkDeleting ? `Deleting ${bulkProgress}/${selectedSubscribers.length}…` : `Delete selected (${selectedSubscribers.length})`}
                </Button>
                {pageAllSelected && !allMatchingSelected && (
                  <button type="button" className={styles.btnAdd} onClick={() => setMany(filteredSubscribers, true)} disabled={isBulkDeleting}>
                    Select all {filteredSubscribers.length}{search.trim() ? ' matching' : ''}
                  </button>
                )}
                <button type="button" className={styles.btnAdd} onClick={() => setSelected(new Set())} disabled={isBulkDeleting}>
                  Clear selection
                </button>
              </div>
            )}

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
                    <th>
                      <input
                        type="checkbox"
                        aria-label="Select this page"
                        checked={pageAllSelected}
                        onChange={() => setMany(shown.items, !pageAllSelected)}
                        disabled={isBulkDeleting}
                      />
                    </th>
                    <th>PLMN</th>
                    <th>UE ID</th>
                    <th>GPSI</th>
                    <th>Delete</th>
                    <th>View</th>
                    <th>Edit</th>
                  </tr>
                </thead>
                <tbody>
                  {shown.items.map((subscriber) => (
                    <tr key={`${subscriber.ueId}-${subscriber.plmnID}`}>
                      <td>
                        <input
                          type="checkbox"
                          aria-label={`Select ${subscriber.ueId}`}
                          checked={selected.has(keyOf(subscriber))}
                          onChange={() => toggle(subscriber)}
                          disabled={isBulkDeleting}
                        />
                      </td>
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

            {!isLoading && !loadError && (
              <Pager page={shown} total={filteredSubscribers.length} onChange={setPage} />
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
        isOpen={isConfirmingBulk}
        onClose={() => setIsConfirmingBulk(false)}
        title="Delete subscribers"
        onSubmit={handleBulkDelete}
      >
        <p>
          Delete <strong>{selectedSubscribers.length}</strong> subscriber{selectedSubscribers.length === 1 ? '' : 's'}
          {selectedSubscribers.length > 0 && <> ({selectedSubscribers[0].ueId}{selectedSubscribers.length > 1 && <> … {selectedSubscribers[selectedSubscribers.length - 1].ueId}</>})</>}?
          This cannot be undone.
        </p>
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
