import { createContext, useContext, useState, useEffect } from 'react'

const ThemeContext = createContext()

const THEMES = {
  light: {
    name: 'Light',
    bg: '#ffffff',
    bgAlt: '#f5f5f5',
    surface: '#f0f0f0',
    surfaceHover: '#e5e5e5',
    border: '#d4d4d4',
    text: '#171717',
    textSecondary: '#525252',
    textMuted: '#737373',
    accent: '#171717',
    accentHover: '#404040',
    accentText: '#ffffff',
    playerBg: '#fafafa',
    cardBg: '#ffffff',
    shadow: 'rgba(0,0,0,0.08)',
    inputBg: '#ffffff',
    inputBorder: '#d4d4d4',
    scrollThumb: '#a3a3a3',
  },
  dark: {
    name: 'Dark',
    bg: '#0a0a0a',
    bgAlt: '#141414',
    surface: '#1a1a1a',
    surfaceHover: '#262626',
    border: '#333333',
    text: '#f5f5f5',
    textSecondary: '#a3a3a3',
    textMuted: '#737373',
    accent: '#f5f5f5',
    accentHover: '#d4d4d4',
    accentText: '#0a0a0a',
    playerBg: '#111111',
    cardBg: '#1a1a1a',
    shadow: 'rgba(0,0,0,0.4)',
    inputBg: '#1a1a1a',
    inputBorder: '#333333',
    scrollThumb: '#525252',
  },
  midnight: {
    name: 'Midnight',
    bg: '#0f172a',
    bgAlt: '#1e293b',
    surface: '#1e293b',
    surfaceHover: '#334155',
    border: '#475569',
    text: '#f1f5f9',
    textSecondary: '#94a3b8',
    textMuted: '#64748b',
    accent: '#38bdf8',
    accentHover: '#7dd3fc',
    accentText: '#0f172a',
    playerBg: '#1e293b',
    cardBg: '#1e293b',
    shadow: 'rgba(0,0,0,0.5)',
    inputBg: '#1e293b',
    inputBorder: '#475569',
    scrollThumb: '#475569',
  },
}

function applyTheme(t) {
  const r = document.documentElement
  Object.entries(t).forEach(([k, v]) => {
    if (k !== 'name') r.style.setProperty(`--${k}`, v)
  })
}

export function ThemeProvider({ children }) {
  const [themeName, setThemeName] = useState(() => localStorage.getItem('devmusic-theme') || 'light')
  const [custom, setCustom] = useState(() => {
    try { return JSON.parse(localStorage.getItem('devmusic-custom')) || null } catch { return null }
  })

  const theme = custom || THEMES[themeName] || THEMES.light

  useEffect(() => {
    applyTheme(theme)
    document.body.style.background = theme.bg
    document.body.style.color = theme.text
  }, [theme])

  useEffect(() => {
    localStorage.setItem('devmusic-theme', themeName)
  }, [themeName])

  useEffect(() => {
    if (custom) localStorage.setItem('devmusic-custom', JSON.stringify(custom))
  }, [custom])

  const switchTheme = (name) => {
    setCustom(null)
    setThemeName(name)
  }

  const updateCustom = (patch) => {
    setCustom(prev => ({ ...(prev || THEMES[themeName]), ...patch }))
  }

  return (
    <ThemeContext.Provider value={{ theme, themeName, switchTheme, updateCustom, custom, themes: THEMES }}>
      {children}
    </ThemeContext.Provider>
  )
}

export const useTheme = () => useContext(ThemeContext)
