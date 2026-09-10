import { useEffect, useState, type ChangeEvent, type CSSProperties, type FormEvent } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useApiFetch } from '../auth/useApiFetch';
import ThemeTokenFields from '../components/ThemeTokenFields';
import { backgroundCSS, defaultTheme, type ThemeTokens } from '../../shared/themes/tokens';
import { themePresets } from '../../shared/themes/presets';
import { validateThemeTokens } from '../../shared/themes/validateTokens';
import { downloadJSON, slugify } from '../../shared/downloadJSON';
import './ThemeEditorPage.css';

async function readErrorMessage(res: Response, fallback: string): Promise<string> {
  const text = await res.text();
  return text || fallback;
}

function previewStyles(tokens: ThemeTokens) {
  const container: CSSProperties = {
    background: backgroundCSS(tokens.background),
    borderRadius: '16px',
    padding: '24px',
    display: 'flex',
    flexWrap: 'wrap',
    gap: '16px',
  };
  const card: CSSProperties = {
    background: tokens.cardBackground,
    border: `1px solid ${tokens.cardBorder}`,
    borderRadius: tokens.borderRadius,
    backdropFilter: `blur(${tokens.blur})`,
    opacity: tokens.opacity,
    color: tokens.textColor,
    fontFamily: tokens.fontFamily,
    fontSize: tokens.fontSize,
    padding: '16px',
    width: '150px',
    height: '96px',
    display: 'flex',
    flexDirection: 'column',
    justifyContent: 'space-between',
  };
  const accent: CSSProperties = { color: tokens.accentColor, fontWeight: 700 };

  return { container, card, accent };
}

export default function ThemeEditorPage() {
  const { themeId } = useParams<{ themeId: string }>();
  const navigate = useNavigate();
  const apiFetch = useApiFetch();
  const isNew = themeId === 'new';

  const [name, setName] = useState('');
  const [isDefault, setIsDefault] = useState(false);
  const [tokens, setTokens] = useState<ThemeTokens>(defaultTheme);
  const [loading, setLoading] = useState(!isNew);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [importOpen, setImportOpen] = useState(false);

  async function uploadImage(file: File): Promise<string> {
    const formData = new FormData();
    formData.append('file', file);
    // No Content-Type header here -- the browser sets the multipart
    // boundary itself from the FormData body; setting one manually
    // would omit the boundary parameter and break parsing server-side.
    const res = await apiFetch('/api/admin/uploads', { method: 'POST', body: formData });
    if (!res.ok) {
      throw new Error(await readErrorMessage(res, 'Upload failed'));
    }
    const body: { url: string } = await res.json();
    return body.url;
  }

  useEffect(() => {
    if (isNew) return;
    apiFetch(`/api/admin/themes/${themeId}`)
      .then((r) => r.json())
      .then((t: { name: string; tokens: Partial<ThemeTokens>; is_default: boolean }) => {
        setName(t.name);
        setIsDefault(t.is_default);
        setTokens({ ...defaultTheme, ...t.tokens });
        setLoading(false);
      });
  }, [themeId, isNew, apiFetch]);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    if (!name.trim()) {
      setError('Name is required');
      return;
    }
    setSubmitting(true);
    try {
      const res = await apiFetch(isNew ? '/api/admin/themes' : `/api/admin/themes/${themeId}`, {
        method: isNew ? 'POST' : 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name, tokens, is_default: isDefault }),
      });
      if (!res.ok) {
        throw new Error(await readErrorMessage(res, 'Save failed'));
      }
      navigate('/admin/themes');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed');
    } finally {
      setSubmitting(false);
    }
  }

  if (loading) {
    return <p>Loading…</p>;
  }

  const preview = previewStyles(tokens);

  return (
    <div className="theme-editor-page">
      <div className="theme-editor-header">
        <button className="btn-secondary" onClick={() => navigate('/admin/themes')}>
          ← Back to Themes
        </button>
        <h1>{isNew ? 'New Theme' : `Edit ${name || 'Theme'}`}</h1>
        <div className="theme-editor-header-actions">
          <button type="button" className="btn-secondary" onClick={() => setImportOpen(true)}>
            Import
          </button>
          <button type="button" className="btn-secondary" onClick={() => downloadJSON(`${slugify(name)}.json`, tokens)}>
            Export
          </button>
        </div>
      </div>

      {isNew && (
        <div className="theme-gallery">
          <span className="kicker">Start from a preset</span>
          <div className="theme-gallery-swatches">
            {themePresets.map((preset) => (
              <button
                type="button"
                key={preset.name}
                className="theme-gallery-swatch"
                style={{ background: backgroundCSS(preset.tokens.background) }}
                onClick={() => {
                  setTokens(preset.tokens);
                  setName(preset.name);
                }}
                title={preset.name}
              >
                <span
                  className="theme-gallery-swatch-card"
                  style={{
                    background: preset.tokens.cardBackground,
                    border: `1px solid ${preset.tokens.cardBorder}`,
                    color: preset.tokens.accentColor,
                  }}
                >
                  Aa
                </span>
                <span className="theme-gallery-swatch-label" style={{ color: preset.tokens.textColor }}>
                  {preset.name}
                </span>
              </button>
            ))}
          </div>
        </div>
      )}

      <form className="theme-editor-layout" onSubmit={handleSubmit}>
        <div className="theme-editor-fields">
          {error && (
            <p className="form-error" role="alert">
              {error}
            </p>
          )}

          <label className="field">
            <span className="kicker">Name</span>
            <input value={name} onChange={(e) => setName(e.target.value)} required autoFocus />
          </label>

          <label className="toggle-row">
            <span>Default theme</span>
            <input type="checkbox" checked={isDefault} onChange={(e) => setIsDefault(e.target.checked)} />
          </label>

          <ThemeTokenFields values={tokens} onChange={(v) => setTokens((prev) => ({ ...prev, ...v }))} onUploadImage={uploadImage} />

          <div className="manifest-form-actions">
            <button type="button" className="btn-secondary" onClick={() => navigate('/admin/themes')}>
              Cancel
            </button>
            <button type="submit" className="btn-primary" disabled={submitting}>
              {submitting ? 'Saving…' : 'Save theme'}
            </button>
          </div>
        </div>

        <div className="theme-editor-preview">
          <h2>Live Preview</h2>
          <div className="preview-screen" style={preview.container}>
            <div style={preview.card}>
              <div>12:45 PM</div>
              <div style={preview.accent}>Clock</div>
            </div>
            <div style={preview.card}>
              <div>72°F, Sunny</div>
              <div style={preview.accent}>Weather</div>
            </div>
            <div style={preview.card}>
              <div>3 events today</div>
              <div style={preview.accent}>Calendar</div>
            </div>
          </div>
        </div>
      </form>

      {importOpen && (
        <div className="modal-scrim">
          <div className="modal-panel">
            <ImportTokensModal
              onApply={(imported) => {
                setTokens(imported);
                setImportOpen(false);
              }}
              onCancel={() => setImportOpen(false)}
            />
          </div>
        </div>
      )}
    </div>
  );
}

