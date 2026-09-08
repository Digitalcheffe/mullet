import { useCallback, useEffect, useMemo, useRef, useState, type FormEvent } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import ReactGridLayout, { useContainerWidth, type Layout, type LayoutItem } from 'react-grid-layout';
import 'react-grid-layout/css/styles.css';
import 'react-resizable/css/styles.css';
import { useApiFetch } from '../auth/useApiFetch';
import ThemeTokenFields from '../components/ThemeTokenFields';
import type { ThemeTokens } from '../../shared/themes/tokens';
import './DesignerPage.css';

interface Screen {
  id: number;
  display_id: number;
  name: string;
  columns: number;
  row_height: number;
  gap: number;
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

interface PaletteItem {
  uiPluginId: string;
  label: string;
  // The data shape this widget reads, or null if it needs no data
  // source (e.g. clock reads the browser's own time). No server-side UI
  // plugin registry exists yet (see architecture.md), so this palette
  // is a hand-maintained list of the widgets a card *could* be, scoped
  // to shapes a compiled-in data plugin can actually produce today.
  dataShape: string | null;
}

const PALETTE: PaletteItem[] = [
  { uiPluginId: 'clock', label: 'Clock', dataShape: null },
  { uiPluginId: 'weather-current', label: 'Weather (Current)', dataShape: 'weather_current' },
  { uiPluginId: 'weather-forecast', label: 'Weather (Forecast)', dataShape: 'weather_forecast' },
  { uiPluginId: 'calendar-agenda', label: 'Calendar Agenda', dataShape: 'events' },
];

const DEFAULT_CARD_SIZE = { w: 4, h: 3 };

function paletteItemFor(uiPluginId: string): PaletteItem | undefined {
  return PALETTE.find((p) => p.uiPluginId === uiPluginId);
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
  const { screenId } = useParams<{ displayId: string; screenId: string }>();
  const navigate = useNavigate();
  const apiFetch = useApiFetch();
  const { width, containerRef, mounted } = useContainerWidth();

  const [screen, setScreen] = useState<Screen | null>(null);
  const [cards, setCards] = useState<Card[]>([]);
  const [manifests, setManifests] = useState<PluginManifest[]>([]);
  const [instances, setInstances] = useState<PluginInstance[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedCardId, setSelectedCardId] = useState<number | null>(null);
  const draggingPluginRef = useRef<string | null>(null);

  const load = useCallback(() => {
    return Promise.all([
      apiFetch(`/api/admin/screens/${screenId}`).then((r) => r.json()),
      apiFetch(`/api/admin/screens/${screenId}/cards`).then((r) => r.json()),
      apiFetch('/api/admin/plugins').then((r) => r.json()),
      apiFetch('/api/admin/plugins/instances').then((r) => r.json()),
    ]).then(([s, c, m, i]) => {
      setScreen(s);
      setCards(c);
      setManifests(m);
      setInstances(i);
      setLoading(false);
    });
  }, [apiFetch, screenId]);

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
  }

