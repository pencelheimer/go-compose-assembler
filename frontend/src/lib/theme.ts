import { useEffect, useState } from 'react'

export type Theme = 'light' | 'dark'

// index.html sets data-theme before first paint (saved choice, else system).
const current = (): Theme => (document.documentElement.dataset.theme === 'dark' ? 'dark' : 'light')

/** The active theme; toggling saves the choice for later visits. */
export function useTheme() {
  const [theme, setTheme] = useState<Theme>(current)
  useEffect(() => {
    document.documentElement.dataset.theme = theme
  }, [theme])
  const toggle = () => {
    const next = theme === 'dark' ? 'light' : 'dark'
    try {
      localStorage.setItem('theme', next)
    } catch {
      // storage unavailable: the choice lasts for this page only
    }
    setTheme(next)
  }
  return { theme, toggle }
}
