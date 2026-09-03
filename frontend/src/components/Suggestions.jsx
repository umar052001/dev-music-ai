import { useState, useEffect, useRef } from 'react'
import { useTheme } from '../context/ThemeContext'
import gsap from 'gsap'
import LibraryPlaylist from './LibraryPlaylist'

export default function Suggestions({ onSearch }) {
  const { theme } = useTheme()
  const [suggestions, setSuggestions] = useState([])
  const [playlistQuery, setPlaylistQuery] = useState('')
  const [playlist, setPlaylist] = useState(null)
  const [loading, setLoading] = useState(true)
  const [playlistLoading, setPlaylistLoading] = useState(false)
  const listRef = useRef(null)

  useEffect(() => {
    fetch('/api/suggestions')
      .then(r => r.json())
      .then(d => { setSuggestions(d.suggestions || []); setLoading(false) })
      .catch(() => setLoading(false))
  }, [])

  useEffect(() => {
    if (suggestions.length && listRef.current) {
      gsap.fromTo(listRef.current.children,
        { opacity: 0, x: -15 },
        { opacity: 1, x: 0, duration: 0.35, stagger: 0.06, ease: 'power2.out' }
      )
    }
  }, [suggestions])

  const generatePlaylist = async () => {
    if (!playlistQuery.trim()) return
    setPlaylistLoading(true)
    try {
      const res = await fetch('/api/playlist-suggest', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ description: playlistQuery }),
      })
      const data = await res.json()
      setPlaylist(data)
    } catch {
      setPlaylist(null)
    } finally {
      setPlaylistLoading(false)
    }
  }

  const typeColors = {
    similar: theme.text,
    discovery: theme.textSecondary,
    mood: theme.textMuted,
    trending: theme.text,
  }

  const typeLabels = {
    similar: 'Similar',
    discovery: 'Discover',
    mood: 'Mood',
    trending: 'Trending',
  }

  return (
    <div style={{ marginBottom: '1.5rem' }}>
      {/* AI Suggestions */}
      <div style={{
        background: theme.cardBg, border: `1px solid ${theme.border}`,
        borderRadius: 12, padding: '1.2rem', marginBottom: '1rem',
      }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '0.8rem' }}>
          <span style={{ fontSize: '1rem', fontWeight: 600 }}>✨ Suggested For You</span>
          <button onClick={() => {
            setLoading(true)
            fetch('/api/suggestions').then(r => r.json()).then(d => { setSuggestions(d.suggestions || []); setLoading(false) })
          }} style={{
            background: 'none', border: `1px solid ${theme.border}`, color: theme.textSecondary,
            padding: '0.3rem 0.6rem', fontSize: '0.8rem', cursor: 'pointer',
          }}>↻ Refresh</button>
        </div>

        {loading && (
          <div style={{ padding: '1rem', color: theme.textMuted, fontSize: '0.9rem', textAlign: 'center' }}>
            Analyzing your music taste...
          </div>
        )}

        {!loading && suggestions.length === 0 && (
          <div style={{ padding: '1rem', color: theme.textMuted, fontSize: '0.9rem', textAlign: 'center' }}>
            Play some music to get personalized suggestions
          </div>
        )}

        <div ref={listRef} style={{ display: 'flex', flexDirection: 'column', gap: '0' }}>
          {suggestions.map((s, i) => (
            <div key={i} onClick={() => onSearch(s.query)} style={{
              display: 'flex', alignItems: 'center', gap: '0.8rem',
              padding: '0.6rem 0.7rem', cursor: 'pointer',
              borderBottom: i < suggestions.length - 1 ? `1px solid ${theme.border}` : 'none',
              transition: 'background 0.15s',
            }}
              onMouseEnter={e => e.currentTarget.style.background = theme.surfaceHover}
              onMouseLeave={e => e.currentTarget.style.background = 'transparent'}
            >
              <span style={{
                padding: '0.15rem 0.5rem', fontSize: '0.65rem', fontWeight: 600,
                background: theme.surface, color: typeColors[s.type] || theme.text,
                border: `1px solid ${theme.border}`, textTransform: 'uppercase',
                letterSpacing: '0.5px', flexShrink: 0,
              }}>
                {typeLabels[s.type] || s.type}
              </span>
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{ fontSize: '0.95rem', fontWeight: 500 }}>{s.query}</div>
                <div style={{ fontSize: '0.75rem', color: theme.textMuted, marginTop: 1 }}>{s.reason}</div>
              </div>
              <span style={{ fontSize: '0.85rem', color: theme.textMuted }}>→</span>
            </div>
          ))}
        </div>
      </div>

      {/* Playlist Generator */}
      <div style={{
        background: theme.cardBg, border: `1px solid ${theme.border}`,
        borderRadius: 12, padding: '1.2rem',
      }}>
        <div style={{ fontSize: '1rem', fontWeight: 600, marginBottom: '0.8rem' }}>🎵 Generate Playlist</div>
        <div style={{ display: 'flex', gap: '0.5rem' }}>
          <input
            style={{
              flex: 1, background: theme.inputBg, color: theme.text,
              border: `2px solid ${theme.inputBorder}`, padding: '0.6rem 0.8rem',
              fontSize: '0.95rem', outline: 'none',
            }}
            placeholder="Describe your mood... e.g. 'chill evening Pakistani music'"
            value={playlistQuery}
            onChange={e => setPlaylistQuery(e.target.value)}
            onKeyDown={e => e.key === 'Enter' && generatePlaylist()}
            onFocus={e => e.target.style.borderColor = theme.text}
            onBlur={e => e.target.style.borderColor = theme.inputBorder}
          />
          <button onClick={generatePlaylist} disabled={playlistLoading} style={{
            background: theme.text, color: theme.bg, border: 'none',
            padding: '0.6rem 1rem', fontSize: '0.9rem', fontWeight: 600,
            cursor: playlistLoading ? 'wait' : 'pointer', opacity: playlistLoading ? 0.6 : 1,
          }}>
            {playlistLoading ? '...' : 'Generate'}
          </button>
        </div>

        {playlist && (
          <div style={{
            marginTop: '0.8rem', padding: '0.8rem',
            background: theme.surface, border: `1px solid ${theme.border}`,
          }}>
            <div style={{ fontSize: '1rem', fontWeight: 600 }}>{playlist.name}</div>
            <div style={{ fontSize: '0.8rem', color: theme.textMuted, marginTop: '0.3rem' }}>
              {playlist.track_count} tracks · {playlist.mood}
            </div>
            <button onClick={() => onSearch(playlist.query)} style={{
              marginTop: '0.6rem', background: theme.text, color: theme.bg,
              border: 'none', padding: '0.5rem 1rem', fontSize: '0.85rem',
              fontWeight: 600, cursor: 'pointer',
            }}>
              Search & Download
            </button>
          </div>
        )}
      </div>

      {/* AI Playlist from downloaded songs */}
      <div style={{ marginTop: '1rem' }}>
        <LibraryPlaylist />
      </div>
    </div>
  )
}
