import { useCallback, useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import Sidebar from '../../components/sidebar/Sidebar'
import Button from '../../components/button/button'
import Modal from '../../components/modal/modal'
import NotificationContainer from '../../components/notifications/NotificationContainer'
import { useNotifications } from '../../hooks/useNotifications'
import { api, extractErrorMessage } from '../../apiClient'
import type { TesterStoredProfile } from '../../api'
import Pager from '../../components/pager/Pager'
import { pageOf } from '../../components/pager/paging'
import { lastProfileId } from './testerProfiles'
import styles from './tester.module.css'

// PROFILES_PAGE is how many profiles one page of the list shows.
const PROFILES_PAGE = 20

function engineLabel(engine: string): string {
  return engine === '' ? 'socket' : engine
}

export default function TesterProfilesPage() {
  const navigate = useNavigate()
  const { errors, successes, addError, addSuccess, removeNotification } = useNotifications()
  const [profiles, setProfiles] = useState<TesterStoredProfile[] | null>(null)
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [page, setPage] = useState(1)
  // the profiles waiting for the delete confirmation
  const [toDelete, setToDelete] = useState<TesterStoredProfile[] | null>(null)
  const [isDeleting, setIsDeleting] = useState(false)
  const openId = lastProfileId()

  const refresh = useCallback(() => api.testerProfileList()
    .then((response) => {
      setProfiles(response.data)
      // forget selections of profiles that are gone
      setSelected((cur) => new Set(response.data.filter((p) => cur.has(p.id)).map((p) => p.id)))
    })
    .catch((error) => addError(extractErrorMessage(error, 'Failed to load the saved profiles'))), [addError])

  useEffect(() => { void refresh() }, [refresh])

  const list = profiles ?? []
  const chosen = list.filter((p) => selected.has(p.id))
  const allSelected = list.length > 0 && chosen.length === list.length
  const shown = pageOf(list, page, PROFILES_PAGE)

  function toggle(id: string) {
    setSelected((cur) => {
      const next = new Set(cur)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  async function handleDelete() {
    const targets = toDelete ?? []
    setToDelete(null)
    setIsDeleting(true)
    const results = await Promise.allSettled(targets.map((p) => api.testerProfileDelete(p.id)))
    const failed = targets.filter((_, i) => results[i].status === 'rejected')
    const done = targets.length - failed.length
    if (done > 0) addSuccess(`Deleted ${done} profile${done === 1 ? '' : 's'}`)
    if (failed.length > 0) {
      const first = results.find((r) => r.status === 'rejected') as PromiseRejectedResult
      addError(`${failed.length} could not be deleted, e.g. "${failed[0].profile.name}": ${extractErrorMessage(first.reason, 'Failed to delete')}`)
    }
    setIsDeleting(false)
    await refresh()
  }

  return (
    <div className={styles.layout}>
      <NotificationContainer errors={errors} successes={successes} onClose={removeNotification} />
      <Sidebar />

      <main className={styles.content}>
        <header className={styles.header}>
          <div>
            <h2 className={styles.title}>Throughput Tester · Profiles</h2>
            <p className={styles.subtitle}>Saved test setups. Edit opens one on the Setup page; names are unique.</p>
          </div>
          <div className={styles.headerActions}>
            <Button onClick={() => navigate('/tester?new')}>+ New profile</Button>
          </div>
        </header>

        <section className={styles.card}>
          {chosen.length > 0 && (
            <div className={styles.toolbar}>
              <Button variant="danger" onClick={() => setToDelete(chosen)} disabled={isDeleting}>
                {isDeleting ? 'Deleting…' : `Delete selected (${chosen.length})`}
              </Button>
              <button type="button" className={styles.linkButton} onClick={() => setSelected(new Set())} disabled={isDeleting}>
                Clear selection
              </button>
            </div>
          )}
          {profiles === null && <p className={styles.hint}>Loading…</p>}
          {profiles?.length === 0 && <p className={styles.hint}>No saved profiles yet. Create one with New profile, or save one from the Setup page.</p>}
          {list.length > 0 && (
            <div className={styles.tableWrap}>
              <table className={styles.table}>
                <thead>
                  <tr>
                    <th>
                      <input type="checkbox" aria-label="Select all profiles" checked={allSelected}
                        ref={(el) => { if (el) el.indeterminate = chosen.length > 0 && !allSelected }}
                        onChange={() => setSelected(allSelected ? new Set() : new Set(list.map((p) => p.id)))}
                        disabled={isDeleting} />
                    </th>
                    <th>Name</th>
                    <th>gNBs / UEs</th>
                    <th>UL / DL per UE</th>
                    <th>Packet</th>
                    <th>Engine</th>
                    <th>Updated</th>
                    <th />
                  </tr>
                </thead>
                <tbody>
                  {shown.items.map((p) => (
                    <tr key={p.id}>
                      <td>
                        <input type="checkbox" aria-label={`Select ${p.profile.name}`} checked={selected.has(p.id)}
                          onChange={() => toggle(p.id)} disabled={isDeleting} />
                      </td>
                      <td>
                        {p.profile.name}
                        {p.id === openId && <span className={`${styles.pill} ${styles.pillActive}`} style={{ marginLeft: '0.5rem' }}>open in Setup</span>}
                      </td>
                      <td className={styles.mono}>{p.profile.scale.gnbCount} / {p.profile.scale.ueCount}</td>
                      <td className={styles.mono}>{p.profile.traffic.ulMbps} / {p.profile.traffic.dlMbps} Mbps</td>
                      <td className={styles.mono}>{p.profile.traffic.packetSize} B</td>
                      <td className={styles.mono}>{engineLabel(p.profile.traffic.engine ?? '')}</td>
                      <td>{new Date(p.updatedAt).toLocaleString()}</td>
                      <td>
                        <button type="button" className={styles.linkButton} onClick={() => navigate(`/tester?profile=${encodeURIComponent(p.id)}`)}>Edit</button>
                        <button type="button" className={styles.linkButton} onClick={() => setToDelete([p])} disabled={isDeleting}>Delete</button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
              <Pager page={shown} total={list.length} onChange={setPage} />
            </div>
          )}
        </section>
      </main>

      <Modal
        isOpen={toDelete !== null}
        onClose={() => setToDelete(null)}
        title={toDelete?.length === 1 ? 'Delete profile' : 'Delete profiles'}
        onSubmit={handleDelete}
        submitLabel="Delete"
      >
        <p className={styles.modalText}>
          {toDelete?.length === 1
            ? <>Delete profile <strong>{toDelete[0].profile.name}</strong>?</>
            : <>Delete <strong>{toDelete?.length}</strong> profiles ({toDelete?.map((p) => p.profile.name).slice(0, 3).join(', ')}{(toDelete?.length ?? 0) > 3 && ', …'})?</>}
          {' '}Past runs in History keep their copy. This cannot be undone.
        </p>
      </Modal>
    </div>
  )
}
