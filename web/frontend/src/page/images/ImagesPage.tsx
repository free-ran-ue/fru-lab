import { useCallback, useEffect, useState } from 'react'
import Sidebar from '../../components/sidebar/Sidebar'
import Button from '../../components/button/button'
import Modal from '../../components/modal/modal'
import NotificationContainer from '../../components/notifications/NotificationContainer'
import { useNotifications } from '../../hooks/useNotifications'
import { api, extractErrorMessage } from '../../apiClient'
import type { ImageInfo } from '../../api'
import styles from './images-page.module.css'

const GROUP_LABELS: Record<string, string> = {
  free5gc: 'free5GC Core',
  'free-ran-ue': 'free-ran-ue',
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
  return `${value.toFixed(unitIndex === 0 ? 0 : 1)} ${units[unitIndex]}`
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
  const isAnyPending = Object.keys(pending).length > 0

  return (
    <div className={styles.layout}>
      <NotificationContainer errors={errors} successes={successes} onClose={removeNotification} />

      <Sidebar />

      <main className={styles.content}>
        <header className={styles.header}>
          <div>
            <h2 className={styles.title}>Images</h2>
            <p className={styles.subtitle}>Docker images this app deploys, all pinned to their template's tag. Clear one to force a fresh pull on the next deploy.</p>
          </div>
          <div className={styles.headerActions}>
            {presentCount > 0 && (
              <button className={styles.clearAllButton} onClick={() => setConfirmClearAll(true)} disabled={isAnyPending}>
                {isAnyPending ? 'Clearing…' : 'Clear All'}
              </button>
            )}
            <Button variant="secondary" onClick={refresh} disabled={isLoading}>
              {isLoading ? 'Refreshing…' : 'Refresh'}
            </Button>
          </div>
        </header>

        <div className={styles.body}>
          {isLoading ? (
            <p className={styles.emptyState}>Loading images…</p>
          ) : (
            groups.map((group) => (
              <section key={group} className={styles.group}>
                <h3 className={styles.groupTitle}>{groupLabel(group)}</h3>
                <div className={styles.imageList}>
                  {images.filter((image) => image.group === group).map((image) => {
                    const action = pending[image.key]
                    return (
                      <div key={image.key} className={styles.imageRow}>
                        <div className={styles.imageInfo}>
                          <span className={styles.imageName}>{image.name}</span>
                          <span className={styles.imageRef}>{image.image}</span>
                        </div>

                        <div className={styles.imageMeta}>
                          <span className={`${styles.statusPill} ${image.present ? styles.statusPillPresent : styles.statusPillAbsent}`}>
                            <span className={styles.statusDot} />
                            {image.present ? 'Pulled' : 'Not pulled'}
                          </span>
                          {image.present && <span className={styles.metaText}>{formatSize(image.size)}</span>}
                          {image.present && image.createdAt && (
                            <span className={styles.metaText}>{new Date(image.createdAt).toLocaleString()}</span>
                          )}
                        </div>

                        <div className={styles.imageActions}>
                          <button
                            className={styles.linkButton}
                            onClick={() => handlePull(image)}
                            disabled={Boolean(action)}
                          >
                            {action === 'pull' ? 'Pulling…' : 'Pull'}
                          </button>
                          <button
                            className={styles.dangerLinkButton}
                            onClick={() => setConfirmClearTarget(image)}
                            disabled={!image.present || Boolean(action)}
                          >
                            {action === 'clear' ? 'Clearing…' : 'Clear'}
                          </button>
                        </div>
                      </div>
                    )
                  })}
                </div>
              </section>
            ))
          )}
        </div>
      </main>

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
    </div>
  )
}
