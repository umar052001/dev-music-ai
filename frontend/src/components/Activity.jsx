import { useState, useEffect } from 'react'
import { useTheme } from '../context/ThemeContext'

export default function Activity() {
  const { theme } = useTheme()
  const [activity, setActivity] = useState([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetch('/api/activity?limit=30')
      .then(r => r.json())
      .then(d => { setActivity(d.activity || []); setLoading(false) })
      .catch(() => setLoading(false))
  }, [])

  const actionIcons = {
    play: '▶',
    search: '🔍',
    download: '⬇',
    skip: '⏭',
  }

  const fmtTime = (t) => {
    if (!t) return ''
    const d = new Date(t + 'Z')
    const now = new Date()
    const diff = (now - d) / 1000
    if (diff < 60) return 'just now'
    if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
    if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`
    return d.toLocaleDateString()
  }

  return (
    <div style={{
      background: theme.cardBg, border: `1px solid ${theme.border}`,
      borderRadius: 12, padding: '1.2rem',
    }}>
      <div style={{ fontSize: '1rem', fontWeight: 600, marginBottom: '0.8rem' }}>📋 Activity</div>

      {loading && (
        <div style={{ padding: '1rem', color: theme.textMuted, fontSize: '0.9rem', textAlign: 'center' }}>
          Loading...
        </div>
      )}

      {!loading && activity.length === 0 && (
        <div style={{ padding: '1rem', color: theme.textMuted, fontSize: '0.9rem', textAlign: 'center' }}>
          No activity yet
        </div>
      )}

      {activity.map((a, i) => (
        <div key={i} style={{
          display: 'flex', alignItems: 'center', gap: '0.6rem',
          padding: '0.4rem 0',
          borderBottom: i < activity.length - 1 ? `1px solid ${theme.border}` : 'none',
        }}>
          <span style={{ fontSize: '0.8rem', width: 24, textAlign: 'center', flexShrink: 0 }}>
            {actionIcons[a.action] || '•'}
          </span>
          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{
              fontSize: '0.85rem', whiteSpace: 'nowrap',
              overflow: 'hidden', textOverflow: 'ellipsis',
            }}>
              {a.track || a.query || '—'}
            </div>
            {a.artist && (
              <div style={{ fontSize: '0.7rem', color: theme.textMuted }}>{a.artist}</div>
            )}
          </div>
          <span style={{ fontSize: '0.7rem', color: theme.textMuted, flexShrink: 0 }}>
            {fmtTime(a.created_at)}
          </span>
        </div>
      ))}
    </div>
  )
}
