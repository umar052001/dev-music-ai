import { useTheme } from '../context/ThemeContext'
import { useRef, useEffect } from 'react'
import gsap from 'gsap'

export default function Header() {
  const { theme, themes, themeName, switchTheme, custom } = useTheme()
  const logoRef = useRef(null)

  useEffect(() => {
    gsap.fromTo(logoRef.current, { opacity: 0, y: -20 }, { opacity: 1, y: 0, duration: 0.6, ease: 'power3.out' })
  }, [])

  return (
    <header style={{
      position: 'sticky', top: 0, zIndex: 100,
      background: theme.bg, borderBottom: `1px solid ${theme.border}`,
      padding: '0.8rem 1.5rem',
      display: 'flex', alignItems: 'center', gap: '1rem',
    }}>
      <div ref={logoRef} style={{ display: 'flex', alignItems: 'baseline', gap: '0.45rem' }}>
        <span style={{
          fontSize: '1.35rem', fontWeight: 800, letterSpacing: '-0.02em',
          color: theme.text, lineHeight: 1,
        }}>Dev<span style={{ fontWeight: 300, color: theme.textSecondary }}>Music</span></span>
      </div>

      <div style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: '0.6rem' }}>
        <select
          value={custom ? 'custom' : themeName}
          onChange={e => switchTheme(e.target.value)}
          style={{
            background: theme.surface, color: theme.text,
            border: `1px solid ${theme.border}`, padding: '0.4rem 0.7rem',
            fontSize: '0.85rem', cursor: 'pointer', borderRadius: 8,
          }}
        >
          {Object.entries(themes).map(([k, v]) => (
            <option key={k} value={k}>{v.name}</option>
          ))}
          <option value="custom">Custom</option>
        </select>
      </div>
    </header>
  )
}
