import { useState, useRef, useEffect } from 'react'
import { useTheme } from '../context/ThemeContext'
import { usePlayer } from '../context/PlayerContext'
import gsap from 'gsap'
import LibraryPlaylist from './LibraryPlaylist'

const QUICK = [
  'Pakistani classical music', 'Atif Aslam', 'Bilal Saeed',
  'Murtaza Qizilbash nasheed', 'Arabic nasheed', 'Pakistani OST drama',
  'Nusrat Fateh Ali Khan', 'chill desi music',
]

const PAGE_SIZE = 10

export default function Search() {
  const { theme } = useTheme()
  const { play, logAct } = usePlayer()
  const [query, setQuery] = useState('')
  const [org, setOrg] = useState('artist_album')
  const [allResults, setAllResults] = useState([])
  const [visibleCount, setVisibleCount] = useState(PAGE_SIZE)
  const [loading, setLoading] = useState(false)
  const [loadingMore, setLoadingMore] = useState(false)
  const [hasSearched, setHasSearched] = useState(false)
  const [downloading, setDownloading] = useState(new Set())
  const [suggestions, setSuggestions] = useState([])
  const [sugLoading, setSugLoading] = useState(true)
  const [playlistQuery, setPlaylistQuery] = useState('')
  const [playlist, setPlaylist] = useState(null)
  const [playlistLoading, setPlaylistLoading] = useState(false)
  const listRef = useRef(null)

  useEffect(() => {
    fetch('/api/suggestions')
      .then(r => r.json())
      .then(d => { setSuggestions(d.suggestions || []); setSugLoading(false) })
      .catch(() => setSugLoading(false))
  }, [])

  useEffect(() => {
    if (allResults.length && listRef.current) {
      gsap.fromTo(
        listRef.current.children,
        { opacity: 0, y: 20, scale: 0.97 },
        { opacity: 1, y: 0, scale: 1, duration: 0.4, stagger: 0.05, ease: 'power2.out' }
      )
    }
  }, [allResults, visibleCount])

  const doSearch = async (q) => {
    const searchQ = q || query
    if (!searchQ.trim()) return
    setLoading(true)
    setHasSearched(true)
    setAllResults([])
    setVisibleCount(PAGE_SIZE)
    logAct('search', null, '', searchQ)
    try {
      const res = await fetch('/api/search', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ query: searchQ.trim(), limit: 50 }),
      })
      const data = await res.json()
      setAllResults(data.results || [])
    } catch {
      setAllResults([])
    } finally {
      setLoading(false)
    }
  }

  const loadMore = async () => {
    if (allResults.length >= visibleCount) {
      setVisibleCount(v => v + PAGE_SIZE)
      return
    }
    setLoadingMore(true)
    const q = query.trim()
    try {
      const res = await fetch('/api/search', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ query: q, limit: 50 }),
      })
      const data = await res.json()
      setAllResults(prev => {
        const seen = new Set(prev.map(p => p.id))
        const fresh = (data.results || []).filter(r => !seen.has(r.id) && r.id)
        return [...prev, ...fresh]
      })
      setVisibleCount(v => v + PAGE_SIZE)
    } catch {
    } finally {
      setLoadingMore(false)
    }
  }

  const downloadTrack = async (r) => {
    setDownloading(prev => new Set([...prev, r.id]))
    logAct('download', { name: r.title }, r.uploader, r.url)
    try {
      await fetch('/api/download', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ url: r.url, organization: org }),
      })
      setTimeout(() => setDownloading(prev => { const n = new Set(prev); n.delete(r.id); return n }), 3000)
    } catch {
      setDownloading(prev => { const n = new Set(prev); n.delete(r.id); return n })
    }
  }

  const downloadAll = async () => {
    for (const r of allResults) {
      await downloadTrack(r)
      await new Promise(ok => setTimeout(ok, 500))
    }
  }

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

  const inputStyle = {
    background: theme.inputBg, color: theme.text,
    border: `2px solid ${theme.inputBorder}`, padding: '0.7rem 1rem',
    borderRadius: 8,
    fontSize: '1rem', width: '100%', outline: 'none',
    transition: 'border-color 0.2s, box-shadow 0.2s',
  }

  const visibleResults = allResults.slice(0, visibleCount)
  const hasMore = visibleCount < allResults.length

  return (
    <div>
      {/* Sticky search bar */}
      <div style={{
        position: 'sticky', top: 0, zIndex: 50,
        background: theme.bg, padding: '0.5rem 0 1rem',
        borderBottom: `1px solid ${theme.border}`, marginBottom: '1rem',
      }}>
        <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }}>
          <input
            style={{ ...inputStyle, flex: 1, minWidth: 200 }}
            placeholder="Search songs, artists, or albums..."
            value={query}
            onChange={e => setQuery(e.target.value)}
            onKeyDown={e => e.key === 'Enter' && doSearch()}
            onFocus={e => { e.target.style.borderColor = theme.text; e.target.style.boxShadow = `0 0 0 3px ${theme.accent}22` }}
            onBlur={e => { e.target.style.borderColor = theme.inputBorder; e.target.style.boxShadow = 'none' }}
          />
          <button onClick={() => doSearch()} disabled={loading} style={{
            background: theme.text, color: theme.bg, border: 'none',
            padding: '0.7rem 1.5rem', borderRadius: 8, fontSize: '1rem', fontWeight: 700,
            cursor: loading ? 'wait' : 'pointer', opacity: loading ? 0.6 : 1,
            transition: 'opacity 0.2s',
          }}>
            {loading ? '...' : 'Search'}
          </button>
        </div>

        <div style={{ display: 'flex', gap: '1rem', marginTop: '0.7rem', flexWrap: 'wrap', alignItems: 'center' }}>
          <label style={{ fontSize: '0.85rem', color: theme.textMuted, display: 'flex', alignItems: 'center', gap: '0.4rem' }}>
            Organize:
            <select style={{ ...inputStyle, width: 'auto', padding: '0.4rem', fontSize: '0.85rem', borderRadius: 6 }} value={org} onChange={e => setOrg(e.target.value)}>
              <option value="artist_album">Artist / Album</option>
              <option value="artist_only">Artist Only</option>
            </select>
          </label>
          {allResults.length > 0 && (
            <button onClick={downloadAll} style={{
              marginLeft: 'auto', background: theme.text, color: theme.bg,
              border: 'none', padding: '0.5rem 1rem', borderRadius: 6, fontSize: '0.85rem',
              fontWeight: 700, cursor: 'pointer',
            }}>
              Download All ({allResults.length})
            </button>
          )}
        </div>
      </div>

      <div style={{ display: 'flex', gap: '0.4rem', flexWrap: 'wrap', marginBottom: '1rem' }}>
        {QUICK.map(q => (
          <button key={q} onClick={() => { setQuery(q); setTimeout(() => doSearch(q), 0) }} style={{
            background: theme.surface, color: theme.textSecondary,
            border: `1px solid ${theme.border}`, padding: '0.35rem 0.7rem',
            fontSize: '0.8rem', cursor: 'pointer', transition: 'all 0.15s',
          }}
            onMouseEnter={e => { e.target.style.background = theme.text; e.target.style.color = theme.bg }}
            onMouseLeave={e => { e.target.style.background = theme.surface; e.target.style.color = theme.textSecondary }}
          >
            {q}
          </button>
        ))}
      </div>

      {/* For You: AI suggestions + playlist builders, shown before a search */}
      {!hasSearched && (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(320px, 1fr))', gap: '1rem', marginBottom: '1.5rem' }}>
          {/* AI suggestions */}
          <div style={{ background: theme.cardBg, border: `1px solid ${theme.border}`, borderRadius: 12, padding: '1.1rem' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '0.7rem' }}>
              <span style={{ fontSize: '1rem', fontWeight: 600 }}>Suggested for you</span>
              <button onClick={() => {
                setSugLoading(true)
                fetch('/api/suggestions').then(r => r.json()).then(d => { setSuggestions(d.suggestions || []); setSugLoading(false) })
              }} style={{
                background: 'none', border: `1px solid ${theme.border}`, color: theme.textSecondary,
                padding: '0.25rem 0.6rem', fontSize: '0.8rem', cursor: 'pointer', borderRadius: 6,
              }}>↻ Refresh</button>
            </div>
            {sugLoading && <div style={{ padding: '0.6rem', color: theme.textMuted, fontSize: '0.9rem' }}>Analyzing your taste...</div>}
            {!sugLoading && suggestions.length === 0 && (
              <div style={{ padding: '0.6rem', color: theme.textMuted, fontSize: '0.9rem' }}>
                Play some music to get personalized suggestions
              </div>
            )}
            {suggestions.map((s, i) => (
              <div key={i} onClick={() => { setQuery(s.query); setTimeout(() => doSearch(s.query), 0) }} style={{
                display: 'flex', alignItems: 'center', gap: '0.7rem',
                padding: '0.5rem 0.6rem', cursor: 'pointer',
                borderBottom: i < suggestions.length - 1 ? `1px solid ${theme.border}` : 'none',
                borderRadius: 6,
              }}
                onMouseEnter={e => e.currentTarget.style.background = theme.surfaceHover}
                onMouseLeave={e => e.currentTarget.style.background = 'transparent'}
              >
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{ fontSize: '0.92rem', fontWeight: 500, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{s.query}</div>
                  <div style={{ fontSize: '0.75rem', color: theme.textMuted, marginTop: 1 }}>{s.reason}</div>
                </div>
                <span style={{ fontSize: '0.85rem', color: theme.textMuted }}>→</span>
              </div>
            ))}
          </div>

          {/* Playlist generator (search & download) */}
          <div style={{ background: theme.cardBg, border: `1px solid ${theme.border}`, borderRadius: 12, padding: '1.1rem' }}>
            <div style={{ fontSize: '1rem', fontWeight: 600, marginBottom: '0.7rem' }}>Generate playlist</div>
            <div style={{ display: 'flex', gap: '0.5rem' }}>
              <input
                style={{ flex: 1, background: theme.inputBg, color: theme.text, border: `2px solid ${theme.inputBorder}`, padding: '0.6rem 0.8rem', fontSize: '0.9rem', outline: 'none', borderRadius: 8 }}
                placeholder="Describe a mood, e.g. 'chill evening'"
                value={playlistQuery}
                onChange={e => setPlaylistQuery(e.target.value)}
                onKeyDown={e => e.key === 'Enter' && generatePlaylist()}
                onFocus={e => e.target.style.borderColor = theme.text}
                onBlur={e => e.target.style.borderColor = theme.inputBorder}
              />
              <button onClick={generatePlaylist} disabled={playlistLoading} style={{
                background: theme.text, color: theme.bg, border: 'none',
                padding: '0.6rem 1rem', fontSize: '0.85rem', fontWeight: 700,
                borderRadius: 8, cursor: playlistLoading ? 'wait' : 'pointer', opacity: playlistLoading ? 0.6 : 1,
              }}>
                {playlistLoading ? '...' : 'Generate'}
              </button>
            </div>
            {playlist && (
              <div style={{ marginTop: '0.8rem', padding: '0.8rem', background: theme.surface, border: `1px solid ${theme.border}`, borderRadius: 8 }}>
                <div style={{ fontSize: '1rem', fontWeight: 600 }}>{playlist.name}</div>
                <div style={{ fontSize: '0.8rem', color: theme.textMuted, marginTop: '0.3rem' }}>
                  {playlist.track_count} tracks · {playlist.mood}
                </div>
                <button onClick={() => { setQuery(playlist.query); setTimeout(() => doSearch(playlist.query), 0) }} style={{
                  marginTop: '0.6rem', background: theme.text, color: theme.bg,
                  border: 'none', padding: '0.5rem 1rem', fontSize: '0.82rem', fontWeight: 700, borderRadius: 8, cursor: 'pointer',
                }}>
                  Search & Download
                </button>
              </div>
            )}
          </div>
        </div>
      )}

      {/* AI playlist from your downloaded music, always accessible */}
      <div style={{ marginBottom: '1.5rem' }}>
        <LibraryPlaylist />
      </div>

      {loading && (
        <div style={{ textAlign: 'center', padding: '3rem', color: theme.textMuted, fontSize: '1rem' }}>
          <div style={{
            width: 32, height: 32, border: `3px solid ${theme.border}`,
            borderTopColor: theme.text, margin: '0 auto 1rem',
            animation: 'spin 0.8s linear infinite',
          }} />
          Searching...
        </div>
      )}

      {!loading && hasSearched && allResults.length === 0 && (
        <div style={{ textAlign: 'center', padding: '3rem', color: theme.textMuted, fontSize: '1rem' }}>
          No results found
        </div>
      )}

      <div ref={listRef}>
        {visibleResults.map((r, i) => (
          <div key={r.id + i} style={{
            display: 'flex', alignItems: 'center', gap: '0.8rem',
            padding: '0.7rem 0.8rem', borderRadius: 8, marginBottom: '0.4rem',
            background: downloading.has(r.id) ? theme.surfaceHover : theme.cardBg,
            border: `1px solid ${theme.border}`,
            cursor: 'default', transition: 'background 0.15s',
          }}
            onMouseEnter={e => !downloading.has(r.id) && (e.currentTarget.style.background = theme.surfaceHover)}
            onMouseLeave={e => !downloading.has(r.id) && (e.currentTarget.style.background = theme.cardBg)}
          >
            <span style={{ width: 28, textAlign: 'center', color: theme.textMuted, fontSize: '0.85rem' }}>
              {String(i + 1).padStart(2, '0')}
            </span>
            <img
              src={`https://img.youtube.com/vi/${r.id}/mqdefault.jpg`}
              alt="" loading="lazy"
              style={{ width: 50, height: 50, objectFit: 'cover', borderRadius: 6, flexShrink: 0 }}
            />
            <div style={{ flex: 1, minWidth: 0 }}>
              <div style={{ fontSize: '1rem', fontWeight: 500, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                {r.clean_title || r.title}
              </div>
              <div style={{ fontSize: '0.85rem', color: theme.textMuted, marginTop: 2 }}>
                {r.uploader || 'Unknown'}
              </div>
            </div>
            <span style={{ fontSize: '0.85rem', color: theme.textMuted, flexShrink: 0 }}>
              {r.duration > 0 ? `${Math.floor(r.duration / 60)}:${String(Math.floor(r.duration % 60)).padStart(2, '0')}` : '—'}
            </span>
            <div style={{ display: 'flex', gap: '0.4rem', flexShrink: 0 }}>
              <button onClick={() => play(r, visibleResults)} style={{
                background: 'none', border: `1px solid ${theme.border}`,
                color: theme.text, padding: '0.4rem 0.6rem', borderRadius: 6, cursor: 'pointer',
                fontSize: '0.9rem', transition: 'all 0.15s',
              }}
                onMouseEnter={e => { e.target.style.background = theme.text; e.target.style.color = theme.bg }}
                onMouseLeave={e => { e.target.style.background = 'none'; e.target.style.color = theme.text }}
              >▶</button>
              <button onClick={() => downloadTrack(r)} disabled={downloading.has(r.id)} style={{
                background: 'none', border: `1px solid ${theme.border}`,
                color: theme.text, padding: '0.4rem 0.6rem', borderRadius: 6, cursor: 'pointer',
                fontSize: '0.9rem', opacity: downloading.has(r.id) ? 0.4 : 1,
                transition: 'all 0.15s',
              }}
                onMouseEnter={e => { if (!downloading.has(r.id)) { e.target.style.background = theme.text; e.target.style.color = theme.bg } }}
                onMouseLeave={e => { e.target.style.background = 'none'; e.target.style.color = theme.text }}
              >{downloading.has(r.id) ? '...' : '⬇'}</button>
            </div>
          </div>
        ))}
      </div>

      {allResults.length > 0 && !hasMore && (
        <div style={{ textAlign: 'center', padding: '1.5rem', color: theme.textMuted, fontSize: '0.85rem' }}>
          You're all caught up — {allResults.length} results
        </div>
      )}

      {allResults.length > 0 && hasMore && (
        <div style={{ textAlign: 'center', marginTop: '1rem' }}>
          <button onClick={loadMore} disabled={loadingMore} style={{
            background: theme.surface, color: theme.text, border: `1px solid ${theme.border}`,
            padding: '0.7rem 2rem', borderRadius: 8, fontSize: '0.95rem', fontWeight: 600,
            cursor: loadingMore ? 'wait' : 'pointer',
            transition: 'all 0.15s',
          }}
            onMouseEnter={e => { e.target.style.background = theme.text; e.target.style.color = theme.bg }}
            onMouseLeave={e => { e.target.style.background = theme.surface; e.target.style.color = theme.text }}
          >
            {loadingMore ? 'Loading…' : `Load more results (${allResults.length - visibleCount} remaining)`}
          </button>
        </div>
      )}

      <style>{`@keyframes spin{to{transform:rotate(360deg)}}`}</style>
    </div>
  )
}
