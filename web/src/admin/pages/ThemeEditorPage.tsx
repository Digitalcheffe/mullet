import { useEffect, useState, type CSSProperties, type FormEvent } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useApiFetch } from '../auth/useApiFetch';
import ThemeTokenFields from '../components/ThemeTokenFields';
import { backgroundCSS, defaultTheme, type ThemeTokens } from '../../shared/themes/tokens';
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
      </div>

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

          <ThemeTokenFields values={tokens} onChange={(v) => setTokens((prev) => ({ ...prev, ...v }))} />

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
    </div>
  );
}
