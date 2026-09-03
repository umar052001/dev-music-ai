import { useTheme } from '../context/ThemeContext'
import { usePlayer } from '../context/PlayerContext'
import { useRef, useEffect } from 'react'
import gsap from 'gsap'

export default function PlayerBar() {
  const { theme } = useTheme()
  const { current, playing, progress, duration, volume, loop, shuffle, toggle, seek, changeVolume, prev, next, hasPrev, hasNext, toggleLoop, toggleShuffle } = usePlayer()
  const barRef = useRef(null)
  const visible = !!current

  useEffect(() => {
    if (visible && barRef.current) {
      gsap.fromTo(
        barRef.current,
        { y: 60, opacity: 0, xPercent: -50 },
        { y: 0, opacity: 1, xPercent: -50, duration: 0.5, ease: 'power3.out' }
      )
    }
  }, [visible])

  const fmt = (sec) => {
    if (!sec || sec <= 0) return '0:00'
    const m = Math.floor(sec / 60)
    const s = Math.floor(sec % 60)
    return m + ':' + String(s).padStart(2, '0')
  }

  if (!visible) return null

  const btnBase = {
    background: 'none', border: 'none', cursor: 'pointer',
    color: theme.text, fontSize: '1rem', padding: '0.3rem',
    opacity: 0.5, transition: 'opacity 0.15s',
  }

  return (
    <div ref={barRef} style={{
      position: 'fixed', bottom: 12, left: '50%', transform: 'translateX(-50%)',
      width: 'calc(100% - 2rem)', maxWidth: 1100, zIndex: 200,
      background: theme.playerBg, border: `1px solid ${theme.border}`,
      borderRadius: 14, boxShadow: `0 8px 30px ${theme.shadow}`,
      padding: '0.7rem 1.5rem',
      display: 'grid', gridTemplateColumns: '1fr 3fr 1fr', alignItems: 'center', gap: '1rem',
    }}>
      {/* Info */}
      <div style={{ minWidth: 0 }}>
        <div style={{ fontSize: '0.95rem', fontWeight: 600, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
          {current.clean_title || current.title || current.name?.replace(/\.[^.]+$/, '') || '—'}
        </div>
        <div style={{ fontSize: '0.8rem', color: theme.textMuted, marginTop: 2 }}>
          {current.artist || current.uploader || 'Unknown'}
        </div>
      </div>

      {/* Controls */}
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '0.25rem' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.3rem' }}>
          <button onClick={toggleShuffle} style={{ ...btnBase, opacity: shuffle ? 1 : 0.4, fontSize: '0.85rem', color: shuffle ? theme.text : theme.textMuted }} title="Shuffle (S)">⇄</button>
          <button onClick={prev} disabled={!hasPrev} style={{ ...btnBase, opacity: hasPrev ? 0.8 : 0.2 }}>⏮</button>
          <button onClick={toggle} style={{
            width: 44, height: 44, background: theme.text, color: theme.bg,
            border: 'none', borderRadius: '50%', fontSize: '1.2rem', cursor: 'pointer',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
          }}>{playing ? '⏸' : '▶'}</button>
          <button onClick={next} disabled={!hasNext} style={{ ...btnBase, opacity: hasNext ? 0.8 : 0.2 }}>⏭</button>
          <button onClick={toggleLoop} style={{ ...btnBase, opacity: loop ? 1 : 0.4, fontSize: '0.85rem', color: loop ? theme.text : theme.textMuted }} title="Loop (L)">⟳</button>
        </div>

        <div style={{ width: '100%', maxWidth: 500, display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
          <span style={{ fontSize: '0.7rem', color: theme.textMuted, width: 38, textAlign: 'right', fontVariantNumeric: 'tabular-nums' }}>
            {fmt(duration * progress / 100)}
          </span>
          <div onClick={e => {
            const rect = e.currentTarget.getBoundingClientRect()
            seek(((e.clientX - rect.left) / rect.width) * 100)
          }} style={{
            flex: 1, height: 5, background: theme.border, cursor: 'pointer', position: 'relative',
          }}>
            <div style={{ height: '100%', background: theme.text, width: `${progress}%`, transition: 'width 0.3s linear' }} />
          </div>
          <span style={{ fontSize: '0.7rem', color: theme.textMuted, width: 38, fontVariantNumeric: 'tabular-nums' }}>
            {fmt(duration)}
          </span>
        </div>
      </div>

      {/* Volume + shortcuts hint */}
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-end', gap: '0.3rem' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.4rem' }}>
          <span style={{ fontSize: '0.8rem' }}>🔊</span>
          <input type="range" min={0} max={100} value={volume} onChange={e => changeVolume(Number(e.target.value))} style={{
            width: 70, accentColor: theme.text,
          }} />
        </div>
        <div style={{ fontSize: '0.6rem', color: theme.textMuted, opacity: 0.5 }}>
          Space: play/pause · Shift+←→: skip · L: loop · S: shuffle
        </div>
      </div>
    </div>
  )
}
