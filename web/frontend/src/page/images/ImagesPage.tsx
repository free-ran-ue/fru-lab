import { useCallback, useEffect, useState } from 'react'
import AppLayout from '../../components/layout/AppLayout'
import Modal from '../../components/modal/modal'
import NotificationContainer from '../../components/notifications/NotificationContainer'
import { useNotifications } from '../../hooks/useNotifications'
import { api, extractErrorMessage } from '../../apiClient'
import type { ImageInfo } from '../../api'
import styles from './images-page.module.css'

const GROUP_LABELS: Record<string, string> = {
  free5gc: 'free5GC core',
  'free-ran-ue': 'free-ran-ue',
}

// formatStamp shows a date as "YYYY-MM-DD HH:MM:SS" in local time.
function formatStamp(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  const p = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
}

function groupLabel(group: string): string {
  return GROUP_LABELS[group] ?? group
}

function formatSize(bytes?: number): string {
  if (!bytes) return '—'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let value = bytes
  let unitIndex = 0
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024
    unitIndex += 1
  }
  return `${value.toFixed(unitIndex === 0 ? 0 : unitIndex >= 3 ? 2 : 1)} ${units[unitIndex]}`
}

// Which action (if any) is in flight for a given image, keyed by its route
// key - lets the row disable just its own buttons instead of the whole page
// while a pull/clear is running.
type PendingAction = 'pull' | 'clear'

