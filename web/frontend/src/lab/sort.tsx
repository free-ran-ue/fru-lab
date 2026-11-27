import { useMemo, useState, type ReactNode } from 'react'
import styles from './sort.module.css'

export type SortValue = string | number | null | undefined
export type SortDir = 'asc' | 'desc'
export interface SortState { key: string; dir: SortDir }

const BLANK = new Set(['', '—', '-'])

// compareValues orders numbers by value and text naturally ("ue-2" before
// "ue-10"); blanks always go last, whichever way it sorts.
export function compareValues(a: SortValue, b: SortValue, dir: SortDir): number {
  const ea = a === null || a === undefined || (typeof a === 'string' && BLANK.has(a.trim()))
  const eb = b === null || b === undefined || (typeof b === 'string' && BLANK.has(b.trim()))
  if (ea || eb) return ea === eb ? 0 : ea ? 1 : -1
  const r = typeof a === 'number' && typeof b === 'number'
    ? a - b
    : String(a).localeCompare(String(b), undefined, { numeric: true, sensitivity: 'base' })
  return dir === 'asc' ? r : -r
}

// nextSort is a header click: ascending, then descending, then off.
export function nextSort(cur: SortState | null, key: string): SortState | null {
  if (!cur || cur.key !== key) return { key, dir: 'asc' }
  if (cur.dir === 'asc') return { key, dir: 'desc' }
  return null
}

// useSort sorts rows by the clicked column; `keys` maps a column key to
// the value it sorts by. With no column picked the rows keep their order.
export function useSort<T>(rows: T[], keys: Record<string, (r: T) => SortValue>, initial: SortState | null = null) {
  const [sort, setSort] = useState<SortState | null>(initial)
  const sorted = useMemo(() => {
    if (!sort || !keys[sort.key]) return rows
    const get = keys[sort.key]
    // a stable sort: equal rows keep their incoming order
    return rows.map((r, i) => ({ r, i })).sort((x, y) => compareValues(get(x.r), get(y.r), sort.dir) || x.i - y.i).map((x) => x.r)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [rows, sort])
  return { sorted, sort, toggle: (key: string) => setSort((cur) => nextSort(cur, key)) }
}

// SortArrow shows which way a column sorts (or that it can).
export function SortArrow({ dir }: { dir?: SortDir }) {
  return <span className={`${styles.arrow} ${dir ? styles.on : ''}`} aria-hidden="true">{dir === 'desc' ? '▼' : dir === 'asc' ? '▲' : '↕'}</span>
}

// SortTh is a sortable <th>: click to sort ascending, again descending,
// a third time back to the table's own order.
export function SortTh({ k, sort, onSort, className, children }: { k: string; sort: SortState | null; onSort: (k: string) => void; className?: string; children: ReactNode }) {
  const dir = sort?.key === k ? sort.dir : undefined
  return (
    <th className={className} aria-sort={dir === 'asc' ? 'ascending' : dir === 'desc' ? 'descending' : 'none'}>
      <button type="button" className={styles.btn} onClick={() => onSort(k)}>
        {children}<SortArrow dir={dir} />
      </button>
    </th>
  )
}
