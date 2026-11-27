import { useMemo, useState, type ReactNode } from 'react'
import { compareValues, nextSort, SortArrow, type SortState, type SortValue } from '../sort'
import type { NodeStatus } from '../types'
import { Dot } from '../LabUi'
import styles from './item-list.module.css'

export interface ItemRow {
  id: string
  status: NodeStatus
  // one cell per column; the first is the name
  cells: ReactNode[]
  // a button at the row's right end (deploy, stop)
  action?: ReactNode
  // can be ticked for a bulk action (only with `checked` on the list);
  // a string says why not
  checkable?: true | string
  // what each column sorts by; a plain text / number cell sorts by itself
  sort?: SortValue[]
}

interface Props {
  label: string
  columns: string[]
  // CSS grid columns after the status dot, e.g. "minmax(0,1fr) 7rem 5rem"
  template: string
  rows: ItemRow[]
  selectedId: string | null
  onSelect: (id: string) => void
  // shown above the rows: filters, search
  head?: ReactNode
  empty: ReactNode
  // tick boxes at the rows' left end, for bulk actions
  checked?: Set<string>
  onChecked?: (next: Set<string>) => void
  // the column it sorts by until a header is clicked
  defaultSort?: SortState
}

// ItemList is a view's left pane: a header row and one selectable row per
// item; the selected item's detail shows on the right.
export default function ItemList({ label, columns, template, rows, selectedId, onSelect, head, empty, checked, onChecked, defaultSort }: Props) {
  // click a header: ascending, descending, back to the list's own order
  const [sort, setSort] = useState<SortState | null>(defaultSort ?? null)
  const ordered = useMemo(() => {
    if (!sort) return rows
    const col = Number(sort.key)
    const value = (r: ItemRow): SortValue => {
      if (r.sort && col < r.sort.length) return r.sort[col]
      const c = r.cells[col]
      return typeof c === 'string' || typeof c === 'number' ? c : undefined
    }
    return rows.map((r, i) => ({ r, i })).sort((a, b) => compareValues(value(a.r), value(b.r), sort.dir) || a.i - b.i).map((x) => x.r)
  }, [rows, sort])
  const withActions = rows.some((r) => r.action)
  const withChecks = Boolean(checked && onChecked)
  const grid = { gridTemplateColumns: `${withChecks ? '1rem ' : ''}0.75rem ${template}${withActions ? ' 4.5rem' : ''}` }
  const tickable = rows.filter((r) => r.checkable === true).map((r) => r.id)
  const allTicked = tickable.length > 0 && tickable.every((id) => checked?.has(id))
  const tick = (id: string, on: boolean) => {
    const next = new Set(checked)
    if (on) next.add(id)
    else next.delete(id)
    onChecked?.(next)
  }
  return (
    <div className={styles.wrap}>
      {head && <div className={styles.head}>{head}</div>}
      {rows.length === 0
        ? <div className={styles.empty}>{empty}</div>
        : (
          <div role="listbox" aria-label={label} className={styles.list}>
            <div className={styles.cols} style={grid}>
              {withChecks && (
                <input
                  type="checkbox"
                  className={styles.check}
                  checked={allTicked}
                  disabled={tickable.length === 0}
                  onChange={(e) => onChecked?.(e.target.checked ? new Set(tickable) : new Set())}
                  aria-label="Tick every row"
                />
              )}
              <span />
              {columns.map((c, i) => {
                const dir = sort?.key === String(i) ? sort.dir : undefined
                return (
                  <button key={c} type="button" className={styles.colBtn} aria-sort={dir === 'asc' ? 'ascending' : dir === 'desc' ? 'descending' : 'none'} onClick={() => setSort((cur) => nextSort(cur, String(i)))}>
                    {c}<SortArrow dir={dir} />
                  </button>
                )
              })}
              {withActions && <span />}
            </div>
            {ordered.map((r) => (
              <div
                key={r.id}
                role="option"
                tabIndex={0}
                aria-selected={r.id === selectedId}
                className={styles.row}
                style={grid}
                onClick={() => onSelect(r.id)}
                onKeyDown={(e) => { if (e.target === e.currentTarget && (e.key === 'Enter' || e.key === ' ')) { e.preventDefault(); onSelect(r.id) } }}
              >
                {withChecks && (
                  <input
                    type="checkbox"
                    className={styles.check}
                    checked={Boolean(checked?.has(r.id))}
                    disabled={r.checkable !== true}
                    title={typeof r.checkable === 'string' ? r.checkable : undefined}
                    onClick={(e) => e.stopPropagation()}
                    onChange={(e) => tick(r.id, e.target.checked)}
                    aria-label={`Tick ${String(r.cells[0])}`}
                  />
                )}
                <Dot status={r.status} />
                {r.cells.map((c, i) => <span key={i} className={i === 0 ? styles.name : styles.cell}>{c}</span>)}
                {withActions && <span className={styles.action} onClick={(e) => e.stopPropagation()} onKeyDown={(e) => e.stopPropagation()}>{r.action}</span>}
              </div>
            ))}
          </div>
        )}
    </div>
  )
}