export default function ImagesPage() {
  const { errors, successes, addError, addSuccess, removeNotification } = useNotifications()
  const [images, setImages] = useState<ImageInfo[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [pending, setPending] = useState<Record<string, PendingAction>>({})
  const [confirmClearTarget, setConfirmClearTarget] = useState<ImageInfo | null>(null)
  const [confirmClearAll, setConfirmClearAll] = useState(false)

  const refresh = useCallback(async () => {
    try {
      const response = await api.imageList()
      setImages(response.data.images ?? [])
    } catch (error) {
      addError(extractErrorMessage(error, 'Failed to load images'))
    } finally {
      setIsLoading(false)
    }
  }, [addError])

  useEffect(() => {
    refresh()
  }, [refresh])

  async function handlePull(image: ImageInfo) {
    setPending((current) => ({ ...current, [image.key]: 'pull' }))
    try {
      await api.imagePull(image.key)
      addSuccess(`Pulled ${image.name}`)
      await refresh()
    } catch (error) {
      addError(extractErrorMessage(error, `Failed to pull ${image.name}`))
    } finally {
      setPending((current) => {
        const next = { ...current }
        delete next[image.key]
        return next
      })
    }
  }

  // pullMissing pulls every image not on this host, one after another
  // (each is a full docker pull), with the same per-image pull as the rows.
  async function pullMissing() {
    const missing = images.filter((image) => !image.present && !pending[image.key])
    if (missing.length === 0) return
    setPending((current) => {
      const next = { ...current }
      for (const image of missing) next[image.key] = 'pull'
      return next
    })
    const failed: string[] = []
    for (const image of missing) {
      try {
        await api.imagePull(image.key)
      } catch (error) {
        failed.push(`${image.name}: ${extractErrorMessage(error, 'pull failed')}`)
      }
      setPending((current) => {
        const next = { ...current }
        delete next[image.key]
        return next
      })
    }
    if (failed.length < missing.length) addSuccess(`Pulled ${missing.length - failed.length} of ${missing.length} missing image${missing.length === 1 ? '' : 's'}`)
    if (failed.length > 0) addError(`Failed to pull ${failed.length}, e.g. ${failed[0]}`)
    await refresh()
  }

  async function handleConfirmClear() {
    if (!confirmClearTarget) return
    const image = confirmClearTarget
    setConfirmClearTarget(null)

    setPending((current) => ({ ...current, [image.key]: 'clear' }))
    try {
      await api.imageRemove(image.key)
      addSuccess(`Cleared ${image.name}`)
      await refresh()
    } catch (error) {
      addError(extractErrorMessage(error, `Failed to clear ${image.name}`))
    } finally {
      setPending((current) => {
        const next = { ...current }
        delete next[image.key]
        return next
      })
    }
  }

  async function handleConfirmClearAll() {
    setConfirmClearAll(false)

    const presentImages = images.filter((image) => image.present)
    if (presentImages.length === 0) return

    setPending((current) => {
      const next = { ...current }
      for (const image of presentImages) next[image.key] = 'clear'
      return next
    })
    try {
      const results = await Promise.allSettled(presentImages.map((image) => api.imageRemove(image.key)))
      const failedCount = results.filter((result) => result.status === 'rejected').length
      if (failedCount > 0) {
        addError(`Failed to clear ${failedCount} of ${presentImages.length} image(s)`)
      } else {
        addSuccess('Cleared all images')
      }
    } finally {
      await refresh()
      setPending((current) => {
        const next = { ...current }
        for (const image of presentImages) delete next[image.key]
        return next
      })
    }
  }

  const groups = Array.from(new Set(images.map((image) => image.group)))
  const presentCount = images.filter((image) => image.present).length
  const missingCount = images.length - presentCount
  const totalSize = images.reduce((sum, image) => sum + (image.present ? image.size ?? 0 : 0), 0)
  const isAnyPending = Object.keys(pending).length > 0

  return (
    <AppLayout
      crumbs={[{ label: 'Images' }]}
      overlay={<NotificationContainer errors={errors} successes={successes} onClose={removeNotification} />}
    >
      <section className={styles.header}>
        <h1 className={styles.title}>Images</h1>
        <span className={styles.subtitle}>
          Docker images this app deploys, pinned to each template&apos;s tag. Clear one to force a fresh pull on the next deploy.
          {!isLoading && <> · <span className="num">{presentCount} of {images.length}</span> on this host · <span className="num">{formatSize(totalSize)}</span></>}
        </span>
        <div className={styles.headerActions}>
          <button
            type="button"
            className={`${styles.btn} ${styles.btnDanger}`}
            onClick={() => setConfirmClearAll(true)}
            disabled={presentCount === 0 || isAnyPending}
          >
            Clear all
          </button>
          <button type="button" className={styles.btn} onClick={refresh} disabled={isLoading}>
            {isLoading ? 'Refreshing…' : 'Refresh'}
          </button>
          <button
            type="button"
            className={`${styles.btn} ${styles.btnPrimary}`}
            onClick={pullMissing}
            disabled={missingCount === 0 || isAnyPending}
          >
            Pull missing ({missingCount})
          </button>
        </div>
      </section>

      {isLoading ? (
        <section className={styles.panel}><p className={styles.emptyState}>Loading images…</p></section>
      ) : (
        groups.map((group) => {
          const groupImages = images.filter((image) => image.group === group)
          return (
            <section key={group} className={styles.panel}>
              <div className={styles.ph}>
                <h2>{groupLabel(group)}</h2>
                <span className={`num ${styles.aux}`}>
                  {groupImages.filter((image) => image.present).length} of {groupImages.length} pulled
                </span>
              </div>
              <div className={styles.tableWrap}>
                <table className={styles.tbl}>
                  <thead>
                    <tr>
                      <th>Image</th>
                      <th>Status</th>
                      <th className={styles.r}>Size</th>
                      <th>Created</th>
                      <th className={styles.right}>Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {groupImages.map((image) => {
                      const action = pending[image.key]
                      return (
                        <tr key={image.key}>
                          <td>
                            <div className={styles.imgName}>
                              <b>{image.name}</b>
                              <span>{image.image}</span>
                            </div>
                          </td>
                          <td>
                            <span className={`${styles.tag} ${image.present ? styles.tagOk : styles.tagIdle}`}>
                              <i aria-hidden="true" />
                              {image.present ? 'Pulled' : 'Not pulled'}
                            </span>
                          </td>
                          <td className={styles.r}>{image.present ? formatSize(image.size) : '—'}</td>
                          <td className={styles.m}>
                            {image.present && image.createdAt ? formatStamp(image.createdAt) : '—'}
                          </td>
                          <td className={styles.right}>
                            <span className={styles.rowActions}>
                              <button
                                type="button"
                                className={`${styles.btn} ${styles.btnSm}`}
                                onClick={() => handlePull(image)}
                                disabled={Boolean(action)}
                              >
                                {action === 'pull' ? 'Pulling…' : image.present ? 'Update' : 'Pull'}
                              </button>
                              <button
                                type="button"
                                className={`${styles.btn} ${styles.btnSm} ${styles.btnDanger}`}
                                onClick={() => setConfirmClearTarget(image)}
                                disabled={!image.present || Boolean(action)}
                              >
                                {action === 'clear' ? 'Clearing…' : 'Clear'}
                              </button>
                            </span>
                          </td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
              </div>
            </section>
          )
        })
      )}

      <Modal
        isOpen={confirmClearTarget !== null}
        onClose={() => setConfirmClearTarget(null)}
        title="Clear image"
        onSubmit={handleConfirmClear}
      >
        <p>Clear <strong>{confirmClearTarget?.name}</strong> ({confirmClearTarget?.image}) from the local image cache? The next deploy will pull it fresh.</p>
      </Modal>

      <Modal
        isOpen={confirmClearAll}
        onClose={() => setConfirmClearAll(false)}
        title="Clear all images"
        onSubmit={handleConfirmClearAll}
      >
        <p>Clear all {presentCount} pulled image{presentCount === 1 ? '' : 's'} from the local cache? The next deploy will pull each one fresh.</p>
      </Modal>
    </AppLayout>
  )
}
