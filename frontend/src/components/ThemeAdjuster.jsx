import { useState } from 'react'
import { useTheme } from '../context/ThemeContext'

const FIELDS = [
  { key: 'bg', label: 'Background' },
  { key: 'bgAlt', label: 'Alt Background' },
  { key: 'surface', label: 'Surface' },
  { key: 'cardBg', label: 'Card Background' },
  { key: 'border', label: 'Border' },
  { key: 'text', label: 'Text' },
  { key: 'textSecondary', label: 'Secondary Text' },
  { key: 'textMuted', label: 'Muted Text' },
  { key: 'accent', label: 'Accent' },
  { key: 'inputBg', label: 'Input Background' },
]

export default function ThemeAdjuster({ open, onClose }) {
  const { theme, themeName, switchTheme, updateCustom } = useTheme()
  const [local, setLocal] = useState({})

  if (!open) return null

  const current = { ...theme, ...local }

  const set = (key, val) => {
    const patch = { ...local, [key]: val }
    setLocal(patch)
    updateCustom(patch)
  }

  return (
    <div style={{
      position: 'fixed', top: 0, right: 0, bottom: 0, width: 340, zIndex: 300,
      background: theme.bg, borderLeft: `1px solid ${theme.border}`,
      boxShadow: '-4px 0 20px ' + theme.shadow,
      display: 'flex', flexDirection: 'column', overflow: 'hidden',
    }}>
      <div style={{
        padding: '1rem 1.2rem', borderBottom: `1px solid ${theme.border}`,
        display: 'flex', justifyContent: 'space-between', alignItems: 'center',
      }}>
        <span style={{ fontSize: '1.1rem', fontWeight: 700 }}>Theme Settings</span>
        <button onClick={onClose} style={{
          background: 'none', border: 'none', fontSize: '1.3rem',
          cursor: 'pointer', color: theme.text,
        }}>✕</button>
      </div>

      <div style={{ padding: '1rem 1.2rem', borderBottom: `1px solid ${theme.border}` }}>
        <div style={{ fontSize: '0.8rem', color: theme.textMuted, marginBottom: '0.5rem' }}>PRESETS</div>
        <div style={{ display: 'flex', gap: '0.4rem', flexWrap: 'wrap' }}>
          {['light', 'dark', 'midnight'].map(name => (
            <button key={name} onClick={() => switchTheme(name)} style={{
              background: themeName === name ? theme.text : theme.surface,
              color: themeName === name ? theme.bg : theme.text,
              border: `1px solid ${theme.border}`, padding: '0.4rem 0.8rem',
              fontSize: '0.8rem', cursor: 'pointer', textTransform: 'capitalize',
            }}>{name}</button>
          ))}
        </div>
      </div>

      <div style={{ flex: 1, overflowY: 'auto', padding: '1rem 1.2rem' }}>
        <div style={{ fontSize: '0.8rem', color: theme.textMuted, marginBottom: '0.7rem' }}>CUSTOMIZE COLORS</div>
        {FIELDS.map(f => (
          <div key={f.key} style={{ display: 'flex', alignItems: 'center', gap: '0.6rem', marginBottom: '0.6rem' }}>
            <label style={{ flex: 1, fontSize: '0.85rem' }}>{f.label}</label>
            <input type="color" value={current[f.key] || '#000000'}
              onChange={e => set(f.key, e.target.value)}
              style={{ width: 32, height: 28, border: `1px solid ${theme.border}`, cursor: 'pointer', padding: 0 }}
            />
            <input type="text" value={current[f.key] || ''}
              onChange={e => set(f.key, e.target.value)}
              style={{
                width: 80, background: theme.inputBg, color: theme.text,
                border: `1px solid ${theme.inputBorder}`, padding: '0.25rem 0.4rem',
                fontSize: '0.75rem', fontFamily: 'monospace',
              }}
            />
          </div>
        ))}
      </div>

      <div style={{ padding: '0.8rem 1.2rem', borderTop: `1px solid ${theme.border}`, fontSize: '0.7rem', color: theme.textMuted }}>
        Changes save automatically
      </div>
    </div>
  )
}
