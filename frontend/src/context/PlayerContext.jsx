import { createContext, useContext, useState, useRef, useCallback, useEffect } from 'react'

const PlayerContext = createContext()

export function PlayerProvider({ children }) {
  const audioRef = useRef(null)
  const [current, setCurrent] = useState(null)
  const [playing, setPlaying] = useState(false)
  const [progress, setProgress] = useState(0)
  const [duration, setDuration] = useState(0)
  const [queue, setQueue] = useState([])
  const [queueIdx, setQueueIdx] = useState(-1)
  const [loop, setLoop] = useState(() => localStorage.getItem('devmusic-loop') === 'true')
  const [shuffle, setShuffle] = useState(() => localStorage.getItem('devmusic-shuffle') === 'true')
  const [volume, setVolume] = useState(() => {
    const v = localStorage.getItem('devmusic-volume')
    return v ? Number(v) : 80
  })

  // Activity logging
  const logAct = useCallback((action, track, artist, url) => {
    fetch('/api/activity', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ action, track: track?.name || track?.title || '', artist: artist || track?.artist || track?.uploader || '', url: url || track?.url || '' }),
    }).catch(() => {})
  }, [])

  const play = useCallback((track, list) => {
    if (audioRef.current) {
      audioRef.current.pause()
    }

    let src
    if (track.path) {
      src = `/api/file/${track.path}`
    } else if (track.url) {
      src = `/api/stream?url=${encodeURIComponent(track.url)}`
    } else {
      return
    }

    const audio = new Audio(src)
    audio.volume = volume / 100
    audioRef.current = audio

    setCurrent(track)
    setPlaying(true)
    setProgress(0)
    setDuration(0)

    if (list && list.length) {
      let idx = list.findIndex(t => (t.id && t.id === track.id) || (t.path && t.path === track.path))
      // If shuffle, randomize the queue order
      let orderedList = [...list]
      if (shuffle && list.length > 1) {
        orderedList = [...list]
        const currentTrack = orderedList.splice(Math.max(0, idx), 1)[0]
        for (let i = orderedList.length - 1; i > 0; i--) {
          const j = Math.floor(Math.random() * (i + 1));
          [orderedList[i], orderedList[j]] = [orderedList[j], orderedList[i]]
        }
        orderedList.unshift(currentTrack)
        idx = 0
      }
      setQueue(orderedList)
      setQueueIdx(idx >= 0 ? idx : 0)
    }

    audio.addEventListener('loadedmetadata', () => setDuration(audio.duration))
    audio.addEventListener('timeupdate', () => {
      if (audio.duration) setProgress((audio.currentTime / audio.duration) * 100)
    })
    audio.addEventListener('ended', () => {
      logAct('play', track, track?.artist || track?.uploader, track?.url)
      if (loop) {
        audio.currentTime = 0
        audio.play()
      } else {
        setQueueIdx(prev => {
          const next = prev + 1
          if (next < queue.length) {
            setTimeout(() => play(queue[next]), 0)
            return next
          }
          setPlaying(false)
          return prev
        })
      }
    })
    audio.addEventListener('play', () => logAct('play', track, track?.artist || track?.uploader, track?.url))
    audio.addEventListener('error', () => setPlaying(false))
    audio.play().catch(() => {})
  }, [volume, loop, shuffle, queue, logAct])

  const toggle = useCallback(() => {
    if (!audioRef.current) return
    if (audioRef.current.paused) {
      audioRef.current.play()
      setPlaying(true)
    } else {
      audioRef.current.pause()
      setPlaying(false)
    }
  }, [])

  const seek = useCallback((pct) => {
    if (audioRef.current && audioRef.current.duration) {
      audioRef.current.currentTime = (pct / 100) * audioRef.current.duration
    }
  }, [])

  const changeVolume = useCallback((v) => {
    setVolume(v)
    localStorage.setItem('devmusic-volume', v)
    if (audioRef.current) audioRef.current.volume = v / 100
  }, [])

  const prev = useCallback(() => {
    if (queueIdx > 0) play(queue[queueIdx - 1])
  }, [queueIdx, queue, play])

  const next = useCallback(() => {
    if (queueIdx < queue.length - 1) {
      play(queue[queueIdx + 1])
    } else if (loop && queue.length > 0) {
      play(queue[0])
    }
  }, [queueIdx, queue, play, loop])

  const toggleLoop = useCallback(() => {
    setLoop(p => { localStorage.setItem('devmusic-loop', !p); return !p })
  }, [])

  const toggleShuffle = useCallback(() => {
    setShuffle(p => { localStorage.setItem('devmusic-shuffle', !p); return !p })
  }, [])

  // Keyboard shortcuts
  useEffect(() => {
    const handler = (e) => {
      // Don't trigger if user is typing in an input
      if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA' || e.target.tagName === 'SELECT') return

      if (e.code === 'Space') {
        e.preventDefault()
        toggle()
      } else if (e.code === 'ArrowRight' && e.shiftKey) {
        next()
      } else if (e.code === 'ArrowLeft' && e.shiftKey) {
        prev()
      } else if (e.code === 'ArrowUp' && e.shiftKey) {
        e.preventDefault()
        changeVolume(Math.min(100, volume + 5))
      } else if (e.code === 'ArrowDown' && e.shiftKey) {
        e.preventDefault()
        changeVolume(Math.max(0, volume - 5))
      } else if (e.key === 'l' || e.key === 'L') {
        toggleLoop()
      } else if (e.key === 's' || e.key === 'S') {
        toggleShuffle()
      }
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [toggle, next, prev, volume, changeVolume, toggleLoop, toggleShuffle])

  return (
    <PlayerContext.Provider value={{
      current, playing, progress, duration,
      queue, queueIdx, volume, loop, shuffle,
      play, toggle, seek, changeVolume, prev, next,
      toggleLoop, toggleShuffle, logAct,
      hasPrev: queueIdx > 0 || loop,
      hasNext: queueIdx < queue.length - 1 || loop,
    }}>
      {children}
    </PlayerContext.Provider>
  )
}

export const usePlayer = () => useContext(PlayerContext)
