import { useCallback, useEffect, useState, type FormEvent } from 'react';
import { useApiFetch } from '../auth/useApiFetch';
import './DisplaysPage.css';

interface Theme {
  id: number;
  name: string;
  is_default: boolean;
}

interface Display {
  id: number;
  name: string;
  slug: string;
  theme_id?: number;
  rotation_seconds: number;
  show_top_bar: boolean;
  show_bottom_bar: boolean;
}

interface Screen {
  id: number;
  display_id: number;
  name: string;
  position: number;
  columns: number;
  row_height: number;
  gap: number;
}

interface DisplayFormValues {
  name: string;
  slug: string;
  theme_id: number | null;
  rotation_seconds: number;
  show_top_bar: boolean;
  show_bottom_bar: boolean;
}

interface ScreenFormValues {
  name: string;
  columns: number;
  row_height: number;
  gap: number;
}

type DisplayPanel = { mode: 'add' } | { mode: 'edit'; display: Display } | null;
type ScreenPanel = { mode: 'add' } | { mode: 'edit'; screen: Screen } | null;

async function readErrorMessage(res: Response, fallback: string): Promise<string> {
  const text = await res.text();
  return text || fallback;
}

export default function DisplaysPage() {
  const apiFetch = useApiFetch();
  const [displays, setDisplays] = useState<Display[]>([]);
  const [themes, setThemes] = useState<Theme[]>([]);
  const [loading, setLoading] = useState(true);
  const [displayPanel, setDisplayPanel] = useState<DisplayPanel>(null);

  const [selectedDisplayId, setSelectedDisplayId] = useState<number | null>(null);
  const [screens, setScreens] = useState<Screen[]>([]);
  const [screensLoading, setScreensLoading] = useState(false);
  const [screenPanel, setScreenPanel] = useState<ScreenPanel>(null);

  const loadDisplays = useCallback(() => {
    return Promise.all([
      apiFetch('/api/admin/displays').then((r) => r.json()),
      apiFetch('/api/admin/themes').then((r) => r.json()),
    ]).then(([d, t]) => {
      setDisplays(d);
      setThemes(t);
      setLoading(false);
    });
  }, [apiFetch]);

  useEffect(() => {
    loadDisplays();
  }, [loadDisplays]);

  const loadScreens = useCallback(
    (displayId: number) => {
      setScreensLoading(true);
      return apiFetch(`/api/admin/displays/${displayId}/screens`)
        .then((r) => r.json())
        .then((s: Screen[]) => {
          setScreens([...s].sort((a, b) => a.position - b.position));
          setScreensLoading(false);
        });
    },
    [apiFetch],
  );

  useEffect(() => {
    if (selectedDisplayId != null) {
      loadScreens(selectedDisplayId);
    }
  }, [selectedDisplayId, loadScreens]);

  // Deselects the current display (or selects a new one), clearing the
  // stale screen list synchronously rather than through an effect --
  // toggling selection is a discrete user action, not a value derived
  // from external state.
  function selectDisplay(id: number | null) {
    setSelectedDisplayId(id);
    if (id == null) setScreens([]);
  }

  async function submitDisplay(values: DisplayFormValues, url: string, method: 'POST' | 'PUT') {
    const res = await apiFetch(url, {
      method,
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name: values.name,
        slug: values.slug,
        theme_id: values.theme_id,
        rotation_seconds: values.rotation_seconds,
        show_top_bar: values.show_top_bar,
        show_bottom_bar: values.show_bottom_bar,
      }),
    });
    if (!res.ok) {
      throw new Error(await readErrorMessage(res, 'Save failed'));
    }
    setDisplayPanel(null);
    await loadDisplays();
  }

  async function handleDeleteDisplay(display: Display) {
    if (!confirm(`Remove "${display.name}"? This also removes its screens and cards.`)) return;
    await apiFetch(`/api/admin/displays/${display.id}`, { method: 'DELETE' });
    if (selectedDisplayId === display.id) selectDisplay(null);
    loadDisplays();
  }

  async function submitScreen(values: ScreenFormValues, url: string, method: 'POST' | 'PUT', position?: number) {
    const res = await apiFetch(url, {
      method,
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name: values.name,
        position: position ?? 0,
        columns: values.columns,
        row_height: values.row_height,
        gap: values.gap,
      }),
    });
    if (!res.ok) {
      throw new Error(await readErrorMessage(res, 'Save failed'));
    }
    setScreenPanel(null);
    if (selectedDisplayId != null) await loadScreens(selectedDisplayId);
  }

  async function handleDeleteScreen(screen: Screen) {
    if (!confirm(`Remove screen "${screen.name}"? This also removes its cards.`)) return;
    await apiFetch(`/api/admin/screens/${screen.id}`, { method: 'DELETE' });
    if (selectedDisplayId != null) loadScreens(selectedDisplayId);
  }

  async function moveScreen(index: number, direction: -1 | 1) {
    const target = index + direction;
    if (target < 0 || target >= screens.length) return;
    const a = screens[index];
    const b = screens[target];
    // Swap positions. Screens keep their own columns/row_height/gap --
    // only rotation order changes.
    await Promise.all([
      apiFetch(`/api/admin/screens/${a.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: a.name, position: b.position, columns: a.columns, row_height: a.row_height, gap: a.gap }),
      }),
      apiFetch(`/api/admin/screens/${b.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: b.name, position: a.position, columns: b.columns, row_height: b.row_height, gap: b.gap }),
      }),
    ]);
    if (selectedDisplayId != null) await loadScreens(selectedDisplayId);
  }

  function themeName(themeId?: number): string {
    if (themeId == null) return 'None';
    return themes.find((t) => t.id === themeId)?.name ?? `Theme #${themeId}`;
  }

  if (loading) {
    return <p>Loading…</p>;
  }

  const selectedDisplay = displays.find((d) => d.id === selectedDisplayId) ?? null;

  return (
    <div className="displays-page">
      <h1>Displays</h1>

      <section>
        <div className="section-header">
          <h2>Your Displays</h2>
          <button className="btn-secondary" onClick={() => setDisplayPanel({ mode: 'add' })}>
            + Add Display
          </button>
        </div>

        {displays.length === 0 ? (
          <div className="empty-state">No displays configured yet.</div>
        ) : (
          <div className="display-list">
            {displays.map((d) => (
              <div className={`display-row${selectedDisplayId === d.id ? ' selected' : ''}`} key={d.id}>
                <div className="display-info" onClick={() => selectDisplay(selectedDisplayId === d.id ? null : d.id)}>
                  <div className="display-name">{d.name}</div>
                  <div className="display-detail">
                    /display/{d.slug} &middot; every {d.rotation_seconds}s &middot; theme: {themeName(d.theme_id)}
                  </div>
                </div>
                <button className="btn-secondary" onClick={() => selectDisplay(selectedDisplayId === d.id ? null : d.id)}>
                  {selectedDisplayId === d.id ? 'Hide screens' : 'Screens'}
                </button>
                <button className="btn-secondary" onClick={() => setDisplayPanel({ mode: 'edit', display: d })}>
                  Edit
                </button>
                <button className="btn-danger" onClick={() => handleDeleteDisplay(d)}>
                  Delete
                </button>
              </div>
            ))}
          </div>
        )}
      </section>

      {selectedDisplay && (
        <section>
          <div className="section-header">
            <h2>Screens for &ldquo;{selectedDisplay.name}&rdquo;</h2>
            <button className="btn-secondary" onClick={() => setScreenPanel({ mode: 'add' })}>
              + Add Screen
            </button>
          </div>

          {screensLoading ? (
            <p>Loading…</p>
          ) : screens.length === 0 ? (
            <div className="empty-state">No screens on this display yet. A display needs at least one to show anything.</div>
          ) : (
            <div className="screen-list">
              {screens.map((s, i) => (
                <div className="screen-row" key={s.id}>
                  <div className="screen-order">
                    <button className="btn-mini" disabled={i === 0} onClick={() => moveScreen(i, -1)} title="Move earlier">
                      ▲
                    </button>
                    <button className="btn-mini" disabled={i === screens.length - 1} onClick={() => moveScreen(i, 1)} title="Move later">
                      ▼
                    </button>
                  </div>
                  <div className="screen-info">
                    <div className="screen-name">{s.name}</div>
                    <div className="screen-detail">
                      {s.columns} columns &middot; {s.row_height}px rows &middot; {s.gap}px gap
                    </div>
                  </div>
                  <button className="btn-secondary" onClick={() => setScreenPanel({ mode: 'edit', screen: s })}>
                    Edit
                  </button>
                  <button className="btn-danger" onClick={() => handleDeleteScreen(s)}>
                    Delete
                  </button>
                </div>
              ))}
            </div>
          )}
        </section>
      )}

      {displayPanel && (
        <div className="modal-scrim" onClick={() => setDisplayPanel(null)}>
          <div className="modal-panel" onClick={(e) => e.stopPropagation()}>
            <h2>{displayPanel.mode === 'add' ? 'Add Display' : `Edit ${displayPanel.display.name}`}</h2>
            <DisplayForm
              themes={themes}
              initialValues={
                displayPanel.mode === 'add'
                  ? { name: '', slug: '', theme_id: null, rotation_seconds: 30, show_top_bar: true, show_bottom_bar: true }
                  : {
                      name: displayPanel.display.name,
                      slug: displayPanel.display.slug,
                      theme_id: displayPanel.display.theme_id ?? null,
                      rotation_seconds: displayPanel.display.rotation_seconds,
                      show_top_bar: displayPanel.display.show_top_bar,
                      show_bottom_bar: displayPanel.display.show_bottom_bar,
                    }
              }
              submitLabel={displayPanel.mode === 'add' ? 'Add display' : 'Save changes'}
              onSubmit={(values) =>
                displayPanel.mode === 'add'
                  ? submitDisplay(values, '/api/admin/displays', 'POST')
                  : submitDisplay(values, `/api/admin/displays/${displayPanel.display.id}`, 'PUT')
              }
              onCancel={() => setDisplayPanel(null)}
            />
          </div>
        </div>
      )}

      {screenPanel && selectedDisplayId != null && (
        <div className="modal-scrim" onClick={() => setScreenPanel(null)}>
          <div className="modal-panel" onClick={(e) => e.stopPropagation()}>
            <h2>{screenPanel.mode === 'add' ? 'Add Screen' : `Edit ${screenPanel.screen.name}`}</h2>
            <ScreenForm
              initialValues={
                screenPanel.mode === 'add'
                  ? { name: '', columns: 16, row_height: 40, gap: 8 }
                  : {
                      name: screenPanel.screen.name,
                      columns: screenPanel.screen.columns,
                      row_height: screenPanel.screen.row_height,
                      gap: screenPanel.screen.gap,
                    }
              }
              submitLabel={screenPanel.mode === 'add' ? 'Add screen' : 'Save changes'}
              onSubmit={(values) =>
                screenPanel.mode === 'add'
                  ? submitScreen(values, `/api/admin/displays/${selectedDisplayId}/screens`, 'POST', screens.length)
                  : submitScreen(values, `/api/admin/screens/${screenPanel.screen.id}`, 'PUT', screenPanel.screen.position)
              }
              onCancel={() => setScreenPanel(null)}
            />
          </div>
        </div>
      )}
    </div>
  );
}

