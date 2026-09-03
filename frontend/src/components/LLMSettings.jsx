import { useState, useEffect } from 'react'
import { useTheme } from '../context/ThemeContext'

const PROVIDER_INFO = {
  ollama: {
    name: 'Ollama (local + cloud)',
    needsKey: false,
    basePlaceholder: 'http://localhost:11434',
    hint: 'Uses your Ollama server. Pick a :cloud model for cloud, :latest for local.',
    modelField: 'ollamaModel',
  },
  openai: {
    name: 'OpenAI',
    needsKey: true,
    basePlaceholder: 'https://api.openai.com/v1',
    hint: 'Standard OpenAI API. Key required.',
  },
  groq: {
    name: 'Groq (fast)',
    needsKey: true,
    basePlaceholder: 'https://api.groq.com/openai/v1',
    hint: 'OpenAI-compatible, very fast inference.',
  },
  anthropic: {
    name: 'Anthropic (Claude)',
    needsKey: true,
    basePlaceholder: 'https://api.anthropic.com/v1',
    hint: 'Claude models. Uses the Messages API.',
  },
  gemini: {
    name: 'Google Gemini',
    needsKey: true,
    basePlaceholder: 'https://generativelanguage.googleapis.com/v1beta',
    hint: 'Gemini models via the generateContent API.',
  },
}