interface ImportTokensModalProps {
  onApply: (tokens: ThemeTokens) => void;
  onCancel: () => void;
}

// Import doesn't touch the saved theme at all -- it only populates the
// editor's own in-memory fields (and, through those, the live preview),
// same as typing values in by hand. The admin still has to review and
// hit "Save theme" for anything to actually persist.
function ImportTokensModal({ onApply, onCancel }: ImportTokensModalProps) {
  const [text, setText] = useState('');
  const [error, setError] = useState<string | null>(null);

  function applyText(raw: string) {
    setError(null);
    let parsed: unknown;
    try {
      parsed = JSON.parse(raw);
    } catch {
      setError('Not valid JSON.');
      return;
    }
    const result = validateThemeTokens(parsed);
    if (!result.ok) {
      setError(result.error);
      return;
    }
    onApply(result.tokens);
  }

  function handleFileChange(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    file.text().then((content) => {
      setText(content);
      applyText(content);
    });
    e.target.value = ''; // allow re-selecting the same file later
  }

  return (
    <div className="manifest-form">
      <h2>Import Theme</h2>
      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}

      <label className="field">
        <span className="kicker">Upload a .json file</span>
        <input type="file" accept="application/json,.json" onChange={handleFileChange} />
      </label>

      <label className="field">
        <span className="kicker">Or paste JSON</span>
        <textarea rows={8} value={text} onChange={(e) => setText(e.target.value)} spellCheck={false} placeholder="{ ... }" />
      </label>

      <div className="manifest-form-actions">
        <button type="button" className="btn-secondary" onClick={onCancel}>
          Cancel
        </button>
        <button type="button" className="btn-primary" onClick={() => applyText(text)} disabled={text.trim() === ''}>
          Validate &amp; Apply
        </button>
      </div>
    </div>
  );
}
