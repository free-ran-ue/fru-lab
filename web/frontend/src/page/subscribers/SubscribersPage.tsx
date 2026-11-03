import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import AppLayout from '../../components/layout/AppLayout'
import Modal from '../../components/modal/modal'
import NotificationContainer from '../../components/notifications/NotificationContainer'
import { useNotifications } from '../../hooks/useNotifications'
import { webconsoleApi, extractWebconsoleErrorMessage } from '../../webconsoleApiClient'
import type { Subscriber } from '../../webconsoleApi'
import { MAX_BULK_SUBSCRIBERS } from './subscriberForm'
import { BULK_CONCURRENCY, runLimited } from './bulk'
import { pageOf } from '../../components/pager/paging'
import styles from './webconsole-style.module.css'

const PAGE_SIZES = [10, 50, 100]

// pageButtons is the numbered buttons of the footer pager: the first and
// last page, the current one and its neighbours, null for a gap.
function pageButtons(current: number, pages: number): Array<number | null> {
  const out: Array<number | null> = []
  for (let n = 1; n <= pages; n++) {
    if (n === 1 || n === pages || Math.abs(n - current) <= 1) out.push(n)
    else if (out[out.length - 1] !== null) out.push(null)
  }
  return out
}

export default function SubscribersPage() {
  const navigate = useNavigate()
  const [subscribers, setSubscribers] = useState<Subscriber[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [search, setSearch] = useState('')
  const [deletingUeId, setDeletingUeId] = useState<string | null>(null)
  const [confirmTarget, setConfirmTarget] = useState<Subscriber | null>(null)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(PAGE_SIZES[0])
  const [plmn, setPlmn] = useState('')
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
    const matching = subscribers.filter((subscriber) =>
      (!plmn || subscriber.plmnID === plmn) &&
      (!query ||
        subscriber.ueId.toLowerCase().includes(query) ||
        subscriber.plmnID.toLowerCase().includes(query) ||
        (subscriber.gpsi || '').toLowerCase().includes(query)))
    return [...matching].sort((a, b) => a.ueId.localeCompare(b.ueId) || a.plmnID.localeCompare(b.plmnID))
  }, [subscribers, search, plmn])
  const plmns = useMemo(() => Array.from(new Set(subscribers.map((s) => s.plmnID))).sort(), [subscribers])
  const isFiltered = Boolean(search.trim() || plmn)

  const shown = pageOf(filteredSubscribers, page, pageSize)
  const keyOf = (s: Subscriber) => `${s.ueId}|${s.plmnID}`
  const selectedSubscribers = subscribers.filter((s) => selected.has(keyOf(s)))
  // the header checkbox selects every subscriber matching the search, on
  // every page, not just the page shown
  const allMatchingSelected = filteredSubscribers.length > 0 && filteredSubscribers.every((s) => selected.has(keyOf(s)))
  const someMatchingSelected = filteredSubscribers.some((s) => selected.has(keyOf(s)))
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
    <AppLayout
      crumbs={[{ label: 'Subscribers' }]}
      overlay={<NotificationContainer errors={errors} successes={successes} onClose={removeNotification} />}
    >
      <section className={styles.pageHeader}>
        <div className={styles.titleBlock}>
          <h1 className={styles.title}>Subscribers</h1>
          <p className={styles.subtitle}>Subscriptions in free5GC&apos;s UDR, managed through its WebConsole.</p>
        </div>
        <div className={styles.headerActions}>
          {selectedSubscribers.length > 0 && (
            <>
              <button type="button" className={styles.btn} onClick={() => setSelected(new Set())} disabled={isBulkDeleting}>
                Clear selection
              </button>
              <button type="button" className={`${styles.btn} ${styles.btnDangerText}`} onClick={() => setIsConfirmingBulk(true)} disabled={isBulkDeleting}>
                {isBulkDeleting ? `Deleting ${bulkProgress}/${selectedSubscribers.length}…` : `Delete ${selectedSubscribers.length} selected`}
              </button>
            </>
          )}
          <button type="button" className={styles.btn} onClick={() => { setUeCount('2'); setIsAskingCount(true) }}>Add in bulk</button>
          <button type="button" className={`${styles.btn} ${styles.btnPrimary}`} onClick={() => navigate('/subscribers/new')}>
            <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.8" aria-hidden="true"><path d="M7 2v10M2 7h10" /></svg>
            Add subscriber
          </button>
        </div>
      </section>

      <section className={styles.panel}>
        <div className={styles.toolbar}>
          <input
            className={`${styles.in} ${styles.search}`}
            aria-label="Search subscribers"
            placeholder="Search SUPI or GPSI"
            value={search}
            onChange={(event) => { setSearch(event.target.value); setPage(1) }}
          />
          <select
            className={styles.in}
            aria-label="PLMN"
            value={plmn}
            onChange={(event) => { setPlmn(event.target.value); setPage(1) }}
          >
            <option value="">All PLMNs</option>
            {plmns.map((id) => <option key={id} value={id}>{id}</option>)}
          </select>
          {!isLoading && !loadError && (
            <span className={`num ${styles.range}`}>
              {shown.from}–{shown.to} of {filteredSubscribers.length.toLocaleString()}
              {filteredSubscribers.length !== subscribers.length && <> (of {subscribers.length.toLocaleString()})</>}
            </span>
          )}
        </div>

        {isLoading ? (
          <p className={styles.emptyState}>Loading subscribers…</p>
        ) : loadError ? (
          <p className={styles.emptyState}>{loadError}</p>
        ) : filteredSubscribers.length === 0 ? (
          <p className={styles.emptyState}>No Subscription</p>
        ) : (
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th className={`${styles.th} ${styles.checkCell}`}>
                    <input
                      type="checkbox"
                      aria-label={isFiltered ? 'Select all matching subscribers' : 'Select all subscribers'}
                      title={`Select all ${filteredSubscribers.length}${isFiltered ? ' matching' : ''}, on every page`}
                      checked={allMatchingSelected}
                      ref={(el) => { if (el) el.indeterminate = someMatchingSelected && !allMatchingSelected }}
                      onChange={() => setMany(filteredSubscribers, !allMatchingSelected)}
                      disabled={isBulkDeleting}
                    />
                  </th>
                  <th className={styles.th}>UE ID</th>
                  <th className={styles.th}>PLMN</th>
                  <th className={styles.th}>GPSI</th>
                  <th className={`${styles.th} ${styles.right}`}>Actions</th>
                </tr>
              </thead>
              <tbody>
                {shown.items.map((subscriber) => (
                  <tr key={`${subscriber.ueId}-${subscriber.plmnID}`} className={styles.row}>
                    <td className={styles.td}>
                      <input
                        type="checkbox"
                        aria-label={`Select ${subscriber.ueId}`}
                        checked={selected.has(keyOf(subscriber))}
                        onChange={() => toggle(subscriber)}
                        disabled={isBulkDeleting}
                      />
                    </td>
                    <td className={`${styles.td} num`}>{subscriber.ueId}</td>
                    <td className={`${styles.td} num`}>{subscriber.plmnID}</td>
                    <td className={`${styles.td} num ${styles.mutedCell}`}>{subscriber.gpsi || '—'}</td>
                    <td className={`${styles.td} ${styles.right}`}>
                      <button
                        type="button"
                        className={styles.link}
                        onClick={() => navigate(`/subscribers/${encodeURIComponent(subscriber.ueId)}/${encodeURIComponent(subscriber.plmnID)}`)}
                      >
                        View
                      </button>
                      <span className={styles.sep} aria-hidden="true">|</span>
                      <button
                        type="button"
                        className={styles.link}
                        onClick={() => navigate(`/subscribers/${encodeURIComponent(subscriber.ueId)}/${encodeURIComponent(subscriber.plmnID)}/edit`)}
                      >
                        Edit
                      </button>
                      <span className={styles.sep} aria-hidden="true">|</span>
                      <button
                        type="button"
                        className={styles.linkDanger}
                        onClick={() => setConfirmTarget(subscriber)}
                        disabled={deletingUeId === subscriber.ueId}
                      >
                        {deletingUeId === subscriber.ueId ? 'Deleting…' : 'Delete'}
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {!isLoading && !loadError && filteredSubscribers.length > 0 && (
          <div className={styles.footer}>
            <label htmlFor="rows-per-page">Rows per page</label>
            <select
              id="rows-per-page"
              className={`${styles.in} ${styles.footerSelect}`}
              value={pageSize}
              onChange={(event) => { setPageSize(Number(event.target.value)); setPage(1) }}
            >
              {PAGE_SIZES.map((size) => <option key={size} value={size}>{size}</option>)}
            </select>
            <nav className={styles.pages} aria-label="Pages">
              <button
                type="button"
                className={`${styles.btn} ${styles.btnSm} ${styles.btnMuted}`}
                onClick={() => setPage(shown.page - 1)}
                disabled={shown.page === 1}
              >
                Previous
              </button>
              {pageButtons(shown.page, shown.pages).map((n, i) => n === null
                ? <span key={`gap-${i}`} className={styles.ellipsis} aria-hidden="true">…</span>
                : (
                  <button
                    key={n}
                    type="button"
                    className={`${styles.btn} ${styles.btnSm} num ${n === shown.page ? styles.btnCurrent : ''}`}
                    aria-current={n === shown.page ? 'page' : undefined}
                    onClick={() => setPage(n)}
                  >
                    {n}
                  </button>
                ))}
              <button
                type="button"
                className={`${styles.btn} ${styles.btnSm}`}
                onClick={() => setPage(shown.page + 1)}
                disabled={shown.page === shown.pages}
              >
                Next
              </button>
            </nav>
          </div>
        )}
      </section>

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
        <form className={styles.countForm} onSubmit={(event) => { event.preventDefault(); startAdd() }}>
          <div className={styles.fld}>
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
          <p className={styles.hint}>
            {isCountValid
              ? 'You fill in the first subscriber; the rest get the next IMSIs (+1 each) with every other field the same.'
              : `Enter a whole number from 1 to ${MAX_BULK_SUBSCRIBERS}.`}
          </p>
        </form>
      </Modal>
    </AppLayout>
  )
}
