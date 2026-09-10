import { useCallback, useEffect, useMemo, useRef, useState, type FormEvent } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import ReactGridLayout, { useContainerWidth, type Layout, type LayoutItem } from 'react-grid-layout';
import 'react-grid-layout/css/styles.css';
import 'react-resizable/css/styles.css';
import { useApiFetch } from '../auth/useApiFetch';
import ThemeTokenFields from '../components/ThemeTokenFields';
import type { ThemeTokens } from '../../shared/themes/tokens';
import type { ConfigField } from '../../shared/types/plugin';
import { getUIPlugin, uiPlugins } from '../../plugins/registry';
import './DesignerPage.css';

interface Display {
  id: number;
  name: string;
  slug: string;
}

interface Screen {
  id: number;
  display_id: number;
  name: string;
  position: number;
  columns: number;
  row_height: number;
  gap: number;
  layout_mode: 'simple' | 'freeform';
}

interface Card {
  id: number;
  screen_id: number;
  ui_plugin_id: string;
  data_plugin_instance_id?: number;
  x: number;
  y: number;
  w: number;
  h: number;
  config: Record<string, unknown>;
  theme_override?: Partial<ThemeTokens>;
}

interface PluginManifest {
  id: string;
  name: string;
  data_shapes: string[] | null;
}

interface PluginInstance {
  id: number;
  plugin_id: string;
  instance_name: string;
}

const DEFAULT_CARD_SIZE = { w: 4, h: 3 };

// Mirrors internal/api/display_handlers.go's simpleColumns/simpleRowHeight/
// simpleGap and freeformColumns/freeformRowHeight/freeformGap -- needed
// here only for the layout-mode toggle, which sends these explicitly
// rather than relying on the backend's own omitted-field defaulting
// (that defaulting treats an explicit 0 gap as intentional on update, by
// design -- see screenDefaults' doc comment -- so the toggle can't rely
// on omission to land on either mode's own gap default here).
const SIMPLE_GRID = { columns: 4, row_height: 90, gap: 12 };
const FREEFORM_GRID = { columns: 8, row_height: 60, gap: 10 };

// Simple mode (issue #73): S/M/L presets map directly onto each widget's
// own already-declared minSize/defaultSize/maxSize -- no new per-widget
// data needed. Those were authored against freeform's wider default
// (16-column) grid, though, so a widget's width is clamped to whatever
// the screen's own column count actually is (4, for simple mode's fixed
// grid) -- height is left alone, since there's no fixed row limit to
// clamp against. This is why several widgets' Medium/Large end up the
// same width: once a widget's declared width already meets or exceeds
// the grid's column count, there's nowhere further for width to grow,
// and only height still differs between presets.
type SizePreset = 'S' | 'M' | 'L';
const SIZE_PRESETS: SizePreset[] = ['S', 'M', 'L'];
const SIZE_PRESET_NAMES: Record<SizePreset, string> = { S: 'Small', M: 'Medium', L: 'Large' };

function clampToGrid(size: { w: number; h: number }, cols: number): { w: number; h: number } {
  return { w: Math.min(size.w, cols), h: size.h };
}

function presetSize(plugin: ReturnType<typeof getUIPlugin>, preset: SizePreset, cols: number): { w: number; h: number } {
  if (!plugin) return DEFAULT_CARD_SIZE;
  const raw = preset === 'S' ? plugin.minSize : preset === 'L' ? (plugin.maxSize ?? plugin.defaultSize) : plugin.defaultSize;
  return clampToGrid(raw, cols);
}

// A faint reference grid behind the cards, so the column/row structure
// is visible even in empty space rather than only implied by wherever
// cards happen to already sit -- otherwise there's no way to see the
// grid you're placing things onto at all. Row lines line up exactly
// with react-grid-layout's own math (each row is rowHeight + gap
// pixels); column lines are percentage-based and only an approximation
// of RGL's actual column edges (which subtract margin from available
// width per column) -- close enough to read as a grid, not meant to be
// a pixel-perfect placement guide.
function gridlineBackground(cols: number, rowHeight: number, gap: number): string {
  const colPct = 100 / cols;
  const rowPitch = rowHeight + gap;
  return [
    `repeating-linear-gradient(to right, transparent 0, transparent calc(${colPct}% - 1px), var(--border) calc(${colPct}% - 1px), var(--border) ${colPct}%)`,
    `repeating-linear-gradient(to bottom, transparent 0, transparent ${rowPitch - 1}px, var(--border) ${rowPitch - 1}px, var(--border) ${rowPitch}px)`,
  ].join(', ');
}

