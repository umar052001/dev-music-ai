import { usePlayer } from '../context/PlayerContext'
import { useEffect, useRef } from 'react'
import gsap from 'gsap'

function Note({ delay, x, size }) {
  return (
    <div style={{
      position: 'absolute', left: `${x}%`, bottom: -20,
      fontSize: size, opacity: 0,
      color: 'var(--textMuted)',
      pointerEvents: 'none',
      animation: `floatNote 4s ${delay}s ease-in-out infinite`,
    }}>
      ♪
    </div>
  )
}

function WaveBars({ playing }) {
  const barsRef = useRef([])

  useEffect(() => {
    if (playing) {
      barsRef.current.forEach((bar, i) => {
        if (!bar) return
        gsap.to(bar, {
          scaleY: () => 0.3 + Math.random() * 1.8,
          duration: 0.4 + Math.random() * 0.3,
          repeat: -1,
          yoyo: true,
          ease: 'sine.inOut',
          delay: i * 0.05,
        })
      })
    } else {
      barsRef.current.forEach(bar => {
        if (!bar) return
        gsap.killTweensOf(bar)
        gsap.to(bar, { scaleY: 0.2, duration: 0.5, ease: 'power2.out' })
      })
    }
  }, [playing])

  return (
    <div style={{ display: 'flex', alignItems: 'flex-end', gap: 2, height: 20 }}>
      {Array.from({ length: 5 }).map((_, i) => (
        <div key={i} ref={el => barsRef.current[i] = el} style={{
          width: 3, height: '100%', background: 'var(--textMuted)',
          transformOrigin: 'bottom', transform: 'scaleY(0.2)',
          opacity: 0.4,
        }} />
      ))}
    </div>
  )
}

function VinylDisc({ playing }) {
  const discRef = useRef(null)

  useEffect(() => {
    if (playing) {
      gsap.to(discRef.current, { rotation: 360, duration: 3, repeat: -1, ease: 'none' })
    } else {
      gsap.killTweensOf(discRef.current)
    }
  }, [playing])

  return (
    <div ref={discRef} style={{
      width: 28, height: 28, borderRadius: '50%',
      border: '2px solid var(--textMuted)',
      position: 'relative', opacity: 0.3,
    }}>
      <div style={{
        width: 8, height: 8, borderRadius: '50%',
        background: 'var(--textMuted)', position: 'absolute',
        top: '50%', left: '50%', transform: 'translate(-50%, -50%)',
      }} />
    </div>
  )
}

export default function MusicVisuals() {
  const { playing } = usePlayer()

  return (
    <div style={{
      position: 'fixed', top: 0, right: 0, pointerEvents: 'none',
      display: 'flex', flexDirection: 'column', alignItems: 'flex-end',
      gap: '1rem', padding: '5rem 1.5rem 0 0', zIndex: 1, opacity: 0.5,
    }}>
      <VinylDisc playing={playing} />
      <WaveBars playing={playing} />
    </div>
  )
}

export function FloatingNotes() {
  const notes = [
    { delay: 0, x: 10, size: '1rem' },
    { delay: 1.5, x: 30, size: '0.8rem' },
    { delay: 3, x: 50, size: '1.2rem' },
    { delay: 0.8, x: 70, size: '0.7rem' },
    { delay: 2.2, x: 85, size: '0.9rem' },
  ]

  return (
    <div style={{ position: 'fixed', bottom: 0, left: 0, right: 0, height: 100, pointerEvents: 'none', zIndex: 0, overflow: 'hidden' }}>
      {notes.map((n, i) => <Note key={i} {...n} />)}
      <style>{`
        @keyframes floatNote {
          0% { transform: translateY(0) rotate(0deg); opacity: 0; }
          10% { opacity: 0.3; }
          90% { opacity: 0.3; }
          100% { transform: translateY(-80vh) rotate(20deg); opacity: 0; }
        }
      `}</style>
    </div>
  )
}