interface DisplayFormProps {
  themes: Theme[];
  initialValues: DisplayFormValues;
  submitLabel: string;
  onSubmit: (values: DisplayFormValues) => Promise<void>;
  onCancel: () => void;
}

function DisplayForm({ themes, initialValues, submitLabel, onSubmit, onCancel }: DisplayFormProps) {
  const [values, setValues] = useState(initialValues);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await onSubmit(values);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <form className="manifest-form" onSubmit={handleSubmit}>
      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}

      <label className="field">
        <span className="kicker">Name</span>
        <input value={values.name} onChange={(e) => setValues((v) => ({ ...v, name: e.target.value }))} required autoFocus />
      </label>

      <label className="field">
        <span className="kicker">Slug</span>
        <input
          value={values.slug}
          onChange={(e) => setValues((v) => ({ ...v, slug: e.target.value }))}
          placeholder="kitchen"
          required
        />
        <span className="field-help">Used for the URL: /display/{values.slug || '…'}</span>
      </label>

      <label className="field">
        <span className="kicker">Theme</span>
        <select
          value={values.theme_id ?? ''}
          onChange={(e) => setValues((v) => ({ ...v, theme_id: e.target.value === '' ? null : Number(e.target.value) }))}
        >
          <option value="">None</option>
          {themes.map((t) => (
            <option key={t.id} value={t.id}>
              {t.name}
            </option>
          ))}
        </select>
      </label>

      <label className="field">
        <span className="kicker">Rotation interval (seconds)</span>
        <input
          type="number"
          min={1}
          value={values.rotation_seconds}
          onChange={(e) => setValues((v) => ({ ...v, rotation_seconds: Number(e.target.value) }))}
        />
        <span className="field-help">How long the display stays on each screen before advancing.</span>
      </label>

      <label className="toggle-row">
        <span>Show top bar</span>
        <input
          type="checkbox"
          checked={values.show_top_bar}
          onChange={(e) => setValues((v) => ({ ...v, show_top_bar: e.target.checked }))}
        />
      </label>

      <label className="toggle-row">
        <span>Show bottom bar</span>
        <input
          type="checkbox"
          checked={values.show_bottom_bar}
          onChange={(e) => setValues((v) => ({ ...v, show_bottom_bar: e.target.checked }))}
        />
      </label>

      <div className="manifest-form-actions">
        <button type="button" className="btn-secondary" onClick={onCancel}>
          Cancel
        </button>
        <button type="submit" className="btn-primary" disabled={submitting}>
          {submitting ? 'Saving…' : submitLabel}
        </button>
      </div>
    </form>
  );
}

