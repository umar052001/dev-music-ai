import { useState, useEffect, useRef } from 'react'
import { useTheme } from '../context/ThemeContext'

// DownloadStatus polls the persisted download status endpoint and renders a
// progress panel. It survives page refreshes and tab switches because the
// backend keeps history in SQLite. Pass batchId to follow one batch, or leave
// it empty to show recent downloads (used as a global indicator).
export default function DownloadStatus({ batchId, compact = false }) {
  const { theme } = useTheme()
  const [data, setData] = useState(null)
  const [paused, setPaused] = useState(false)
  const timer = useRef(null)

  useEffect(() => {
    if (paused) return undefined
    fetchStatus()

    timer.current = setInterval(fetchStatus, 2000)
    return () => {
      if (timer.current) clearInterval(timer.current)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [batchId, paused])

  async function fetchStatus() {
    const q = batchId ? `?batch_id=${encodeURIComponent(batchId)}` : '?limit=20'
    try {
      const r = await fetch(`/api/downloads/status${q}`)
      const d = await r.json()
      setData(d)
      if (d?.complete) setPaused(true)
    } catch {
      // keep last data on transient errors
    }
  }

  const total = data?.total ?? 0
  const done = data?.done ?? 0
  const failed = data?.failed ?? 0
  const running = data?.running ?? 0
  const active = data?.active ?? 0
  const items = data?.items ?? []

  if (!data || total === 0) {
    return (
      <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', color: theme.textMuted, fontSize: '0.85rem' }}>
        <span style={{
          width: 7, height: 7, borderRadius: '50%', background: theme.textMuted, flexShrink: 0,
        }} />
        No downloads yet
      </div>
    )
  }

  const pct = total > 0 ? Math.round(((done + failed) / total) * 100) : 0

  return (
    <div style={{
      background: theme.cardBg, border: `1px solid ${theme.border}`, borderRadius: 12,
      padding: compact ? '0.7rem 0.9rem' : '1rem 1.1rem',
    }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '0.5rem' }}>
        <span style={{ fontSize: compact ? '0.85rem' : '1rem', fontWeight: 600 }}>Downloads</span>
        {active > 0 && (
          <span style={{ fontSize: '0.78rem', color: theme.textSecondary }}>
            {running} running · {data.queued ?? 0} queued
          </span>
        )}
      </div>

      {total > 0 && (
        <>
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.7rem' }}>
            <div style={{ flex: 1, height: 8, borderRadius: 4, background: theme.border, overflow: 'hidden' }}>
              <div style={{
                height: '100%', width: `${pct}%`, borderRadius: 4,
                background: failed > 0 && pct === 100 ? theme.accentHover : theme.text,
                transition: 'width 0.3s',
              }} />
            </div>
            <span style={{ fontSize: '0.8rem', color: theme.textMuted, flexShrink: 0, minWidth: 90, textAlign: 'right' }}>
              {done} / {total}
              {failed > 0 ? ` · ${failed} failed` : ''}
            </span>
          </div>

          {!compact && active > 0 && (
            <button onClick={() => setPaused(p => !p)} style={{
              marginTop: '0.5rem', background: 'none', border: `1px solid ${theme.border}`,
              color: theme.textSecondary, fontSize: '0.75rem', padding: '0.2rem 0.5rem', cursor: 'pointer', borderRadius: 6,
            }}>
              {paused ? 'Resume updates' : 'Pause'}
            </button>
          )}
        </>
      )}

      {!compact && items.length > 0 && (
        <div style={{ marginTop: '0.6rem', maxHeight: 220, overflowY: 'auto' }}>
          {items.map(it => (
            <div key={it.id} style={{
              display: 'flex', alignItems: 'center', gap: '0.5rem',
              padding: '0.3rem 0', fontSize: '0.82rem',
              borderBottom: '1px solid ' + theme.border,
            }}>
              <span style={{ flexShrink: 0, fontSize: '0.7rem', color: theme.textMuted, width: 60 }}>
                {statusLabel(it.status)}
              </span>
              <span style={{ flex: 1, minWidth: 0, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', color: theme.text }}>
                {it.title || it.url}
              </span>
              {it.status === 'error' && it.error && (
                  <span style={{ color: theme.textMuted, fontSize: '0.7rem', flexShrink: 0, maxWidth: 200, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                  {it.error}
                </span>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

function statusLabel(s) {
  switch (s) {
    case 'queued': return 'Queued'
    case 'running': return 'Running'
    case 'done': return '✓ Done'
    case 'error': return '✗ Failed'
    case 'cancelled': return '—'
    default: return s
  }
}
