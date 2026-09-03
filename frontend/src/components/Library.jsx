import { useState, useEffect, useRef } from 'react'
import { useTheme } from '../context/ThemeContext'
import { usePlayer } from '../context/PlayerContext'
import gsap from 'gsap'

export default function Library() {
  const { theme } = useTheme()
  const { play, current } = usePlayer()
  const [lib, setLib] = useState({ artists: [] })
  const [loading, setLoading] = useState(true)
  const gridRef = useRef(null)

  useEffect(() => {
    fetch('/api/library')
      .then(r => r.json())
      .then(d => { setLib(d); setLoading(false) })
      .catch(() => setLoading(false))
  }, [])

  useEffect(() => {
    if (lib.artists.length && gridRef.current) {
      gsap.fromTo(
        gridRef.current.children,
        { opacity: 0, y: 30, scale: 0.95 },
        { opacity: 1, y: 0, scale: 1, duration: 0.5, stagger: 0.08, ease: 'power3.out' }
      )
    }
  }, [lib])

  const fmtSize = (b) => {
    if (b < 1024) return b + ' B'
    if (b < 1048576) return (b / 1024).toFixed(1) + ' KB'
    return (b / 1048576).toFixed(1) + ' MB'
  }

  if (loading) {
    return (
      <div style={{ textAlign: 'center', padding: '3rem', color: theme.textMuted, fontSize: '1rem' }}>
        Loading library...
      </div>
    )
  }

  if (!lib.artists.length) {
    return (
      <div style={{ textAlign: 'center', padding: '3rem', color: theme.textMuted, fontSize: '1rem' }}>
        No music downloaded yet. Search and download some tracks!
      </div>
    )
  }

  return (
    <div ref={gridRef} style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(320px, 1fr))', gap: '1rem' }}>
      {lib.artists.map(a => {
        const total = a.albums.reduce((s, al) => s + al.tracks.length, 0)
        return (
          <div key={a.name} style={{
            background: theme.cardBg, border: `1px solid ${theme.border}`,
            borderRadius: 12, overflow: 'hidden',
          }}>
            <div style={{
              padding: '1rem', display: 'flex', alignItems: 'center', gap: '0.8rem',
              borderBottom: `1px solid ${theme.border}`, background: theme.surface,
            }}>
              <div style={{
                width: 44, height: 44, background: theme.text, color: theme.bg,
                display: 'flex', alignItems: 'center', justifyContent: 'center',
                fontSize: '1.1rem', fontWeight: 700, flexShrink: 0, borderRadius: 10,
              }}>
                {(a.name || '?')[0].toUpperCase()}
              </div>
              <div>
                <div style={{ fontSize: '1rem', fontWeight: 600 }}>{a.name}</div>
                <div style={{ fontSize: '0.8rem', color: theme.textMuted, marginTop: 2 }}>
                  {total} tracks · {a.albums.length} {a.albums.length === 1 ? 'album' : 'albums'}
                </div>
              </div>
            </div>
            <div style={{ maxHeight: 280, overflowY: 'auto' }}>
              {a.albums.map(al => (
                <div key={al.name} style={{ padding: '0.5rem 0' }}>
                  <div style={{
                    padding: '0.3rem 1rem', fontSize: '0.8rem', fontWeight: 600,
                    color: theme.textSecondary, textTransform: 'uppercase', letterSpacing: '0.5px',
                  }}>
                    {al.name}
                  </div>
                  {al.tracks.map((t, ti) => {
                    const isPlaying = current && current.path === t.path
                    return (
                      <div key={t.path} onClick={() => play(t, al.tracks)} style={{
                        display: 'flex', alignItems: 'center', gap: '0.5rem',
                        padding: '0.4rem 1rem', fontSize: '0.9rem',
                        cursor: 'pointer', transition: 'background 0.1s',
                        background: isPlaying ? theme.surfaceHover : 'transparent',
                      }}
                        onMouseEnter={e => e.currentTarget.style.background = theme.surfaceHover}
                        onMouseLeave={e => e.currentTarget.style.background = isPlaying ? theme.surfaceHover : 'transparent'}
                      >
                        <span style={{ width: 22, color: theme.textMuted, fontSize: '0.8rem', flexShrink: 0 }}>
                          {isPlaying ? '♪' : ti + 1}
                        </span>
                        <span style={{ flex: 1, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                          {t.name.replace(/\.[^.]+$/, '')}
                        </span>
                        <span style={{ fontSize: '0.75rem', color: theme.textMuted, flexShrink: 0 }}>
                          {fmtSize(t.size)}
                        </span>
                      </div>
                    )
                  })}
                </div>
              ))}
            </div>
          </div>
        )
      })}
    </div>
  )
}