export default function LLMSettings({ open, onClose, onSaved }) {
  const { theme } = useTheme()
  const [cfg, setCfg] = useState(null)
  const [health, setHealth] = useState(null)
  const [apiKey, setApiKey] = useState('')
  const [saving, setSaving] = useState(false)
  const [savedMsg, setSavedMsg] = useState('')

  const load = () => {
    fetch('/api/llm/config').then(r => r.json()).then(d => {
      setCfg(d)
    })
    fetch('/api/llm/status').then(r => r.json()).then(setHealth).catch(() => {})
  }

  useEffect(() => {
    if (open) {
      setSavedMsg('')
      load()
    }
    // eslint-disable-next-line
  }, [open])

  if (!open || !cfg) return null

  const info = PROVIDER_INFO[cfg.provider] || PROVIDER_INFO.ollama

  const set = (field, value) => setCfg(prev => ({ ...prev, [field]: value }))

  const save = async () => {
    setSaving(true)
    setSavedMsg('')
    const body = {
      provider: cfg.provider,
      model: cfg.model,
      fast_model: cfg.fast_model,
      api_base: cfg.api_base,
      timeout_sec: cfg.timeout_sec,
      ollama_local_model: cfg.ollama_local_model,
      ollama_cloud_model: cfg.ollama_cloud_model,
      openai_model: cfg.openai_model,
      groq_model: cfg.groq_model,
      anthropic_model: cfg.anthropic_model,
      gemini_model: cfg.gemini_model,
    }
    if (apiKey) body.api_key = apiKey
    try {
      const res = await fetch('/api/llm/config-set', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      })
      if (res.ok) {
        setSavedMsg('Saved!')
        setApiKey('')
        setHealth(null)
        fetch('/api/llm/status').then(r => r.json()).then(setHealth).catch(() => {})
        onSaved && onSaved()
      } else {
        setSavedMsg('Save failed')
      }
    } catch {
      setSavedMsg('Could not connect')
    } finally {
      setSaving(false)
    }
  }

  const input = {
    background: theme.inputBg, color: theme.text,
    border: `1px solid ${theme.inputBorder}`, padding: '0.45rem 0.6rem',
    borderRadius: 6, fontSize: '0.85rem', width: '100%', outline: 'none',
  }
  const label = { display: 'block', fontSize: '0.75rem', color: theme.textMuted, marginBottom: '0.25rem' }

  return (
    <div style={{
      position: 'fixed', top: 0, right: 0, bottom: 0, width: 380, zIndex: 300,
      background: theme.bg, borderLeft: `1px solid ${theme.border}`,
      boxShadow: '-4px 0 20px ' + theme.shadow,
      display: 'flex', flexDirection: 'column', overflow: 'hidden',
    }}>
      <div style={{
        padding: '1rem 1.2rem', borderBottom: `1px solid ${theme.border}`,
        display: 'flex', justifyContent: 'space-between', alignItems: 'center',
      }}>
        <span style={{ fontSize: '1.1rem', fontWeight: 700 }}>AI Settings</span>
        <button onClick={onClose} style={{
          background: 'none', border: 'none', fontSize: '1.3rem',
          cursor: 'pointer', color: theme.text,
        }}>✕</button>
      </div>

      {/* Health status */}
      <div style={{ padding: '0.8rem 1.2rem', borderBottom: `1px solid ${theme.border}` }}>
        <div style={{ fontSize: '0.8rem', color: theme.textMuted, marginBottom: '0.4rem' }}>STATUS</div>
        {health ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem', fontSize: '0.82rem' }}>
            <span style={{ color: health.available ? '#22c55e' : '#ef4444' }}>
              {health.available ? '● Available' : '● Unavailable'}
            </span>
            <span>Provider: <b>{health.provider}</b> {health.cloud ? '(cloud)' : '(local)'}</span>
            <span>Model: {health.model}</span>
            <span>Local Ollama fallback: {health.local_ollama_ok ? '● up' : '○ down'}</span>
            {health.error && <span style={{ color: '#ef4444', fontSize: '0.75rem' }}>{health.error}</span>}
          </div>
        ) : (
          <span style={{ fontSize: '0.82rem', color: theme.textMuted }}>Checking…</span>
        )}
      </div>

      <div style={{ flex: 1, overflowY: 'auto', padding: '1rem 1.2rem' }}>
        {/* Provider */}
        <div style={{ marginBottom: '0.8rem' }}>
          <label style={label}>PROVIDER</label>
          <select value={cfg.provider} onChange={e => set('provider', e.target.value)} style={{ ...input, cursor: 'pointer' }}>
            {Object.keys(PROVIDER_INFO).map(k => (
              <option key={k} value={k}>{PROVIDER_INFO[k].name}</option>
            ))}
          </select>
          <div style={{ fontSize: '0.72rem', color: theme.textMuted, marginTop: '0.3rem' }}>{info.hint}</div>
        </div>

        {info.needsKey && (
          <div style={{ marginBottom: '0.8rem' }}>
            <label style={label}>API KEY {cfg.api_key_set ? '(already set — leave blank to keep, or enter to replace)' : ''}</label>
            <input type="password" placeholder={cfg.api_key_set ? '••••••••' : 'Enter API key'} value={apiKey} onChange={e => setApiKey(e.target.value)} style={input} />
          </div>
        )}

        <div style={{ marginBottom: '0.8rem' }}>
          <label style={label}>PRIMARY / SMART MODEL</label>
          <input value={cfg.model} onChange={e => set('model', e.target.value)} style={input} />
          <div style={{ fontSize: '0.72rem', color: theme.textMuted, marginTop: '0.25rem' }}>
            Used for suggestions & playlist generation.
          </div>
        </div>

        <div style={{ marginBottom: '0.8rem' }}>
          <label style={label}>FAST MODEL (simple tasks)</label>
          <input value={cfg.fast_model} onChange={e => set('fast_model', e.target.value)} style={input} />
          <div style={{ fontSize: '0.72rem', color: theme.textMuted, marginTop: '0.25rem' }}>
            Used for title cleanup & batch parsing.
          </div>
        </div>

        <div style={{ marginBottom: '0.8rem' }}>
          <label style={label}>API BASE URL</label>
          <input value={cfg.api_base} onChange={e => set('api_base', e.target.value)} placeholder={info.basePlaceholder} style={input} />
        </div>

        {/* Per-provider model presets */}
        <div style={{ fontSize: '0.8rem', color: theme.textMuted, marginTop: '1rem', marginBottom: '0.5rem' }}>
          PER-PROVIDER MODEL DEFAULTS
        </div>
        {[
          ['ollama_local_model', 'Ollama local model'],
          ['ollama_cloud_model', 'Ollama cloud model'],
          ['openai_model', 'OpenAI model'],
          ['groq_model', 'Groq model'],
          ['anthropic_model', 'Anthropic model'],
          ['gemini_model', 'Gemini model'],
        ].map(([key, lbl]) => (
          <div key={key} style={{ marginBottom: '0.5rem' }}>
            <label style={{ ...label, fontSize: '0.7rem' }}>{lbl.toUpperCase()}</label>
            <input value={cfg[key]} onChange={e => set(key, e.target.value)} style={input} />
          </div>
        ))}

        <div style={{ marginBottom: '0.5rem' }}>
          <label style={label}>TIMEOUT (seconds)</label>
          <input type="number" value={cfg.timeout_sec} onChange={e => set('timeout_sec', Number(e.target.value))} style={input} />
        </div>
      </div>

      <div style={{ padding: '0.8rem 1.2rem', borderTop: `1px solid ${theme.border}` }}>
        <button onClick={save} disabled={saving} style={{
          width: '100%', background: theme.text, color: theme.bg, border: 'none',
          padding: '0.7rem', borderRadius: 8, fontSize: '0.95rem', fontWeight: 700,
          cursor: saving ? 'wait' : 'pointer', opacity: saving ? 0.6 : 1,
        }}>
          {saving ? 'Saving…' : 'Save & Apply'}
        </button>
        {savedMsg && <div style={{ textAlign: 'center', fontSize: '0.8rem', color: savedMsg === 'Saved!' ? '#22c55e' : '#ef4444', marginTop: '0.4rem' }}>{savedMsg}</div>}
        <div style={{ fontSize: '0.7rem', color: theme.textMuted, textAlign: 'center', marginTop: '0.4rem' }}>
          Saved to: {cfg.config_path}
        </div>
      </div>
    </div>
  )
}
