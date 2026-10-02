import Button from '../button/button'
import type { Page } from './paging'
import styles from './pager.module.css'

interface PagerProps {
  page: Page<unknown>
  total: number
  onChange: (page: number) => void
}

// Pager is the "1–10 of 57 · « ‹ Page 1 / 6 › »" row under a paged
// table; it renders nothing when everything fits on one page.
export default function Pager({ page, total, onChange }: PagerProps) {
  if (page.pages <= 1) return null
  return (
    <div className={styles.pager}>
      <span className={styles.range}>{page.from}–{page.to} of {total}</span>
      <div className={styles.controls}>
        <Button variant="secondary" onClick={() => onChange(1)} disabled={page.page === 1}>«</Button>
        <Button variant="secondary" onClick={() => onChange(page.page - 1)} disabled={page.page === 1}>‹ Prev</Button>
        <span className={styles.current}>Page {page.page} / {page.pages}</span>
        <Button variant="secondary" onClick={() => onChange(page.page + 1)} disabled={page.page === page.pages}>Next ›</Button>
        <Button variant="secondary" onClick={() => onChange(page.pages)} disabled={page.page === page.pages}>»</Button>
      </div>
    </div>
  )
}