function dataSourceLabel(card: Card, instances: PluginInstance[]): string {
  if (card.data_plugin_instance_id == null) return 'No data source';
  return instances.find((i) => i.id === card.data_plugin_instance_id)?.instance_name ?? 'Unknown source';
}

async function readErrorMessage(res: Response, fallback: string): Promise<string> {
  const text = await res.text();
  return text || fallback;
}

export default function DesignerPage() {
  const { displayId, screenId } = useParams<{ displayId: string; screenId: string }>();
  const navigate = useNavigate();
  const apiFetch = useApiFetch();
  const { width, containerRef, mounted } = useContainerWidth();

  const [display, setDisplay] = useState<Display | null>(null);
  const [screens, setScreens] = useState<Screen[]>([]);
  const [screen, setScreen] = useState<Screen | null>(null);
  const [cards, setCards] = useState<Card[]>([]);
  const [manifests, setManifests] = useState<PluginManifest[]>([]);
  const [instances, setInstances] = useState<PluginInstance[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedCardId, setSelectedCardId] = useState<number | null>(null);
  const draggingPluginRef = useRef<string | null>(null);
  const [saveStatus, setSaveStatus] = useState<'idle' | 'saving' | 'saved' | 'error'>('idle');
  const savedTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  // Local draft for freeform mode's editable columns/row_height/gap
  // fields -- kept separate from `screen` so typing doesn't fire a save
  // on every keystroke; committed on blur, and re-synced whenever the
  // loaded screen's own values change (a fresh load, or switching to a
  // different screen entirely).
  const [gridDraft, setGridDraft] = useState(FREEFORM_GRID);

  // Every card mutation already persists immediately (drag, resize,
  // add, delete, settings) -- there's no separate "Save" step to gate
  // behind a button. This just surfaces that autosave visibly while
  // you're in the Designer, since nothing else in the UI indicated it
  // was happening.
  async function withSaveStatus(action: () => Promise<void>) {
    if (savedTimeoutRef.current) {
      clearTimeout(savedTimeoutRef.current);
      savedTimeoutRef.current = null;
    }
    setSaveStatus('saving');
    try {
      await action();
      setSaveStatus('saved');
      savedTimeoutRef.current = setTimeout(() => setSaveStatus('idle'), 2000);
    } catch (err) {
      setSaveStatus('error');
      throw err;
    }
  }

  useEffect(() => {
    return () => {
      if (savedTimeoutRef.current) clearTimeout(savedTimeoutRef.current);
    };
  }, []);

  useEffect(() => {
    if (screen) setGridDraft({ columns: screen.columns, row_height: screen.row_height, gap: screen.gap });
    // Deliberately narrow deps (not the whole `screen` object): `load()`
    // returns a fresh object every call, including from unrelated
    // mutations (adding a card, say) -- keying on it directly would
    // reset this draft, and any edit still in progress in the inputs
    // below, every time something else on the page saves.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [screen?.id, screen?.columns, screen?.row_height, screen?.gap]);

  const load = useCallback(() => {
    return Promise.all([
      apiFetch(`/api/admin/displays/${displayId}`).then((r) => r.json()),
      apiFetch(`/api/admin/displays/${displayId}/screens`).then((r) => r.json()),
      apiFetch(`/api/admin/screens/${screenId}`).then((r) => r.json()),
      apiFetch(`/api/admin/screens/${screenId}/cards`).then((r) => r.json()),
      apiFetch('/api/admin/plugins').then((r) => r.json()),
      apiFetch('/api/admin/plugins/instances').then((r) => r.json()),
    ]).then(([d, scr, s, c, m, i]) => {
      setDisplay(d);
      setScreens([...scr].sort((a: Screen, b: Screen) => a.position - b.position));
      setScreen(s);
      setCards(c);
      setManifests(m);
      setInstances(i);
      setLoading(false);
    });
  }, [apiFetch, displayId, screenId]);

  useEffect(() => {
    load();
  }, [load]);

  const layout: Layout = useMemo(() => cards.map((c) => ({ i: String(c.id), x: c.x, y: c.y, w: c.w, h: c.h })), [cards]);

  // Persists every card whose position/size changed after a drag or
  // resize. react-grid-layout's compactor can shift more than the one
  // item you touched (to close gaps, avoid overlap), so this diffs the
  // whole returned layout against known card state rather than trusting
  // a single "moved" item.
  async function persistPositions(finalLayout: Layout) {
    const changed = finalLayout.filter((item) => {
      const card = cards.find((c) => String(c.id) === item.i);
      return card && (card.x !== item.x || card.y !== item.y || card.w !== item.w || card.h !== item.h);
    });
    if (changed.length === 0) return;

    // Optimistic local update so the grid doesn't snap back while the
    // requests are in flight.
    setCards((prev) =>
      prev.map((c) => {
        const item = finalLayout.find((l) => l.i === String(c.id));
        return item ? { ...c, x: item.x, y: item.y, w: item.w, h: item.h } : c;
      }),
    );

    await withSaveStatus(async () => {
      await Promise.all(
        changed.map((item) => {
          const card = cards.find((c) => String(c.id) === item.i)!;
          return apiFetch(`/api/admin/cards/${card.id}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
              ui_plugin_id: card.ui_plugin_id,
              data_plugin_instance_id: card.data_plugin_instance_id ?? null,
              x: item.x,
              y: item.y,
              w: item.w,
              h: item.h,
              config: card.config,
              theme_override: card.theme_override ?? null,
            }),
          });
        }),
      );
      await load();
    });
  }

  async function createCard(uiPluginId: string, x: number, y: number, w: number, h: number) {
    await withSaveStatus(async () => {
      await apiFetch(`/api/admin/screens/${screenId}/cards`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          ui_plugin_id: uiPluginId,
          data_plugin_instance_id: null,
          x,
          y,
          w,
          h,
          config: {},
          theme_override: null,
        }),
      });
      await load();
    });
  }

  async function handleDrop(_finalLayout: Layout, item: LayoutItem | undefined) {
    const uiPluginId = draggingPluginRef.current;
    draggingPluginRef.current = null;
    if (!uiPluginId || !item) return;
    await createCard(uiPluginId, item.x, item.y, item.w, item.h);
  }

  // Escape hatch for native HTML5 drag-and-drop, which depends on the
  // browser/OS actually recognizing the gesture as a drag (inconsistent
  // across browsers, trackpads, and touch devices -- see issue #79).
  // Double-clicking a palette item places it directly, stacked below
  // whatever's already on the grid, no drag required. Simple mode places
  // it at the widget's own Medium preset size; freeform keeps the
  // original fixed placeholder size, unchanged.
  async function handlePaletteDoubleClick(uiPluginId: string) {
    if (!screen) return;
    const bottom = cards.reduce((max, c) => Math.max(max, c.y + c.h), 0);
    const size =
      screen.layout_mode === 'simple' ? presetSize(getUIPlugin(uiPluginId), 'M', screen.columns) : DEFAULT_CARD_SIZE;
    await createCard(uiPluginId, 0, bottom, size.w, size.h);
  }

  // The S/M/L picker on a placed card (simple mode only) -- dragging
  // still just repositions a card (see dragConfig below), this is the
  // only way to resize one when resize handles are turned off.
  async function handleResizeCardPreset(card: Card, preset: SizePreset) {
    if (!screen) return;
    const { w, h } = presetSize(getUIPlugin(card.ui_plugin_id), preset, screen.columns);
    await withSaveStatus(async () => {
      await apiFetch(`/api/admin/cards/${card.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          ui_plugin_id: card.ui_plugin_id,
          data_plugin_instance_id: card.data_plugin_instance_id ?? null,
          x: card.x,
          y: card.y,
          w,
          h,
          config: card.config,
          theme_override: card.theme_override ?? null,
        }),
      });
      await load();
    });
  }

  // "Switch to advanced layout" / "Switch to simple layout" -- the
  // opt-in escape hatch from issue #73. Switching to freeform seeds the
  // original full-control grid (16 columns/40px rows/8px gap) rather
  // than leaving it on simple mode's narrow 4-column grid labeled
  // "advanced"; switching back to simple resets to simple's own fixed
  // grid, since not exposing these numbers at all is simple mode's whole
  // point. Either direction leaves every card's x/y/w/h exactly as it
  // was -- cards aren't moved or resized by the toggle itself.
  async function handleToggleLayoutMode() {
    if (!screen) return;
    const nextMode = screen.layout_mode === 'simple' ? 'freeform' : 'simple';
    const grid = nextMode === 'freeform' ? FREEFORM_GRID : SIMPLE_GRID;
    const body = { name: screen.name, position: screen.position, layout_mode: nextMode, ...grid };
    await withSaveStatus(async () => {
      await apiFetch(`/api/admin/screens/${screen.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });
      await load();
    });
  }

  // Commits the freeform grid editor's local draft -- called on blur so
  // typing a new column count doesn't fire a save per keystroke.
  async function handleGridDraftBlur() {
    if (!screen) return;
    if (
      gridDraft.columns === screen.columns &&
      gridDraft.row_height === screen.row_height &&
      gridDraft.gap === screen.gap
    ) {
      return;
    }
    const columns = Math.max(1, gridDraft.columns || 1);
    const rowHeight = Math.max(1, gridDraft.row_height || 1);
    const gap = Math.max(0, gridDraft.gap || 0);
    await withSaveStatus(async () => {
      await apiFetch(`/api/admin/screens/${screen.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: screen.name, position: screen.position, layout_mode: screen.layout_mode,
          columns, row_height: rowHeight, gap,
        }),
      });
      await load();
    });
  }

  async function handleDeleteCard(cardId: number) {
    if (!confirm('Remove this card?')) return;
    await withSaveStatus(async () => {
      await apiFetch(`/api/admin/cards/${cardId}`, { method: 'DELETE' });
      if (selectedCardId === cardId) setSelectedCardId(null);
      await load();
    });
  }

  async function handleSaveCardSettings(
    card: Card,
    values: { data_plugin_instance_id: number | null; config: unknown; theme_override: Partial<ThemeTokens> | null },
  ) {
    await withSaveStatus(async () => {
      const res = await apiFetch(`/api/admin/cards/${card.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          ui_plugin_id: card.ui_plugin_id,
          data_plugin_instance_id: values.data_plugin_instance_id,
          x: card.x,
          y: card.y,
          w: card.w,
          h: card.h,
          config: values.config,
          theme_override: values.theme_override,
        }),
      });
      if (!res.ok) {
        throw new Error(await readErrorMessage(res, 'Save failed'));
      }
      setSelectedCardId(null);
      await load();
    });
  }

  if (loading || !screen) {
    return <p>Loading…</p>;
  }

  const selectedCard = cards.find((c) => c.id === selectedCardId) ?? null;

  return (
    <div className="designer-page">
      <div className="designer-toolbar">
        <button className="designer-back" onClick={() => navigate('/admin/displays')} title="Back to Displays">
          ←
        </button>
        <div className="designer-breadcrumb">
          <span className="designer-breadcrumb-display">{display?.name ?? 'Display'}</span>
          <span className="designer-breadcrumb-chevron">›</span>
          <span className="designer-breadcrumb-screen">{screen.name}</span>
        </div>
        {screens.length > 1 && (
          <div className="designer-screen-tabs">
            {screens.map((s) => (
              <button
                key={s.id}
                className={`designer-screen-tab${s.id === screen.id ? ' active' : ''}`}
                onClick={() => s.id !== screen.id && navigate(`/admin/displays/${displayId}/screens/${s.id}/design`)}
              >
                {s.name}
              </button>
            ))}
          </div>
        )}
        <div className="designer-toolbar-spacer" />
        {saveStatus !== 'idle' && (
          <span className={`designer-save-status designer-save-status-${saveStatus}`}>
            {saveStatus === 'saving' && 'Saving…'}
            {saveStatus === 'saved' && 'Saved'}
            {saveStatus === 'error' && 'Save failed'}
          </span>
        )}
        {screen.layout_mode === 'freeform' ? (
          <div className="designer-grid-editor">
            <label>
              <span className="kicker">Cols</span>
              <input
                type="number"
                min={1}
                value={gridDraft.columns}
                onChange={(e) => setGridDraft((v) => ({ ...v, columns: Number(e.target.value) }))}
                onBlur={handleGridDraftBlur}
              />
            </label>
            <label>
              <span className="kicker">Row px</span>
              <input
                type="number"
                min={1}
                value={gridDraft.row_height}
                onChange={(e) => setGridDraft((v) => ({ ...v, row_height: Number(e.target.value) }))}
                onBlur={handleGridDraftBlur}
              />
            </label>
            <label>
              <span className="kicker">Gap px</span>
              <input
                type="number"
                min={0}
                value={gridDraft.gap}
                onChange={(e) => setGridDraft((v) => ({ ...v, gap: Number(e.target.value) }))}
                onBlur={handleGridDraftBlur}
              />
            </label>
          </div>
        ) : (
          <p className="designer-subtitle">Simple layout &middot; {screen.columns}-column grid</p>
        )}
        <button className="btn-secondary designer-mode-toggle" onClick={handleToggleLayoutMode}>
          {screen.layout_mode === 'simple' ? 'Switch to advanced layout' : 'Switch to simple layout'}
        </button>
      </div>

      <div className="designer-layout">
        <aside className="designer-palette">
          <h2>UI Plugins</h2>
          <p className="palette-help">
            Drag onto the grid to place a card, or double-click to add it instantly
            {screen.layout_mode === 'simple' ? ' at Medium size.' : '.'}
          </p>
          {uiPlugins.map((plugin) => (
            <div
              key={plugin.id}
              className="palette-item"
              draggable
              onDragStart={(e) => {
                draggingPluginRef.current = plugin.id;
                e.dataTransfer.effectAllowed = 'copy';
                e.dataTransfer.setData('text/plain', plugin.id);
              }}
              onDoubleClick={() => handlePaletteDoubleClick(plugin.id)}
            >
              {plugin.name}
            </div>
          ))}
        </aside>

        <div
          className="designer-grid-wrap"
          ref={containerRef}
          style={{ backgroundImage: gridlineBackground(screen.columns, screen.row_height, screen.gap) }}
        >
          {mounted && (
            <ReactGridLayout
              layout={layout}
              width={width}
              gridConfig={{ cols: screen.columns, rowHeight: screen.row_height, margin: [screen.gap, screen.gap] }}
              dragConfig={{ cancel: '.card-settings-btn, .card-delete-btn, .card-size-picker' }}
              resizeConfig={{
                // Simple mode has no resize handles at all -- size only
                // changes via the S/M/L picker on each card. Freeform
                // keeps today's full 8-handle arbitrary resize, unchanged.
                enabled: screen.layout_mode === 'freeform',
                handles: ['se', 'sw', 'ne', 'nw', 'e', 'w', 'n', 's'],
              }}
              dropConfig={{
                enabled: true,
                defaultItem: DEFAULT_CARD_SIZE,
                // In simple mode, a drag-placed card lands at the
                // dragged widget's own Medium preset instead of the
                // fixed placeholder size -- freeform (the "undefined"
                // path) is left exactly as it was.
                onDragOver: () => {
                  if (screen.layout_mode !== 'simple' || !draggingPluginRef.current) return undefined;
                  return presetSize(getUIPlugin(draggingPluginRef.current), 'M', screen.columns);
                },
              }}
              onDragStop={(finalLayout) => persistPositions(finalLayout)}
              onResizeStop={(finalLayout) => persistPositions(finalLayout)}
              onDrop={handleDrop}
            >
              {cards.map((card) => {
                const plugin = getUIPlugin(card.ui_plugin_id);
                const activePreset =
                  screen.layout_mode === 'simple'
                    ? SIZE_PRESETS.find((p) => {
                        const s = presetSize(plugin, p, screen.columns);
                        return s.w === card.w && s.h === card.h;
                      })
                    : undefined;
                return (
                  <div key={String(card.id)} className="designer-card">
                    <div className="designer-card-label">{plugin?.name ?? card.ui_plugin_id}</div>
                    <div className="designer-card-source">{dataSourceLabel(card, instances)}</div>
                    <button className="card-settings-btn" title="Settings" onClick={() => setSelectedCardId(card.id)}>
                      ⚙
                    </button>
                    <button className="card-delete-btn" title="Remove" onClick={() => handleDeleteCard(card.id)}>
                      ✕
                    </button>
                    {screen.layout_mode === 'simple' && (
                      <div className="card-size-picker">
                        {SIZE_PRESETS.map((p) => (
                          <button
                            key={p}
                            className={`card-size-btn${activePreset === p ? ' active' : ''}`}
                            title={SIZE_PRESET_NAMES[p]}
                            onClick={() => handleResizeCardPreset(card, p)}
                          >
                            {p}
                          </button>
                        ))}
                      </div>
                    )}
                  </div>
                );
              })}
            </ReactGridLayout>
          )}
        </div>
      </div>

      {selectedCard && (
        <div className="modal-scrim">
          <div className="modal-panel">
            <CardSettingsPanel
              card={selectedCard}
              uiPlugin={getUIPlugin(selectedCard.ui_plugin_id)}
              instances={instances}
              manifests={manifests}
              onSave={(values) => handleSaveCardSettings(selectedCard, values)}
              onCancel={() => setSelectedCardId(null)}
            />
          </div>
        </div>
      )}
    </div>
  );
}

interface CardSettingsPanelProps {
  card: Card;
  uiPlugin: ReturnType<typeof getUIPlugin>;
  instances: PluginInstance[];
  manifests: PluginManifest[];
  onSave: (values: { data_plugin_instance_id: number | null; config: unknown; theme_override: Partial<ThemeTokens> | null }) => Promise<void>;
  onCancel: () => void;
}

// Card-level overrides stay narrow by design (see architecture.md
// "Theme Cascade"): a card can nudge its own background and accent, not
// take over the whole display's look (font, spacing, etc.).
const CARD_OVERRIDE_FIELDS: (keyof ThemeTokens)[] = ['cardBackground', 'accentColor', 'opacity'];

// One bound input per configSchema entry -- mirrors the shape of
// plugindata.SetupField's own admin-side rendering (ManifestForm.tsx)
// for a data plugin's setup form, just keyed on a UI plugin's
// ConfigField instead. Replaces what used to be a raw "paste JSON"
// textarea: every value here is guaranteed to match what the widget's
// own component actually reads out of `config`, since it's driven by
// the same configSchema the widget declares.
function ConfigFieldInput({
  fieldKey,
  field,
  value,
  onChange,
}: {
  fieldKey: string;
  field: ConfigField;
  value: unknown;
  onChange: (v: unknown) => void;
}) {
  switch (field.type) {
    case 'toggle':
      return (
        <label className="toggle-row" key={fieldKey}>
          <span>{field.label}</span>
          <input type="checkbox" checked={Boolean(value)} onChange={(e) => onChange(e.target.checked)} />
        </label>
      );
    case 'select':
      return (
        <label className="field" key={fieldKey}>
          <span className="kicker">{field.label}</span>
          <select value={String(value ?? '')} onChange={(e) => onChange(e.target.value)}>
            {(field.options ?? []).map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </select>
          {field.helpText && <span className="field-help">{field.helpText}</span>}
        </label>
      );
    case 'multi-select': {
      const selected = Array.isArray(value) ? value.map(String) : [];
      return (
        <label className="field" key={fieldKey}>
          <span className="kicker">{field.label}</span>
          <select
            multiple
            value={selected}
            onChange={(e) => onChange(Array.from(e.target.selectedOptions).map((o) => o.value))}
          >
            {(field.options ?? []).map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </select>
          {field.helpText && <span className="field-help">{field.helpText}</span>}
        </label>
      );
    }
    case 'number':
      return (
        <label className="field" key={fieldKey}>
          <span className="kicker">{field.label}</span>
          <input
            type="number"
            value={typeof value === 'number' ? value : ''}
            onChange={(e) => onChange(e.target.value === '' ? undefined : Number(e.target.value))}
          />
          {field.helpText && <span className="field-help">{field.helpText}</span>}
        </label>
      );
    case 'color':
      return (
        <label className="field" key={fieldKey}>
          <span className="kicker">{field.label}</span>
          <input type="color" value={typeof value === 'string' ? value : '#000000'} onChange={(e) => onChange(e.target.value)} />
          {field.helpText && <span className="field-help">{field.helpText}</span>}
        </label>
      );
    case 'text':
    default:
      return (
        <label className="field" key={fieldKey}>
          <span className="kicker">{field.label}</span>
          <input type="text" value={typeof value === 'string' ? value : ''} onChange={(e) => onChange(e.target.value)} />
          {field.helpText && <span className="field-help">{field.helpText}</span>}
        </label>
      );
  }
}

function CardSettingsPanel({ card, uiPlugin, instances, manifests, onSave, onCancel }: CardSettingsPanelProps) {
  const [dataPluginInstanceId, setDataPluginInstanceId] = useState<number | null>(card.data_plugin_instance_id ?? null);
  const configSchema = useMemo(() => uiPlugin?.configSchema ?? {}, [uiPlugin]);
  const [configValues, setConfigValues] = useState<Record<string, unknown>>(() => {
    const initial: Record<string, unknown> = {};
    for (const [key, field] of Object.entries(configSchema)) {
      const existing = (card.config as Record<string, unknown> | undefined)?.[key];
      initial[key] = existing ?? field.default;
    }
    return initial;
  });
  const [overrideEnabled, setOverrideEnabled] = useState(card.theme_override != null);
  const [themeOverride, setThemeOverride] = useState<Partial<ThemeTokens>>(card.theme_override ?? {});
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const compatibleInstances = useMemo(() => {
    if (!uiPlugin?.dataShape) return [];
    const manifestsById = new Map(manifests.map((m) => [m.id, m]));
    return instances.filter((inst) => manifestsById.get(inst.plugin_id)?.data_shapes?.includes(uiPlugin.dataShape));
  }, [instances, manifests, uiPlugin]);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await onSave({
        data_plugin_instance_id: dataPluginInstanceId,
        config: configValues,
        theme_override: overrideEnabled ? themeOverride : null,
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <form className="manifest-form" onSubmit={handleSubmit}>
      <h2>{uiPlugin?.name ?? card.ui_plugin_id}</h2>
      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}

      {uiPlugin?.dataShape ? (
        <label className="field">
          <span className="kicker">Data source</span>
          <select
            value={dataPluginInstanceId ?? ''}
            onChange={(e) => setDataPluginInstanceId(e.target.value === '' ? null : Number(e.target.value))}
          >
            <option value="">None</option>
            {compatibleInstances.map((inst) => (
              <option key={inst.id} value={inst.id}>
                {inst.instance_name}
              </option>
            ))}
          </select>
          {compatibleInstances.length === 0 && (
            <span className="field-help">No configured plugin instance produces {uiPlugin.dataShape} data yet.</span>
          )}
        </label>
      ) : (
        <p className="field-help">This widget doesn&rsquo;t use a data source.</p>
      )}

      {Object.keys(configSchema).length === 0 ? (
        <p className="field-help">This widget has no configurable options.</p>
      ) : (
        Object.entries(configSchema).map(([key, field]) => (
          <ConfigFieldInput
            key={key}
            fieldKey={key}
            field={field}
            value={configValues[key]}
            onChange={(v) => setConfigValues((prev) => ({ ...prev, [key]: v }))}
          />
        ))
      )}

      <label className="toggle-row">
        <span>Override theme for this card</span>
        <input type="checkbox" checked={overrideEnabled} onChange={(e) => setOverrideEnabled(e.target.checked)} />
      </label>
      {overrideEnabled && (
        <ThemeTokenFields values={themeOverride} onChange={(v) => setThemeOverride((prev) => ({ ...prev, ...v }))} fields={CARD_OVERRIDE_FIELDS} />
      )}

      <div className="manifest-form-actions">
        <button type="button" className="btn-secondary" onClick={onCancel}>
          Cancel
        </button>
        <button type="submit" className="btn-primary" disabled={submitting}>
          {submitting ? 'Saving…' : 'Save changes'}
        </button>
      </div>
    </form>
  );
}
