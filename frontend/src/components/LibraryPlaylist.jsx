import { useState } from 'react'
import { useTheme } from '../context/ThemeContext'
import { usePlayer } from '../context/PlayerContext'

// LibraryPlaylist lets the user ask the AI to build a themed playlist out of
// the songs they have already downloaded, then play it instantly.
export default function LibraryPlaylist() {
  const { theme } = useTheme()
  const { play, current, playing, toggle } = usePlayer()
  const [desc, setDesc] = useState('')
  const [count, setCount] = useState(12)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [playlist, setPlaylist] = useState(null)

  const generate = async () => {
    if (!desc.trim()) return
    setLoading(true)
    setError('')
    setPlaylist(null)
    try {
      const res = await fetch('/api/library/playlist', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ description: desc.trim(), track_count: count }),
      })
      const data = await res.json()
      if (data.error) {
        setError(data.error)
      } else {
        setPlaylist(data)
      }
    } catch {
      setError('Could not reach the server. Please try again.')
    } finally {
      setLoading(false)
    }
  }

  const playAll = () => {
    if (!playlist || !playlist.tracks.length) return
    const list = playlist.tracks.map(t => ({ path: t.path, title: t.title, artist: t.artist }))
    play(list[0], list)
  }

  const playTrack = (i) => {
    const list = playlist.tracks.map(t => ({ path: t.path, title: t.title, artist: t.artist }))
    play(list[i], list)
  }

  const inputStyle = {
    background: theme.inputBg, color: theme.text,
    border: `2px solid ${theme.inputBorder}`, padding: '0.6rem 0.8rem',
    fontSize: '0.95rem', outline: 'none', borderRadius: 8,
    transition: 'border-color 0.2s, box-shadow 0.2s',
  }

  return (
    <div style={{
      background: theme.cardBg, border: `1px solid ${theme.border}`,
      borderRadius: 12, padding: '1.2rem',
    }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '0.4rem', flexWrap: 'wrap', gap: '0.5rem' }}>
        <div style={{ fontSize: '1rem', fontWeight: 600 }}>🎶 AI Playlist from your Music</div>
      </div>
      <div style={{ fontSize: '0.82rem', color: theme.textMuted, marginBottom: '0.8rem' }}>
        The AI builds a playlist from songs you've already downloaded — then press play.
      </div>

      <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }}>
        <input
          style={{ ...inputStyle, flex: 1, minWidth: 180 }}
          placeholder="Describe the vibe... e.g. 'rainy night, sad Pakistani songs'"
          value={desc}
          onChange={e => setDesc(e.target.value)}
          onKeyDown={e => e.key === 'Enter' && generate()}
          onFocus={e => { e.target.style.borderColor = theme.text; e.target.style.boxShadow = `0 0 0 3px ${theme.accent}22` }}
          onBlur={e => { e.target.style.borderColor = theme.inputBorder; e.target.style.boxShadow = 'none' }}
        />
        <select
          style={{ ...inputStyle, width: 'auto', cursor: 'pointer' }}
          value={count}
          onChange={e => setCount(Number(e.target.value))}
          title="How many songs"
        >
          {[5, 8, 10, 12, 15, 20].map(n => <option key={n} value={n}>{n} songs</option>)}
        </select>
        <button onClick={generate} disabled={loading} style={{
          background: theme.text, color: theme.bg, border: 'none',
          padding: '0.6rem 1.2rem', fontSize: '0.9rem', fontWeight: 700,
          borderRadius: 8, cursor: loading ? 'wait' : 'pointer', opacity: loading ? 0.6 : 1,
          transition: 'opacity 0.2s',
        }}>
          {loading ? 'Thinking…' : 'Build Playlist'}
        </button>
      </div>

      {error && (
        <div style={{ marginTop: '0.8rem', fontSize: '0.88rem', color: 'tomato' }}>⚠ {error}</div>
      )}

      {playlist && (
        <div style={{ marginTop: '1rem', borderTop: `1px solid ${theme.border}`, paddingTop: '0.8rem' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '0.5rem' }}>
            <div>
              <div style={{ fontSize: '1.1rem', fontWeight: 700 }}>{playlist.name}</div>
              {playlist.mood && (
                <div style={{ fontSize: '0.8rem', color: theme.textMuted, marginTop: 2 }}>
                  Mood: {playlist.mood} · {playlist.tracks.length} tracks
                </div>
              )}
            </div>
            <div style={{ display: 'flex', gap: '0.4rem' }}>
              <button onClick={playAll} style={{
                background: theme.text, color: theme.bg, border: 'none',
                padding: '0.5rem 1rem', fontSize: '0.85rem', fontWeight: 700, borderRadius: 8, cursor: 'pointer',
              }}>▶ Play All</button>
              <button onClick={generate} disabled={loading} style={{
                background: 'transparent', color: theme.textSecondary, border: `1px solid ${theme.border}`,
                padding: '0.5rem 1rem', fontSize: '0.85rem', borderRadius: 8, cursor: loading ? 'wait' : 'pointer',
              }}>↻ Rebuild</button>
            </div>
          </div>

          <div style={{ marginTop: '0.8rem' }}>
            {playlist.tracks.map((t, i) => {
              const isCurrent = current && current.path === t.path
              const isPlayingThis = isCurrent && playing
              return (
                <div
                  key={i}
                  onClick={() => isCurrent ? toggle() : playTrack(i)}
                  style={{
                    display: 'flex', alignItems: 'center', gap: '0.7rem',
                    padding: '0.5rem 0.6rem', borderRadius: 8, cursor: 'pointer',
                    background: isCurrent ? theme.surfaceHover : 'transparent',
                    borderBottom: i < playlist.tracks.length - 1 ? `1px solid ${theme.border}` : 'none',
                  }}
                  onMouseEnter={e => e.currentTarget.style.background = theme.surfaceHover}
                  onMouseLeave={e => e.currentTarget.style.background = isCurrent ? theme.surfaceHover : 'transparent'}
                >
                  <span style={{ width: 22, textAlign: 'center', color: theme.textMuted, fontSize: '0.85rem' }}>
                    {isPlayingThis ? '⏸' : isCurrent ? '▶' : i + 1}
                  </span>
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{ fontSize: '0.95rem', fontWeight: 500, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                      {t.title}
                    </div>
                    <div style={{ fontSize: '0.78rem', color: theme.textMuted }}>{t.artist}</div>
                  </div>
                </div>
              )
            })}
          </div>
        </div>
      )}
    </div>
  )
}
