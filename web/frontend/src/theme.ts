import { useCallback, useState } from 'react'

// Light/dark choice: dark unless the user picked light. Written to
// <html data-theme>, which index.html also applies before first paint.
export type Theme = 'light' | 'dark'

const STORAGE_KEY = 'frulab-theme'

function currentTheme(): Theme {
  try {
    return localStorage.getItem(STORAGE_KEY) === 'light' ? 'light' : 'dark'
  } catch {
    return 'dark'
  }
}

export function useTheme(): [Theme, () => void] {
  const [theme, setTheme] = useState<Theme>(currentTheme)
  const toggle = useCallback(() => {
    const next: Theme = currentTheme() === 'dark' ? 'light' : 'dark'
    document.documentElement.dataset.theme = next
    try { localStorage.setItem(STORAGE_KEY, next) } catch { /* private mode: this page only */ }
    setTheme(next)
  }, [])
  return [theme, toggle]
}
