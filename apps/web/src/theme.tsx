import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'
import type { ReactNode } from 'react'

export type ThemeMode = 'auto' | 'light' | 'dark'
export type ResolvedTheme = 'light' | 'dark'

const THEME_STORAGE_KEY = 'bikerskz.theme'

function readStoredMode(): ThemeMode {
  try {
    const value = localStorage.getItem(THEME_STORAGE_KEY)
    if (value === 'light' || value === 'dark') return value
  } catch { /* Storage can be unavailable in restricted contexts. */ }
  return 'auto'
}

function systemTheme(): ResolvedTheme {
  return window.matchMedia?.('(prefers-color-scheme: light)').matches ? 'light' : 'dark'
}

function applyThemeToDocument(resolved: ResolvedTheme, mode: ThemeMode) {
  const root = document.documentElement
  if (mode === 'auto') root.removeAttribute('data-theme')
  else root.setAttribute('data-theme', mode)
  root.setAttribute('data-resolved-theme', resolved)
  const meta = document.querySelector<HTMLMetaElement>('meta[name="theme-color"]')
  if (meta) meta.content = resolved === 'light' ? '#f4f6f9' : '#0b0e13'
}

interface ThemeContextValue {
  mode: ThemeMode
  resolved: ResolvedTheme
  setMode: (mode: ThemeMode) => void
  cycleMode: () => void
}

const ThemeContext = createContext<ThemeContextValue>({
  mode: 'auto',
  resolved: 'dark',
  setMode: () => {},
  cycleMode: () => {},
})

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [mode, setModeState] = useState<ThemeMode>(readStoredMode)
  const [system, setSystem] = useState<ResolvedTheme>(systemTheme)

  useEffect(() => {
    const media = window.matchMedia?.('(prefers-color-scheme: light)')
    if (!media) return
    const update = () => setSystem(media.matches ? 'light' : 'dark')
    media.addEventListener('change', update)
    return () => media.removeEventListener('change', update)
  }, [])

  const resolved: ResolvedTheme = mode === 'auto' ? system : mode

  useEffect(() => {
    applyThemeToDocument(resolved, mode)
  }, [resolved, mode])

  const setMode = useCallback((next: ThemeMode) => {
    setModeState(next)
    try {
      if (next === 'auto') localStorage.removeItem(THEME_STORAGE_KEY)
      else localStorage.setItem(THEME_STORAGE_KEY, next)
    } catch { /* Ignore storage quota/privacy failures. */ }
  }, [])

  const cycleMode = useCallback(() => {
    setMode(mode === 'auto' ? (system === 'dark' ? 'light' : 'dark') : mode === 'light' ? 'dark' : 'auto')
  }, [mode, setMode, system])

  const value = useMemo(() => ({ mode, resolved, setMode, cycleMode }), [mode, resolved, setMode, cycleMode])
  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>
}

export function useTheme() {
  return useContext(ThemeContext)
}
