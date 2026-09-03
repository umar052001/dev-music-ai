import { useState, useEffect, useRef } from 'react'
import { useTheme } from '../context/ThemeContext'
import { usePlayer } from '../context/PlayerContext'
import gsap from 'gsap'

export default function AllSongs() {
  const { theme } = useTheme()
  const { play, current } = usePlayer()
  const [songs, setSongs] = useState([])
  const [loading, setLoading] = useState(true)
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const [limit, setLimit] = useState(10)
  const listRef = useRef(null)

  useEffect(() => {
    setLoading(true)
    fetch(`/api/all-songs?page=${page}&limit=${limit}`)
      .then(r => r.json())
      .then(d => {
        setSongs(d.songs || [])
        setTotal(d.count || 0)
        setLoading(false)
      })
      .catch(() => setLoading(false))
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }, [page, limit])

  useEffect(() => {
    if (songs.length && listRef.current) {
      gsap.fromTo(
        listRef.current.children,
        { opacity: 0, x: -20 },
        { opacity: 1, x: 0, duration: 0.3, stagger: 0.03, ease: 'power2.out' }
      )
    }
  }, [songs])

  const fmtSize = (b) => {
    if (b < 1024) return b + ' B'
    if (b < 1048576) return (b / 1024).toFixed(1) + ' KB'
    return (b / 1048576).toFixed(1) + ' MB'
  }

  const totalPages = Math.max(1, Math.ceil(total / limit))
  const pageNumbers = []
  const start = Math.max(1, page - 2)
  const end = Math.min(totalPages, start + 4)
  for (let i = start; i <= end; i++) pageNumbers.push(i)

  if (loading && songs.length === 0) {
    return (
      <div style={{ textAlign: 'center', padding: '3rem', color: theme.textMuted, fontSize: '1rem' }}>
        Loading songs...
      </div>
    )
  }

  if (!loading && total === 0) {
    return (
      <div style={{ textAlign: 'center', padding: '3rem', color: theme.textMuted, fontSize: '1rem' }}>
        No songs downloaded yet. Search and download some tracks!
      </div>
    )
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem', flexWrap: 'wrap', gap: '0.5rem' }}>
        <span style={{ fontSize: '1rem', fontWeight: 600 }}>All Songs ({total})</span>
        <select
          value={limit}
          onChange={e => { setLimit(Number(e.target.value)); setPage(1) }}
          style={{
            background: theme.surface, color: theme.text,
            border: `1px solid ${theme.border}`, padding: '0.35rem 0.6rem',
            fontSize: '0.85rem', cursor: 'pointer', outline: 'none',
          }}
        >
          {[10, 20, 50, 100].map(n => <option key={n} value={n}>{n} / page</option>)}
        </select>
      </div>

      <div ref={listRef}>
        {songs.map((s, i) => {
          const isPlaying = current && (current.path === s.path)
          const displayIndex = (page - 1) * limit + i + 1
          return (
            <div key={s.path} onClick={() => play(s, songs)} style={{
              display: 'flex', alignItems: 'center', gap: '0.8rem',
              padding: '0.65rem 0.8rem', borderRadius: 8, marginBottom: '0.4rem',
              background: isPlaying ? theme.surfaceHover : theme.cardBg,
              border: `1px solid ${theme.border}`,
              cursor: 'pointer', transition: 'all 0.15s',
            }}
              onMouseEnter={e => { if (!isPlaying) e.currentTarget.style.background = theme.surfaceHover }}
              onMouseLeave={e => { if (!isPlaying) e.currentTarget.style.background = theme.cardBg }}
            >
              <span style={{
                width: 30, textAlign: 'center', fontSize: '0.85rem',
                color: isPlaying ? theme.text : theme.textMuted, fontWeight: isPlaying ? 700 : 400,
              }}>
                {isPlaying ? '♪' : String(displayIndex).padStart(2, '0')}
              </span>
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{
                  fontSize: '1rem', fontWeight: isPlaying ? 600 : 400,
                  whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
                }}>
                  {s.name.replace(/\.[^.]+$/, '')}
                </div>
                <div style={{ fontSize: '0.8rem', color: theme.textMuted, marginTop: 1 }}>
                  {s.artist} {s.album !== 'Singles' ? `· ${s.album}` : ''}
                </div>
              </div>
              <span style={{ fontSize: '0.8rem', color: theme.textMuted, flexShrink: 0 }}>
                {fmtSize(s.size)}
              </span>
            </div>
          )
        })}
      </div>

      {/* Pagination controls */}
      {total > 0 && (
        <div style={{
          display: 'flex', alignItems: 'center', justifyContent: 'center',
          gap: '0.4rem', marginTop: '1.2rem', flexWrap: 'wrap',
        }}>
          <button
            onClick={() => setPage(p => Math.max(1, p - 1))}
            disabled={page <= 1}
            style={pageBtnStyle(theme, page <= 1)}
          >‹ Prev</button>

          {page > 3 && <span style={{ color: theme.textMuted, padding: '0 0.2rem' }}>…</span>}

          {pageNumbers.map(p => (
            <button
              key={p}
              onClick={() => setPage(p)}
              style={{
                ...pageBtnStyle(theme, false),
                background: p === page ? theme.text : theme.surface,
                color: p === page ? theme.bg : theme.textSecondary,
                fontWeight: p === page ? 700 : 400,
              }}
            >{p}</button>
          ))}

          {page < totalPages - 2 && <span style={{ color: theme.textMuted, padding: '0 0.2rem' }}>…</span>}

          <button
            onClick={() => setPage(p => Math.min(totalPages, p + 1))}
            disabled={page >= totalPages}
            style={pageBtnStyle(theme, page >= totalPages)}
          >Next ›</button>

          <span style={{
            marginLeft: '0.6rem', fontSize: '0.8rem', color: theme.textMuted,
          }}>
            Page {page} of {totalPages}
          </span>
        </div>
      )}
    </div>
  )
}

const pageBtnStyle = (theme, disabled) => ({
  background: theme.surface, color: theme.textSecondary,
  border: `1px solid ${theme.border}`, padding: '0.45rem 0.8rem',
  borderRadius: 8, fontSize: '0.85rem', cursor: disabled ? 'default' : 'pointer',
  opacity: disabled ? 0.4 : 1, transition: 'all 0.15s',
})
