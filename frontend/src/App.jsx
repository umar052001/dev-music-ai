import { useState, useEffect, useRef } from 'react'
import { ThemeProvider, useTheme } from './context/ThemeContext'
import { PlayerProvider, usePlayer } from './context/PlayerContext'
import Header from './components/Header'
import Search from './components/Search'
import AllSongs from './components/AllSongs'
import Library from './components/Library'
import Activity from './components/Activity'
import BatchImport from './components/BatchImport'
import PlayerBar from './components/PlayerBar'
import ThemeAdjuster from './components/ThemeAdjuster'
import LLMSettings from './components/LLMSettings'
import MusicVisuals, { FloatingNotes } from './components/MusicVisuals'
import DownloadStatus from './components/DownloadStatus'
import gsap from 'gsap'

function AppInner() {
  const { theme } = useTheme()
  const { current } = usePlayer()
  const [tab, setTab] = useState('search')
  const [adjusterOpen, setAdjusterOpen] = useState(false)
  const [llmOpen, setLlmOpen] = useState(false)
  const [llmAvailable, setLlmAvailable] = useState(true)
  const [llmChecked, setLlmChecked] = useState(false)
  const contentRef = useRef(null)

  // Check LLM availability to decide whether to show AI tabs
  useEffect(() => {
    fetch('/api/llm/status')
      .then(r => r.json())
      .then(d => { setLlmAvailable(!!d.available); setLlmChecked(true) })
      .catch(() => { setLlmAvailable(false); setLlmChecked(true) })
  }, [llmOpen])

  useEffect(() => {
    if (contentRef.current) {
      gsap.fromTo(contentRef.current,
        { opacity: 0, y: 12 },
        { opacity: 1, y: 0, duration: 0.4, ease: 'power2.out' }
      )
    }
  }, [tab])

  const allTabs = [
    { id: 'search', label: 'Search', ai: false },
    { id: 'batch', label: 'Batch Import', ai: true },
    { id: 'all', label: 'All Songs', ai: false },
    { id: 'library', label: 'Library', ai: false },
    { id: 'activity', label: 'Activity', ai: false },
  ]
  // Hide AI tabs when no LLM provider is available
  const tabs = allTabs.filter(t => !t.ai || llmAvailable)

  useEffect(() => {
    // If active tab is hidden, reset to search
    if (llmChecked && !llmAvailable && tab !== 'search' && !['all', 'library', 'activity'].includes(tab)) {
      setTab('search')
    }
  }, [llmChecked, llmAvailable, tab])

  return (
    <div style={{ display: 'flex', flexDirection: 'column', minHeight: '100vh' }}>
      <Header />
      <FloatingNotes />
      <MusicVisuals />

      <main style={{
        flex: 1, padding: '1.5rem', paddingBottom: current ? '7rem' : '1.5rem',
        maxWidth: 1200, width: '100%', margin: '0 auto', position: 'relative', zIndex: 2,
        transition: 'padding 0.3s',
      }}>
        <div style={{ display: 'flex', gap: '0.4rem', marginBottom: '1.5rem', flexWrap: 'wrap', alignItems: 'center' }} role="tablist" aria-label="App sections">
          {tabs.map(t => (
            <button
              key={t.id}
              role="tab"
              aria-selected={tab === t.id}
              aria-controls={`panel-${t.id}`}
              onClick={() => setTab(t.id)}
              title={t.ai && !llmAvailable ? 'AI unavailable — configure in Settings' : ''}
              style={{
                padding: '0.5rem 1.1rem', fontSize: '0.9rem', fontWeight: tab === t.id ? 600 : 500,
                background: tab === t.id ? theme.text : 'transparent',
                color: tab === t.id ? theme.bg : theme.textSecondary,
                border: `1px solid ${tab === t.id ? theme.text : theme.border}`,
                borderRadius: 999, cursor: 'pointer', transition: 'all 0.2s',
                opacity: t.ai && !llmAvailable ? 0.4 : 1,
                outline: 'none',
              }}
              onFocus={e => { e.target.style.boxShadow = `0 0 0 3px ${theme.accent}44` }}
              onBlur={e => { e.target.style.boxShadow = 'none' }}
              disabled={t.ai && !llmAvailable}
            >
              {t.label}
            </button>
          ))}
          <button onClick={() => setAdjusterOpen(true)} aria-label="Open theme settings" style={{
            marginLeft: 'auto', padding: '0.5rem 1rem', fontSize: '0.9rem',
            background: 'transparent', color: theme.textSecondary,
            border: `1px solid ${theme.border}`, borderRadius: 999, cursor: 'pointer',
            transition: 'all 0.2s', outline: 'none',
          }}
            onFocus={e => { e.target.style.boxShadow = `0 0 0 3px ${theme.accent}44` }}
            onBlur={e => { e.target.style.boxShadow = 'none' }}
          >
            ⚙ Theme
          </button>
          <button onClick={() => setLlmOpen(true)} aria-label={`AI settings (${llmAvailable ? 'available' : 'off'})`} aria-pressed={llmOpen} style={{
            padding: '0.5rem 1rem', fontSize: '0.9rem',
            background: llmAvailable ? theme.surface : 'transparent',
            color: llmAvailable ? theme.text : theme.textMuted,
            border: `1px solid ${theme.border}`, borderRadius: 999, cursor: 'pointer',
            transition: 'all 0.2s', outline: 'none',
          }}
            onFocus={e => { e.target.style.boxShadow = `0 0 0 3px ${theme.accent}44` }}
            onBlur={e => { e.target.style.boxShadow = 'none' }}
          >
            ✦ AI{llmAvailable ? '' : ' (off)'}
          </button>
        </div>

        <div ref={contentRef}>
          {tab === 'search' && <div id="panel-search" role="tabpanel"><Search /></div>}
          {tab === 'batch' && llmAvailable && <div id="panel-batch" role="tabpanel"><BatchImport /></div>}
          {tab === 'all' && <div id="panel-all" role="tabpanel"><AllSongs /></div>}
          {tab === 'library' && <div id="panel-library" role="tabpanel"><Library /></div>}
          {tab === 'activity' && <div id="panel-activity" role="tabpanel"><Activity /></div>}
        </div>

        {/* Persistent download status (server-backed, survives refresh/tab switch) */}
        <div style={{ marginTop: '1.5rem' }}>
          <DownloadStatus compact />
        </div>
      </main>

      <PlayerBar />
      <ThemeAdjuster open={adjusterOpen} onClose={() => setAdjusterOpen(false)} />
      <LLMSettings open={llmOpen} onClose={() => setLlmOpen(false)} />
    </div>
  )
}

export default function App() {
  return (
    <ThemeProvider>
      <PlayerProvider>
        <AppInner />
      </PlayerProvider>
    </ThemeProvider>
  )
}