interface ScreenFormProps {
  initialValues: ScreenFormValues;
  submitLabel: string;
  onSubmit: (values: ScreenFormValues) => Promise<void>;
  onCancel: () => void;
}

function ScreenForm({ initialValues, submitLabel, onSubmit, onCancel }: ScreenFormProps) {
  const [values, setValues] = useState(initialValues);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await onSubmit(values);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <form className="manifest-form" onSubmit={handleSubmit}>
      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}

      <label className="field">
        <span className="kicker">Name</span>
        <input value={values.name} onChange={(e) => setValues((v) => ({ ...v, name: e.target.value }))} required autoFocus />
      </label>

      <label className="field">
        <span className="kicker">Columns</span>
        <input
          type="number"
          min={1}
          value={values.columns}
          onChange={(e) => setValues((v) => ({ ...v, columns: Number(e.target.value) }))}
        />
      </label>

      <label className="field">
        <span className="kicker">Row height (pixels)</span>
        <input
          type="number"
          min={1}
          value={values.row_height}
          onChange={(e) => setValues((v) => ({ ...v, row_height: Number(e.target.value) }))}
        />
      </label>

      <label className="field">
        <span className="kicker">Gap (pixels)</span>
        <input
          type="number"
          min={0}
          value={values.gap}
          onChange={(e) => setValues((v) => ({ ...v, gap: Number(e.target.value) }))}
        />
      </label>

      <div className="manifest-form-actions">
        <button type="button" className="btn-secondary" onClick={onCancel}>
          Cancel
        </button>
        <button type="submit" className="btn-primary" disabled={submitting}>
          {submitting ? 'Saving…' : submitLabel}
        </button>
      </div>
    </form>
  );
}
