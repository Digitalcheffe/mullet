import { useCallback, useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useApiFetch } from '../auth/useApiFetch';
import type { ThemeTokens } from '../../shared/themes/tokens';
import { downloadJSON, slugify } from '../../shared/downloadJSON';
import './ThemesPage.css';

interface Theme {
  id: number;
  name: string;
  tokens: ThemeTokens;
  is_default: boolean;
}

async function readErrorMessage(res: Response, fallback: string): Promise<string> {
  const text = await res.text();
  return text || fallback;
}

export default function ThemesPage() {
  const apiFetch = useApiFetch();
  const navigate = useNavigate();
  const [themes, setThemes] = useState<Theme[]>([]);
  const [loading, setLoading] = useState(true);

  const load = useCallback(() => {
    return apiFetch('/api/admin/themes')
      .then((r) => r.json())
      .then((t: Theme[]) => {
        setThemes(t);
        setLoading(false);
      });
  }, [apiFetch]);

  useEffect(() => {
    load();
  }, [load]);

  function handleExport(theme: Theme) {
    downloadJSON(`${slugify(theme.name)}.json`, theme.tokens);
  }

  async function handleDelete(theme: Theme) {
    if (!confirm(`Delete theme "${theme.name}"?`)) return;
    const res = await apiFetch(`/api/admin/themes/${theme.id}`, { method: 'DELETE' });
    if (!res.ok) {
      alert(await readErrorMessage(res, 'Delete failed'));
      return;
    }
    load();
  }

  if (loading) {
    return <p>Loading…</p>;
  }

  return (
    <div className="themes-page">
      <div className="section-header">
        <h1>Themes</h1>
        <button className="btn-primary" onClick={() => navigate('/admin/themes/new')}>
          + Add Theme
        </button>
      </div>

      {themes.length === 0 ? (
        <div className="empty-state">No themes yet.</div>
      ) : (
        <div className="theme-list">
          {themes.map((t) => (
            <div className="theme-row" key={t.id}>
              <div className="theme-swatch" style={{ background: t.tokens.background?.value }}>
                <span
                  className="theme-swatch-card"
                  style={{ background: t.tokens.cardBackground, border: `1px solid ${t.tokens.cardBorder}`, color: t.tokens.accentColor }}
                >
                  Aa
                </span>
              </div>
              <div className="theme-info">
                <div className="theme-name">
                  {t.name}
                  {t.is_default && <span className="default-badge">Default</span>}
                </div>
              </div>
              <button className="btn-secondary" onClick={() => navigate(`/admin/themes/${t.id}`)}>
                Edit
              </button>
              <button className="btn-secondary" onClick={() => handleExport(t)}>
                Export
              </button>
              <button className="btn-danger" onClick={() => handleDelete(t)}>
                Delete
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
