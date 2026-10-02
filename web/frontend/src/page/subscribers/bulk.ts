// Helpers for the subscriber list's bulk create and delete.

// BULK_CONCURRENCY is how many webconsole requests a bulk create or
// delete runs at once.
export const BULK_CONCURRENCY = 8

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
