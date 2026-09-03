import { useState, useEffect, useRef, useCallback } from 'react'
import { useTheme } from '../context/ThemeContext'

// Human-friendly description and icon per activity action.
const ACTION_META = {
  play:     { icon: '▶', label: 'Played', desc: (a) => description(a, san(a.track)) },
  download: { icon: '⬇', label: 'Downloaded', desc: (a) => description(a, san(a.track)) },
  search:   { icon: '🔍', label: 'Searched', desc: (a) => a.query ? `"${a.query}"` : 'searched the library' },
  skip:     { icon: '⏭', label: 'Skipped', desc: (a) => description(a, san(a.track)) },
  login:    { icon: '●', label: 'Logged in', desc: () => 'session started' },
}
const DEFAULT_META = { icon: '•', label: 'Activity', desc: (a) => description(a, san(a.track)) }

function san(s) { return (s || '').trim() }
function description(a, name) {
  if (name) return name
  if (a.query) return `"${a.query}"`
  return '—'
}

// Parse a backend timestamp that may or may not carry a timezone suffix, and
// return a valid Date. Backend returns RFC3339 UTC (e.g. 2026-09-03T20:57:53Z).
function toDate(t) {
  if (!t) return null
  let s = String(t).trim()
  if (s && !/[zZ]|[+-]\d{2}:?\d{2}$/.test(s)) s += 'Z' // assume UTC when missing
  const d = new Date(s)
  return isNaN(d.getTime()) ? null : d
}

function fmtTime(t) {
  const d = toDate(t)
  if (!d) return ''
  const diff = (Date.now() - d.getTime()) / 1000
  if (diff < 5) return 'just now'
  if (diff < 60) return `${Math.floor(diff)}s ago`
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`
  if (diff < 604800) return `${Math.floor(diff / 86400)}d ago`
  return d.toLocaleDateString()
}

function dayLabel(t) {
  const d = toDate(t)
  if (!d) return 'Today'
  const today = new Date()
  const startOf = (x) => new Date(x.getFullYear(), x.getMonth(), x.getDate()).getTime()
  const diffDays = Math.round((startOf(today) - startOf(d)) / 86400000)
  if (diffDays === 0) return 'Today'
  if (diffDays === 1) return 'Yesterday'
  if (diffDays === 2) return '2 days ago'
  return d.toLocaleDateString(undefined, { weekday: 'long', month: 'short', day: 'numeric' })
}

export default function Activity() {
  const { theme } = useTheme()
  const [activity, setActivity] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)
  const timer = useRef(null)

  const load = useCallback(async () => {
    try {
      const r = await fetch('/api/activity?limit=100')
      const d = await r.json()
      setActivity(d.activity || [])
      setError(false)
    } catch {
      setError(true)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    load()
    timer.current = setInterval(load, 10000) // auto-refresh while open
    return () => { if (timer.current) clearInterval(timer.current) }
  }, [load])

  // Group by day, newest first (backend already returns newest-first).
  const groups = []
  const byDay = {}
  for (const a of activity) {
    const key = dayLabel(a.created_at)
    if (!byDay[key]) { byDay[key] = []; groups.push(key) }
    byDay[key].push(a)
  }

  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '0.8rem' }}>
        <div>
          <div style={{ fontSize: '1.1rem', fontWeight: 600 }}>Activity</div>
          <div style={{ fontSize: '0.82rem', color: theme.textMuted, marginTop: 2 }}>
            What you've searched, played, and downloaded — updates automatically.
          </div>
        </div>
        <button
          onClick={load}
          aria-label="Refresh activity"
          title="Refresh activity"
          style={{
            background: 'none', border: `1px solid ${theme.border}`, color: theme.textSecondary,
            padding: '0.4rem 0.8rem', fontSize: '0.82rem', borderRadius: 8, cursor: 'pointer',
            display: 'flex', alignItems: 'center', gap: '0.35rem',
          }}
        >
          ↻ Refresh
        </button>
      </div>

      <div style={{
        background: theme.cardBg, border: `1px solid ${theme.border}`,
        borderRadius: 12, padding: '0.6rem 1.2rem 1.2rem',
      }}>
        {loading && (
          <div style={{ padding: '1.5rem', color: theme.textMuted, fontSize: '0.9rem', textAlign: 'center' }}>
            Loading your activity…
          </div>
        )}

        {!loading && error && activity.length === 0 && (
          <div style={{ padding: '1.5rem', color: theme.textMuted, fontSize: '0.9rem', textAlign: 'center' }}>
            Couldn't load activity. Is the server running?
          </div>
        )}

        {!loading && !error && activity.length === 0 && (
          <div style={{ padding: '2rem', color: theme.textMuted, fontSize: '0.9rem', textAlign: 'center' }}>
            No activity yet. Search for a song, play it, or download something —
            it'll show up here.
          </div>
        )}

        {!loading && activity.length > 0 && (
          <>
            {groups.map(day => (
              <div key={day} style={{ marginTop: '0.8rem' }}>
                <div style={{
                  fontSize: '0.72rem', fontWeight: 700, letterSpacing: '0.06em',
                  textTransform: 'uppercase', color: theme.textMuted, marginBottom: '0.3rem',
                }}>
                  {day}
                </div>
                {byDay[day].map((a, i) => {
                  const meta = ACTION_META[a.action] || DEFAULT_META
                  return (
                    <div
                      key={a.id || `${a.action}-${i}`}
                      aria-label={`${meta.label} ${description(a, san(a.track))}`}
                      style={{
                        display: 'flex', alignItems: 'center', gap: '0.7rem',
                        padding: '0.5rem 0.3rem', borderRadius: 8,
                        borderBottom: '1px solid ' + theme.border,
                      }}
                    >
                      <span
                        aria-hidden="true"
                        style={{
                          width: 30, height: 30, flexShrink: 0, borderRadius: 8,
                          display: 'flex', alignItems: 'center', justifyContent: 'center',
                          background: theme.surface, color: theme.text, fontSize: '0.8rem',
                        }}
                      >
                        {meta.icon}
                      </span>
                      <div style={{ flex: 1, minWidth: 0 }}>
                        <div style={{
                          fontSize: '0.88rem', fontWeight: 500, whiteSpace: 'nowrap',
                          overflow: 'hidden', textOverflow: 'ellipsis', color: theme.text,
                        }}>
                          <span style={{ color: theme.textMuted, fontWeight: 600, marginRight: '0.35rem' }}>
                            {meta.label}
                          </span>
                          {meta.desc(a)}
                        </div>
                        {a.artist && (
                          <div style={{ fontSize: '0.75rem', color: theme.textMuted, marginTop: 1 }}>
                            {a.artist}
                          </div>
                        )}
                      </div>
                      <span style={{ fontSize: '0.72rem', color: theme.textMuted, flexShrink: 0 }}>
                        {fmtTime(a.created_at)}
                      </span>
                    </div>
                  )
                })}
              </div>
            ))}
          </>
        )}
      </div>
    </div>
  )
}
