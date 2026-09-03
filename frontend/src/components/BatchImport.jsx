import { useState, useRef, useEffect } from 'react'
import { useTheme } from '../context/ThemeContext'
import gsap from 'gsap'

export default function BatchImport() {
  const { theme } = useTheme()
  const [text, setText] = useState('')
  const [entries, setEntries] = useState([])
  const [parsing, setParsing] = useState(false)
  const [phase, setPhase] = useState('') // '', 'contacting', 'structuring', 'done'
  const [parseError, setParseError] = useState('')
  const [llmStatus, setLlmStatus] = useState(null)
  const [job, setJob] = useState(null)
  const [org, setOrg] = useState('artist_album')
  const pollRef = useRef(null)
  const listRef = useRef(null)
  const textareaRef = useRef(null)

  useEffect(() => {
    fetch('/api/llm/status').then(r => r.json()).then(setLlmStatus).catch(() => {})
  }, [])

  useEffect(() => {
    if (entries.length && listRef.current) {
      gsap.fromTo(listRef.current.children,
        { opacity: 0, y: 16 },
        { opacity: 1, y: 0, duration: 0.35, stagger: 0.04, ease: 'power2.out' }
      )
    }
  }, [entries])

  useEffect(() => {
    return () => { if (pollRef.current) clearInterval(pollRef.current) }
  }, [])

  const parse = async () => {
    if (!text.trim()) return
    setParsing(true)
    setParseError('')
    setEntries([])
    setPhase('contacting')
    setJob(null)
    try {
      const res = await fetch('/api/batch/parse', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ text }),
      })
      const data = await res.json()
      setPhase('done')
      setEntries(data.entries || [])
      if (data.error) {
        setParseError(data.error)
        setPhase('')
      }
    } catch (e) {
      setParseError('Could not parse. Is the AI provider reachable? Check the ✦ AI settings.')
      setPhase('')
    } finally {
      setParsing(false)
    }
  }

  const startDownload = async () => {
    const valid = entries.filter(e => e.title || e.artist || e.url)
    if (!valid.length) return
    try {
      const res = await fetch('/api/batch/run', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ entries: valid, organization: org }),
      })
      if (!res.ok) {
        const err = await res.json()
        setParseError(err.error || 'Could not start download')
        return
      }
      // Poll for status
      if (pollRef.current) clearInterval(pollRef.current)
      pollRef.current = setInterval(async () => {
        try {
          const r = await fetch('/api/batch/status')
          const d = await r.json()
          setJob(d)
          if (d.status === 'done' || d.status === 'error') {
            clearInterval(pollRef.current)
            pollRef.current = null
          }
        } catch {}
      }, 1500)
    } catch {
      setParseError('Could not start download')
    }
  }

  const updateEntry = (i, field, value) => {
    setEntries(prev => prev.map((e, idx) => idx === i ? { ...e, [field]: value } : e))
  }

  const removeEntry = (i) => {
    setEntries(prev => prev.filter((_, idx) => idx !== i))
  }

  const inputStyle = {
    background: theme.inputBg, color: theme.text,
    border: `2px solid ${theme.inputBorder}`, padding: '0.6rem 0.8rem',
    borderRadius: 8, fontSize: '0.9rem', outline: 'none', width: '100%',
    transition: 'border-color 0.2s',
  }

  const pct = job && job.total > 0 ? Math.round(((job.done + job.failed) / job.total) * 100) : 0
  const running = job && (job.status === 'running' || job.status === 'queued')

  return (
    <div>
      {/* Paste area */}
      <div style={{ background: theme.cardBg, border: `1px solid ${theme.border}`, borderRadius: 12, padding: '1.2rem', marginBottom: '1rem' }}>
        <div style={{ fontSize: '1rem', fontWeight: 600, marginBottom: '0.6rem' }}>
          Paste songs (any format)
        </div>
        <div style={{ fontSize: '0.8rem', color: theme.textMuted, marginBottom: '0.8rem' }}>
          Paste a messy list of songs — links, titles, "Title — Artist", whatever. AI will structure it.
        </div>
        <textarea
          ref={textareaRef}
          value={text}
          onChange={e => setText(e.target.value)}
          placeholder={`[Raaz-e-Fitna](https://music.youtube.com/watch?v=...) — Asfar Hussain & Xulfi\nHar Baar — Murtaza Qizilbash & Samar Jafri\nAwari - Soch\nDemons — Imagine Dragons\n...`}
          style={{
            ...inputStyle, minHeight: 160, resize: 'vertical', fontFamily: 'inherit',
            lineHeight: 1.6, padding: '0.8rem 1rem',
          }}
          onFocus={e => e.target.style.borderColor = theme.text}
          onBlur={e => e.target.style.borderColor = theme.inputBorder}
        />
        <div style={{ display: 'flex', gap: '0.5rem', marginTop: '0.8rem', flexWrap: 'wrap' }}>
          <button onClick={parse} disabled={parsing || !text.trim()} style={{
            background: theme.text, color: theme.bg, border: 'none',
            padding: '0.65rem 1.5rem', borderRadius: 8, fontSize: '0.95rem', fontWeight: 700,
            cursor: (parsing || !text.trim()) ? 'wait' : 'pointer', opacity: (parsing || !text.trim()) ? 0.6 : 1,
          }}>
            {parsing && phase === 'contacting' ? 'Contacting AI…' :
             parsing && phase === 'structuring' ? 'Structuring…' : 'AI: Parse & Preview'}
          </button>
          <button onClick={() => setText('')} style={{
            background: 'transparent', color: theme.textSecondary, border: `1px solid ${theme.border}`,
            padding: '0.65rem 1.2rem', borderRadius: 8, fontSize: '0.9rem', cursor: 'pointer',
          }}>Clear</button>
          {parseError && (
            <span style={{ fontSize: '0.85rem', color: 'tomato', alignSelf: 'center' }}>{parseError}</span>
          )}
          {parsing && (
            <span style={{ fontSize: '0.8rem', color: theme.textMuted, alignSelf: 'center' }}>
              {phase === 'contacting' ? 'Talking to AI — first cloud call can take 10–60s…' : 'Reading your list…'}
            </span>
          )}
        </div>
      </div>

      {/* AI availability warning */}
      {llmStatus && !llmStatus.available && (
        <div style={{
          background: theme.surface, border: `1px solid tomato`, borderRadius: 12,
          padding: '0.9rem 1.2rem', marginBottom: '1rem', fontSize: '0.9rem', color: theme.text,
        }}>
          <b>⚠ AI unavailable.</b> The AI provider is not reachable, so parsing/structuring won't work.
          Open the <b>✦ AI</b> button to configure a provider (Ollama cloud, OpenAI, Groq, Claude, Gemini).
        </div>
      )}

      {/* Progress */}
      {job && (
        <div style={{ background: theme.cardBg, border: `1px solid ${theme.border}`, borderRadius: 12, padding: '1.2rem', marginBottom: '1rem' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '0.6rem', flexWrap: 'wrap', gap: '0.5rem' }}>
            <span style={{ fontSize: '1rem', fontWeight: 600 }}>
              Downloading {running ? '…' : job.status === 'done' ? 'complete' : ''}
            </span>
            <span style={{ fontSize: '0.85rem', color: theme.textMuted }}>
              {job.done}/{job.total} done{job.failed > 0 ? ` · ${job.failed} failed` : ''}
            </span>
          </div>
          <div style={{ height: 10, background: theme.surface, borderRadius: 5, overflow: 'hidden' }}>
            <div style={{
              height: '100%', background: theme.text, borderRadius: 5, width: `${pct}%`,
              transition: 'width 0.5s ease',
            }} />
          </div>
          {job.current && running && (
            <div style={{ fontSize: '0.85rem', color: theme.textMuted, marginTop: '0.5rem' }}>
              Now: {job.current}
            </div>
          )}
        </div>
      )}

      {/* Structured preview */}
      {entries.length > 0 && (
        <div style={{ background: theme.cardBg, border: `1px solid ${theme.border}`, borderRadius: 12, padding: '1.2rem' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '0.8rem', flexWrap: 'wrap', gap: '0.5rem' }}>
            <span style={{ fontSize: '1rem', fontWeight: 600 }}>Preview ({entries.length})</span>
            <select
              value={org}
              onChange={e => setOrg(e.target.value)}
              style={{
                background: theme.surface, color: theme.text, border: `1px solid ${theme.border}`,
                padding: '0.4rem 0.7rem', borderRadius: 8, fontSize: '0.85rem', cursor: 'pointer',
              }}
            >
              <option value="artist_album">Artist / Album</option>
              <option value="artist_only">Artist Only</option>
            </select>
          </div>
          <div style={{ fontStyle: 'italic', fontSize: '0.8rem', color: theme.textMuted, marginBottom: '0.8rem' }}>
            Review & edit before downloading. Only rows with a title or artist will download.
            Rows without a URL are <b>auto-found</b> (searched) at download time.
          </div>

          <div ref={listRef}>
            {entries.map((e, i) => (
              <div key={i} style={{
                display: 'grid', gridTemplateColumns: '1.5fr 1fr 1.5fr auto', gap: '0.5rem',
                padding: '0.5rem 0', alignItems: 'center',
                borderBottom: i < entries.length - 1 ? `1px solid ${theme.border}` : 'none',
              }}>
                <input
                  value={e.title || ''}
                  onChange={ev => updateEntry(i, 'title', ev.target.value)}
                  placeholder="Title"
                  style={{ ...inputStyle, padding: '0.5rem 0.7rem' }}
                />
                <input
                  value={e.artist || ''}
                  onChange={ev => updateEntry(i, 'artist', ev.target.value)}
                  placeholder="Artist"
                  style={{ ...inputStyle, padding: '0.5rem 0.7rem' }}
                />
                <input
                  value={e.url || ''}
                  onChange={ev => updateEntry(i, 'url', ev.target.value)}
                  placeholder="https://..."
                  style={{ ...inputStyle, padding: '0.5rem 0.7rem' }}
                />
                <button onClick={() => removeEntry(i)} title="Remove" style={{
                  background: 'transparent', border: 'none', color: theme.textMuted,
                  fontSize: '1rem', cursor: 'pointer', padding: '0.4rem',
                }}>✕</button>
              </div>
            ))}
          </div>

          <div style={{ display: 'flex', gap: '0.5rem', marginTop: '1rem' }}>
            <button onClick={startDownload} disabled={running} style={{
              background: theme.text, color: theme.bg, border: 'none',
              padding: '0.7rem 1.5rem', borderRadius: 8, fontSize: '0.95rem', fontWeight: 700,
              cursor: running ? 'wait' : 'pointer', opacity: running ? 0.6 : 1,
            }}>
              {running ? 'Running…' : `Download All (${entries.length})`}
            </button>
            <button onClick={() => setEntries([])} disabled={running} style={{
              background: 'transparent', color: theme.textSecondary, border: `1px solid ${theme.border}`,
              padding: '0.7rem 1.2rem', borderRadius: 8, fontSize: '0.9rem', cursor: running ? 'wait' : 'pointer',
            }}>Discard</button>
          </div>
        </div>
      )}
    </div>
  )
}
