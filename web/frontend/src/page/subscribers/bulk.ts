// Helpers for the subscriber list's paging and the bulk create / delete.

// BULK_CONCURRENCY is how many webconsole requests a bulk create or
// delete runs at once.
export const BULK_CONCURRENCY = 8

// PAGE_SIZE is how many subscribers one page of the list shows.
export const PAGE_SIZE = 10

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

export interface Failure<T> {
  item: T
  error: unknown
}

// runLimited calls run for every item, at most limit at a time, so the
// webconsole is not flooded. It keeps going past failures and returns
// them in input order; onProgress gets the number finished so far.
export async function runLimited<T>(
  items: T[],
  limit: number,
  run: (item: T) => Promise<unknown>,
  onProgress?: (done: number) => void,
): Promise<Failure<T>[]> {
  const failed: { index: number, failure: Failure<T> }[] = []
  let next = 0
  let done = 0
  const worker = async () => {
    while (next < items.length) {
      const index = next++
      try {
        await run(items[index])
      } catch (error) {
        failed.push({ index, failure: { item: items[index], error } })
      }
      done++
      onProgress?.(done)
    }
  }
  await Promise.all(Array.from({ length: Math.min(limit, items.length) }, worker))
  return failed.sort((a, b) => a.index - b.index).map((f) => f.failure)
}
