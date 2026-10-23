import { useEffect, useState } from 'react'

export type ThemePref = 'system' | 'light' | 'dark'

function read(): ThemePref {
  try {
    const t = localStorage.getItem('theme')
    return t === 'light' || t === 'dark' ? t : 'system'
  } catch {
    return 'system'
  }
}

/** The theme preference, applied as data-theme on <html> (see index.html). */
export function useTheme() {
  const [pref, setPref] = useState<ThemePref>(read)
  useEffect(() => {
    const root = document.documentElement
    if (pref === 'system') delete root.dataset.theme
    else root.dataset.theme = pref
    try {
      if (pref === 'system') localStorage.removeItem('theme')
      else localStorage.setItem('theme', pref)
    } catch {
      // storage unavailable: the choice lasts for this page only
    }
  }, [pref])
  const cycle = () => setPref((p) => (p === 'system' ? 'light' : p === 'light' ? 'dark' : 'system'))
  return { pref, cycle }
}
