// Client-side paging for the app's long tables.

export interface Page<T> {
  items: T[]
  page: number // 1-based, clamped to the pages that exist
  pages: number // at least 1
  from: number // 1-based index of the first item shown, 0 when empty
  to: number
}

// pageOf returns page (1-based) of items, size per page.
export function pageOf<T>(items: T[], page: number, size: number): Page<T> {
  const pages = Math.max(1, Math.ceil(items.length / size))
  const current = Math.min(Math.max(1, page), pages)
  const start = (current - 1) * size
  const shown = items.slice(start, start + size)
  return { items: shown, page: current, pages, from: shown.length ? start + 1 : 0, to: start + shown.length }
}
