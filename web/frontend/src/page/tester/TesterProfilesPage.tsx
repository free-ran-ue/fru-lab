import { useCallback, useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import AppLayout from '../../components/layout/AppLayout'
import Modal from '../../components/modal/modal'
import NotificationContainer from '../../components/notifications/NotificationContainer'
import { useNotifications } from '../../hooks/useNotifications'
import { api, extractErrorMessage } from '../../apiClient'
import type { TesterStoredProfile } from '../../api'
import Pager from '../../components/pager/Pager'
import { pageOf } from '../../components/pager/paging'
import { lastProfileId } from './testerProfiles'
import { DEFAULT_TESTER_ID, normalizeProfile } from './testerDefaults'
import { testerLabel, useTesters, type TesterOption } from './useTesters'
import styles from './profiles.module.css'

// PROFILES_PAGE is how many profiles one page of the list shows.
const PROFILES_PAGE = 20

// TESTER_COLORS tell testers apart: the first one is the accent, the
// second violet, as in the prototype; more cycle through the brand tones.
const TESTER_COLORS = ['var(--accent)', 'var(--violet)', 'var(--brand-1)', 'var(--brand-3)', 'var(--ok)', 'var(--warn)']

// testerColor is a tester's square colour, by its place in the list.
function testerColor(testers: TesterOption[] | null, id: string): string {
  const i = testers?.findIndex((t) => t.id === id) ?? -1
  return i < 0 ? 'var(--border-strong)' : TESTER_COLORS[i % TESTER_COLORS.length]
}

// engineLabel is the data plane engine a profile asks for; "off" when no
// UE carries traffic.
function engineLabel(saved: TesterStoredProfile['profile']): string {
  const p = normalizeProfile(saved)
  if (!p.scale.throughputUeCount || (!p.traffic.ulMbps && !p.traffic.dlMbps)) return 'off'
  const engine = p.traffic.engine ?? ''
  if (engine === 'afxdp') return 'AF_XDP'
  return engine === '' ? 'socket' : engine
}

// packetSizes is a profile's uplink / downlink packet size.
function packetSizes(saved: TesterStoredProfile['profile']): string {
  const t = normalizeProfile(saved).traffic
  return `${t.ulPacketSize} / ${t.dlPacketSize} B`
}

// shortTime is "MM-DD HH:mm" in local time.
function shortTime(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  const two = (n: number) => String(n).padStart(2, '0')
  return `${two(d.getMonth() + 1)}-${two(d.getDate())} ${two(d.getHours())}:${two(d.getMinutes())}`
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
  const { testers } = useTesters()

  const refresh = useCallback(() => api.testerProfileList()
    .then((response) => {
      setProfiles(response.data)
      // forget selections of profiles that are gone
      setSelected((cur) => new Set(response.data.filter((p) => cur.has(p.id)).map((p) => p.id)))
    })
    .catch((error) => addError(extractErrorMessage(error, 'Failed to load the saved profiles'))), [addError])

  useEffect(() => { void refresh() }, [refresh])

  const [query, setQuery] = useState('')
  const [testerFilter, setTesterFilter] = useState('')

  const list = profiles ?? []
  const q = query.trim().toLowerCase()
  const filtered = list.filter((p) => (q === '' || p.profile.name.toLowerCase().includes(q))
    && (testerFilter === '' || (p.profile.testerId || DEFAULT_TESTER_ID) === testerFilter))
  const chosen = list.filter((p) => selected.has(p.id))
  const allSelected = filtered.length > 0 && filtered.every((p) => selected.has(p.id))
  const shown = pageOf(filtered, page, PROFILES_PAGE)

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
    <AppLayout crumbs={[{ label: 'Profiles' }]}
      overlay={<NotificationContainer errors={errors} successes={successes} onClose={removeNotification} />}>
      <section className={styles.head}>
        <div className={styles.headText}>
          <h1 className={styles.title}>Profiles</h1>
          <p className={styles.subtitle}>Saved test plans. Open one in Setup to run or change it.</p>
        </div>
        <div className={styles.headActions}>
          <button type="button" className={`${styles.btn} ${styles.btnPrimary}`} onClick={() => navigate('/tester?new')}>
            <svg viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.8" aria-hidden="true"><path d="M7 2v10M2 7h10" /></svg>
            New profile
          </button>
        </div>
      </section>

      <section className={styles.panel}>
        <div className={styles.toolbar}>
          <input className={`${styles.in} ${styles.search}`} type="search" aria-label="Search profiles" placeholder="Search by name"
            value={query} onChange={(e) => { setQuery(e.target.value); setPage(1) }} />
          <select className={styles.in} aria-label="Tester" value={testerFilter}
            onChange={(e) => { setTesterFilter(e.target.value); setPage(1) }}>
            <option value="">All testers</option>
            {(testers ?? []).map((t) => <option key={t.id} value={t.id}>{t.name}</option>)}
          </select>
          {chosen.length > 0 && (
            <span className={styles.selection}>
              <button type="button" className={`${styles.btn} ${styles.btnDanger}`} onClick={() => setToDelete(chosen)} disabled={isDeleting}>
                {isDeleting ? 'Deleting…' : `Delete selected (${chosen.length})`}
              </button>
              <button type="button" className={styles.link} onClick={() => setSelected(new Set())} disabled={isDeleting}>
                Clear selection
              </button>
            </span>
          )}
          <span className={`num ${styles.count}`}>
            {filtered.length === list.length
              ? `${list.length} profile${list.length === 1 ? '' : 's'}`
              : `${filtered.length} of ${list.length} profiles`}
          </span>
        </div>
        {profiles === null && <p className={styles.empty}>Loading…</p>}
        {profiles?.length === 0 && <p className={styles.empty}>No saved profiles yet. Create one with New profile, or save one from the Setup page.</p>}
        {list.length > 0 && filtered.length === 0 && <p className={styles.empty}>No profile matches the search.</p>}
        {filtered.length > 0 && (
          <>
            <div className={styles.tableWrap}>
              <table className={styles.table}>
                <thead>
                  <tr>
                    <th className={`${styles.th} ${styles.checkCol}`}>
                      <input type="checkbox" aria-label="Select all profiles" checked={allSelected}
                        ref={(el) => { if (el) el.indeterminate = chosen.length > 0 && !allSelected }}
                        onChange={() => setSelected(allSelected ? new Set() : new Set(filtered.map((p) => p.id)))}
                        disabled={isDeleting} />
                    </th>
                    <th className={styles.th}>Name</th>
                    <th className={styles.th}>Tester</th>
                    <th className={styles.th}>Engine</th>
                    <th className={`${styles.th} ${styles.r}`}>gNBs / UEs</th>
                    <th className={`${styles.th} ${styles.r}`}>UL / DL per UE</th>
                    <th className={`${styles.th} ${styles.r}`}>Packet UL / DL</th>
                    <th className={styles.th}>Updated</th>
                    <th className={`${styles.th} ${styles.r}`}>Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {shown.items.map((p) => {
                    const tid = p.profile.testerId || DEFAULT_TESTER_ID
                    const known = testers === null || testers.some((t) => t.id === tid)
                    const open = () => navigate(`/tester?profile=${encodeURIComponent(p.id)}`)
                    return (
                      <tr key={p.id} className={styles.row}>
                        <td className={styles.td}>
                          <input type="checkbox" aria-label={`Select ${p.profile.name}`} checked={selected.has(p.id)}
                            onChange={() => toggle(p.id)} disabled={isDeleting} />
                        </td>
                        <td className={styles.td}>
                          <button type="button" className={styles.nameLink} onClick={open}>{p.profile.name}</button>
                          {p.id === openId && <span className={`${styles.tag} ${styles.tagActive} ${styles.tagAfter}`}>open in Setup</span>}
                        </td>
                        <td className={styles.td}>
                          <span className={styles.tester}>
                            <span className={styles.testerSquare} style={{ background: testerColor(testers, tid) }} aria-hidden="true" />
                            {testerLabel(testers, p.profile.testerId)}
                          </span>
                          {!known && <span className={`${styles.tag} ${styles.tagBad} ${styles.tagAfter}`}>choose another</span>}
                        </td>
                        <td className={styles.td}><span className={styles.tag}>{engineLabel(p.profile)}</span></td>
                        <td className={`${styles.td} ${styles.r} num`}>{p.profile.scale.gnbCount.toLocaleString()} / {p.profile.scale.ueCount.toLocaleString()}</td>
                        <td className={`${styles.td} ${styles.r} num`}>{p.profile.traffic.ulMbps} / {p.profile.traffic.dlMbps} Mbps</td>
                        <td className={`${styles.td} ${styles.r} num`}>{packetSizes(p.profile)}</td>
                        <td className={`${styles.td} ${styles.dim} num`} title={new Date(p.updatedAt).toLocaleString()}>{shortTime(p.updatedAt)}</td>
                        <td className={`${styles.td} ${styles.r}`}>
                          <button type="button" className={styles.link} onClick={open}>Open in Setup</button>
                          <span className={styles.sep} aria-hidden="true">|</span>
                          <button type="button" className={`${styles.link} ${styles.linkDanger}`} onClick={() => setToDelete([p])} disabled={isDeleting}>Delete</button>
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
            {shown.pages > 1 && (
              <div className={styles.pager}>
                <Pager page={shown} total={filtered.length} onChange={setPage} />
              </div>
            )}
          </>
        )}
      </section>

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
    </AppLayout>
  )
}