  async function handleDrop(_finalLayout: Layout, item: LayoutItem | undefined) {
    const uiPluginId = draggingPluginRef.current;
    draggingPluginRef.current = null;
    if (!uiPluginId || !item) return;

    await apiFetch(`/api/admin/screens/${screenId}/cards`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        ui_plugin_id: uiPluginId,
        data_plugin_instance_id: null,
        x: item.x,
        y: item.y,
        w: item.w,
        h: item.h,
        config: {},
        theme_override: null,
      }),
    });
    await load();
  }

  async function handleDeleteCard(cardId: number) {
    if (!confirm('Remove this card?')) return;
    await apiFetch(`/api/admin/cards/${cardId}`, { method: 'DELETE' });
    if (selectedCardId === cardId) setSelectedCardId(null);
    load();
  }

  async function handleSaveCardSettings(
    card: Card,
    values: { data_plugin_instance_id: number | null; config: unknown; theme_override: Partial<ThemeTokens> | null },
  ) {
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
  }

  if (loading || !screen) {
    return <p>Loading…</p>;
  }

  const selectedCard = cards.find((c) => c.id === selectedCardId) ?? null;

  return (
    <div className="designer-page">
      <div className="designer-header">
        <button className="btn-secondary" onClick={() => navigate('/admin/displays')}>
          ← Back to Displays
        </button>
        <div>
          <h1>{screen.name}</h1>
          <p className="designer-subtitle">
            {screen.columns} columns &middot; {screen.row_height}px rows &middot; {screen.gap}px gap
          </p>
        </div>
      </div>

      <div className="designer-layout">
        <aside className="designer-palette">
          <h2>UI Plugins</h2>
          <p className="palette-help">Drag onto the grid to place a card.</p>
          {PALETTE.map((item) => (
            <div
              key={item.uiPluginId}
              className="palette-item"
              draggable
              onDragStart={(e) => {
                draggingPluginRef.current = item.uiPluginId;
                e.dataTransfer.effectAllowed = 'copy';
                e.dataTransfer.setData('text/plain', item.uiPluginId);
              }}
            >
              {item.label}
            </div>
          ))}
        </aside>

        <div className="designer-grid-wrap" ref={containerRef}>
          {mounted && (
            <ReactGridLayout
              layout={layout}
              width={width}
              gridConfig={{ cols: screen.columns, rowHeight: screen.row_height, margin: [screen.gap, screen.gap] }}
              dragConfig={{ cancel: '.card-settings-btn, .card-delete-btn' }}
              resizeConfig={{ handles: ['se', 'sw', 'ne', 'nw', 'e', 'w', 'n', 's'] }}
              dropConfig={{ enabled: true, defaultItem: DEFAULT_CARD_SIZE }}
              onDragStop={(finalLayout) => persistPositions(finalLayout)}
              onResizeStop={(finalLayout) => persistPositions(finalLayout)}
              onDrop={handleDrop}
            >
              {cards.map((card) => {
                const palette = paletteItemFor(card.ui_plugin_id);
                return (
                  <div key={String(card.id)} className="designer-card">
                    <div className="designer-card-label">{palette?.label ?? card.ui_plugin_id}</div>
                    <div className="designer-card-source">{dataSourceLabel(card, instances)}</div>
                    <button className="card-settings-btn" title="Settings" onClick={() => setSelectedCardId(card.id)}>
                      ⚙
                    </button>
                    <button className="card-delete-btn" title="Remove" onClick={() => handleDeleteCard(card.id)}>
                      ✕
                    </button>
                  </div>
                );
              })}
            </ReactGridLayout>
          )}
        </div>
      </div>

      {selectedCard && (
        <div className="modal-scrim" onClick={() => setSelectedCardId(null)}>
          <div className="modal-panel" onClick={(e) => e.stopPropagation()}>
            <CardSettingsPanel
              card={selectedCard}
              paletteItem={paletteItemFor(selectedCard.ui_plugin_id)}
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
  paletteItem: PaletteItem | undefined;
  instances: PluginInstance[];
  manifests: PluginManifest[];
  onSave: (values: { data_plugin_instance_id: number | null; config: unknown; theme_override: Partial<ThemeTokens> | null }) => Promise<void>;
  onCancel: () => void;
}

// Card-level overrides stay narrow by design (see architecture.md
// "Theme Cascade"): a card can nudge its own background and accent, not
// take over the whole display's look (font, spacing, etc.).
const CARD_OVERRIDE_FIELDS: (keyof ThemeTokens)[] = ['cardBackground', 'accentColor', 'opacity'];

function CardSettingsPanel({ card, paletteItem, instances, manifests, onSave, onCancel }: CardSettingsPanelProps) {
  const [dataPluginInstanceId, setDataPluginInstanceId] = useState<number | null>(card.data_plugin_instance_id ?? null);
  const [configText, setConfigText] = useState(JSON.stringify(card.config ?? {}, null, 2));
  const [overrideEnabled, setOverrideEnabled] = useState(card.theme_override != null);
  const [themeOverride, setThemeOverride] = useState<Partial<ThemeTokens>>(card.theme_override ?? {});
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const compatibleInstances = useMemo(() => {
    if (!paletteItem?.dataShape) return [];
    const manifestsById = new Map(manifests.map((m) => [m.id, m]));
    return instances.filter((inst) => manifestsById.get(inst.plugin_id)?.data_shapes?.includes(paletteItem.dataShape!));
  }, [instances, manifests, paletteItem]);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);

    let config: unknown = {};
    if (configText.trim() !== '') {
      try {
        config = JSON.parse(configText);
      } catch {
        setError('Config must be valid JSON');
        return;
      }
    }

    setSubmitting(true);
    try {
      await onSave({
        data_plugin_instance_id: dataPluginInstanceId,
        config,
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
      <h2>{paletteItem?.label ?? card.ui_plugin_id}</h2>
      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}

      {paletteItem?.dataShape ? (
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
            <span className="field-help">No configured plugin instance produces {paletteItem.dataShape} data yet.</span>
          )}
        </label>
      ) : (
        <p className="field-help">This widget doesn&rsquo;t use a data source.</p>
      )}

      <label className="field">
        <span className="kicker">Config (JSON)</span>
        <textarea rows={5} value={configText} onChange={(e) => setConfigText(e.target.value)} spellCheck={false} />
      </label>

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
