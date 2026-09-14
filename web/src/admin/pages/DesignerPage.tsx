import { useCallback, useEffect, useMemo, useRef, useState, type FormEvent } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import ReactGridLayout, { type Layout, type LayoutItem } from 'react-grid-layout';
import 'react-grid-layout/css/styles.css';
import 'react-resizable/css/styles.css';
import { useApiFetch } from '../auth/useApiFetch';
import ThemeTokenFields, { ColorField } from '../components/ThemeTokenFields';
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
  theme_override?: Partial<ThemeTokens>;
  // A Designer-canvas preview hint (issue #162) -- e.g. "16:9" -- absent
  // means "Custom" (today's free-form behavior). Never constrains
  // columns/row_height/gap, which stay independently configurable.
  aspect_ratio?: string;
}

interface Card {
  id: number;
  screen_id: number;
  ui_plugin_id: string;
  data_plugin_instance_id?: number;
  // Present only for a multi-source card (issue #97) -- see
  // UIPlugin.supportsMultiDataSource.
  data_plugin_instance_ids?: number[];
  x: number;
  y: number;
  w: number;
  h: number;
  config: Record<string, unknown>;
  theme_override?: Partial<ThemeTokens>;
  // Optional card header (issue #145) -- absent/empty header_text means
  // no header renders at all. Always renders in its own reserved strip
  // at the top of the card (issue #154) -- header_halign is the only
  // alignment control.
  header_text?: string;
  header_halign?: 'left' | 'center' | 'right';
  // Positions a widget's own content within its card (issue #149) --
  // absent/'center' on both axes is today's stretch-to-fill behavior.
  content_halign?: 'left' | 'center' | 'right';
  content_valign?: 'top' | 'center' | 'bottom';
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

// Screen-preset dropdown (issue #162). These only set a CSS aspect-ratio
// on the canvas as a sizing preview -- real display hardware varies even
// within one ratio class, so columns/row_height/gap stay independently
// editable no matter which preset (or "Custom") is selected.
const ASPECT_RATIO_PRESETS: { value: string; label: string }[] = [
  { value: '', label: 'Custom' },
  { value: '16:9', label: '16:9 (TV / Monitor)' },
  { value: '4:3', label: '4:3 (Tablet Landscape)' },
  { value: '9:16', label: '9:16 (Tablet Portrait)' },
  { value: '21:9', label: '21:9 (Ultrawide)' },
];

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
// grid you're placing things onto at all. Both row and column lines
// line up exactly with react-grid-layout's own math: each row is
// rowHeight + gap pixels, and each column is (containerWidth -
// gap*(cols+1))/cols + gap pixels (RGL divides the gap-reduced width
// evenly, then adds one gap back per column) -- containerWidth is
// needed to get this right in absolute pixels; a pure percentage
// split (100/cols) ignores the margin RGL subtracts per column and
// drifts from the real column edges by a growing number of pixels
// toward the grid's left edge, which read as cards placed "off grid"
// even though their saved x/y were always correct (issue #165).
function gridlineBackground(cols: number, rowHeight: number, gap: number, containerWidth: number): string {
  const rowPitch = rowHeight + gap;
  const rowGradient = `repeating-linear-gradient(to bottom, transparent 0, transparent ${rowPitch - 1}px, var(--border) ${rowPitch - 1}px, var(--border) ${rowPitch}px)`;
  // Before the canvas has been measured (containerWidth is still 0 on
  // first render), fall back to the old percentage approximation rather
  // than a negative/garbage pixel value -- it's replaced within a frame
  // once useContainerWidth() reports a real width.
  if (containerWidth <= 0) {
    const colPct = 100 / cols;
    return [
      `repeating-linear-gradient(to right, transparent 0, transparent calc(${colPct}% - 1px), var(--border) calc(${colPct}% - 1px), var(--border) ${colPct}%)`,
      rowGradient,
    ].join(', ');
  }
  const colWidth = (containerWidth - gap * (cols + 1)) / cols;
  const colPitch = colWidth + gap;
  return [
    `repeating-linear-gradient(to right, transparent 0, transparent ${colPitch - 1}px, var(--border) ${colPitch - 1}px, var(--border) ${colPitch}px)`,
    rowGradient,
  ].join(', ');
}

function dataSourceLabel(card: Card, instances: PluginInstance[]): string {
  const ids = card.data_plugin_instance_ids?.length ? card.data_plugin_instance_ids : card.data_plugin_instance_id != null ? [card.data_plugin_instance_id] : [];
  if (ids.length === 0) return 'No data source';
  return ids.map((id) => instances.find((i) => i.id === id)?.instance_name ?? 'Unknown source').join(', ');
}

async function readErrorMessage(res: Response, fallback: string): Promise<string> {
  const text = await res.text();
  return text || fallback;
}

export default function DesignerPage() {
  const { displayId, screenId } = useParams<{ displayId: string; screenId: string }>();
  const navigate = useNavigate();
  const apiFetch = useApiFetch();
  // react-grid-layout's own useContainerWidth() (issue #165): the
  // canvas div it measures only exists once `screen` has loaded, which
  // happens well after this component's first commit. An effect keyed
  // off a stable/empty dependency array (their hook's, and an earlier
  // version of this one) only ever runs against that first commit, when
  // the ref is still null -- by the time the real div mounts later,
  // nothing re-triggers it, so width stays stuck at the hook's 1280px
  // fallback forever, silently mismatched against the canvas's real
  // (usually much narrower) rendered width. RGL then positions every
  // card using that wrong 1280px column math, so cards land somewhere
  // other than where the visible grid (and the mouse) says they should
  // -- exactly the "wiggling off grid" bug reported. A callback ref
  // sidesteps this: React invokes it exactly when the node actually
  // attaches, however late that is, so the ResizeObserver always ends
  // up on a real, current node.
  const [width, setWidth] = useState(0);
  const resizeObserverRef = useRef<ResizeObserver | null>(null);
  const containerRef = useCallback((node: HTMLDivElement | null) => {
    resizeObserverRef.current?.disconnect();
    resizeObserverRef.current = null;
    if (!node) return;
    const observer = new ResizeObserver((entries) => {
      const w = entries[0]?.contentRect.width;
      if (w != null) setWidth(Math.round(w));
    });
    observer.observe(node);
    resizeObserverRef.current = observer;
    setWidth(Math.round(node.getBoundingClientRect().width));
  }, []);
  const mounted = width > 0;

  const [display, setDisplay] = useState<Display | null>(null);
  const [screens, setScreens] = useState<Screen[]>([]);
  const [screen, setScreen] = useState<Screen | null>(null);
  const [cards, setCards] = useState<Card[]>([]);
  const [manifests, setManifests] = useState<PluginManifest[]>([]);
  const [instances, setInstances] = useState<PluginInstance[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedCardId, setSelectedCardId] = useState<number | null>(null);
  const [screenSettingsOpen, setScreenSettingsOpen] = useState(false);
  // Collapsing the palette (issue #160) gives the canvas the full
  // available height -- not persisted, since which screen someone's
  // actively placing cards on changes far more often than a sidebar's
  // pin state does.
  const [paletteCollapsed, setPaletteCollapsed] = useState(false);
  // Legibility warning (issue #162) -- dismissible per selected preset;
  // re-armed whenever the preset itself changes so switching to a
  // different (or smaller) preset surfaces the warning again.
  const [legibilityWarningDismissed, setLegibilityWarningDismissed] = useState(false);
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

  // Re-arm the legibility warning whenever the selected preset changes,
  // so a dismissal on one preset doesn't silently suppress it on another.
  useEffect(() => {
    setLegibilityWarningDismissed(false);
  }, [screen?.aspect_ratio]);

  // A card counts as "at its smallest usable size" once it's at or below
  // its own widget's declared minSize on both axes -- the same minSize
  // the resize system already enforces as a floor, so this never flags a
  // card the user couldn't have avoided anyway.
  const undersizedCardCount = useMemo(() => {
    if (!screen?.aspect_ratio || legibilityWarningDismissed) return 0;
    return cards.filter((c) => {
      const plugin = getUIPlugin(c.ui_plugin_id);
      if (!plugin) return false;
      return c.w <= plugin.minSize.w && c.h <= plugin.minSize.h;
    }).length;
  }, [cards, screen?.aspect_ratio, legibilityWarningDismissed]);

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
    // theme_override is echoed back too -- PUT has no partial-update
    // path (same reason grid fields are echoed below), so omitting it
    // here would silently clear a screen's font override just from
    // toggling its layout mode.
    const body = { name: screen.name, position: screen.position, layout_mode: nextMode, ...grid, theme_override: screen.theme_override ?? null, aspect_ratio: screen.aspect_ratio ?? null };
    await withSaveStatus(async () => {
      await apiFetch(`/api/admin/screens/${screen.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });
      await load();
    });
  }

  // Screen-preset dropdown (issue #162) -- purely a Designer-canvas
  // preview hint (CSS aspect-ratio on .designer-grid-wrap). Doesn't touch
  // columns/row_height/gap, which stay independently editable regardless
  // of which preset (or "Custom") is selected.
  async function handleChangeAspectRatio(value: string) {
    if (!screen) return;
    const aspectRatio = value === '' ? null : value;
    await withSaveStatus(async () => {
      await apiFetch(`/api/admin/screens/${screen.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: screen.name, position: screen.position, layout_mode: screen.layout_mode,
          columns: screen.columns, row_height: screen.row_height, gap: screen.gap,
          theme_override: screen.theme_override ?? null,
          aspect_ratio: aspectRatio,
        }),
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
          theme_override: screen.theme_override ?? null,
          aspect_ratio: screen.aspect_ratio ?? null,
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
    values: {
      data_plugin_instance_id: number | null;
      data_plugin_instance_ids?: number[];
      config: unknown;
      theme_override: Partial<ThemeTokens> | null;
      header_text: string | null;
      header_halign: 'left' | 'center' | 'right' | null;
      content_halign: 'left' | 'center' | 'right' | null;
      content_valign: 'top' | 'center' | 'bottom' | null;
    },
  ) {
    await withSaveStatus(async () => {
      const res = await apiFetch(`/api/admin/cards/${card.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          ui_plugin_id: card.ui_plugin_id,
          data_plugin_instance_id: values.data_plugin_instance_id,
          // Omitted (not just empty) for a single-source widget -- the
          // backend leaves card_data_sources untouched when this key is
          // absent, only replacing it when a multi-source widget
          // explicitly sends the full set (see cardRequest's doc
          // comment in internal/api/display_handlers.go).
          ...(values.data_plugin_instance_ids ? { data_plugin_instance_ids: values.data_plugin_instance_ids } : {}),
          x: card.x,
          y: card.y,
          w: card.w,
          h: card.h,
          config: values.config,
          theme_override: values.theme_override,
          header_text: values.header_text,
          header_halign: values.header_halign,
          content_halign: values.content_halign,
          content_valign: values.content_valign,
        }),
      });
      if (!res.ok) {
        throw new Error(await readErrorMessage(res, 'Save failed'));
      }
      setSelectedCardId(null);
      await load();
    });
  }

  // Mirrors handleSaveCardSettings, but for the screen-level font
  // override (issue #87) -- echoes back every other editable field
  // (same reason handleToggleLayoutMode/handleGridDraftBlur do) since
  // PUT has no partial-update path.
  async function handleSaveScreenSettings(themeOverride: Partial<ThemeTokens> | null) {
    if (!screen) return;
    await withSaveStatus(async () => {
      const res = await apiFetch(`/api/admin/screens/${screen.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: screen.name,
          position: screen.position,
          layout_mode: screen.layout_mode,
          columns: screen.columns,
          row_height: screen.row_height,
          gap: screen.gap,
          theme_override: themeOverride,
          aspect_ratio: screen.aspect_ratio ?? null,
        }),
      });
      if (!res.ok) {
        throw new Error(await readErrorMessage(res, 'Save failed'));
      }
      setScreenSettingsOpen(false);
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
        <label className="designer-aspect-picker">
          <span className="kicker">Screen preset</span>
          <select value={screen.aspect_ratio ?? ''} onChange={(e) => handleChangeAspectRatio(e.target.value)}>
            {ASPECT_RATIO_PRESETS.map((p) => (
              <option key={p.value} value={p.value}>{p.label}</option>
            ))}
          </select>
        </label>
        <button className="btn-secondary designer-mode-toggle" onClick={handleToggleLayoutMode}>
          {screen.layout_mode === 'simple' ? 'Switch to advanced layout' : 'Switch to simple layout'}
        </button>
        <button className="btn-secondary" onClick={() => setScreenSettingsOpen(true)}>
          Screen font{screen.theme_override ? ' •' : ''}
        </button>
      </div>

      {undersizedCardCount > 0 && (
        <div className="designer-legibility-warning">
          <span>
            {undersizedCardCount} card{undersizedCardCount === 1 ? '' : 's'} at their smallest usable size on this{' '}
            {ASPECT_RATIO_PRESETS.find((p) => p.value === screen.aspect_ratio)?.label ?? 'preset'} screen -- they may be
            hard to read.
          </span>
          <button type="button" className="designer-legibility-dismiss" onClick={() => setLegibilityWarningDismissed(true)}>
            Dismiss
          </button>
        </div>
      )}

      <div className="designer-layout">
        <aside className={`designer-palette${paletteCollapsed ? ' collapsed' : ''}`}>
          <div className="designer-palette-header">
            <h2>UI Plugins</h2>
            <button
              type="button"
              className="designer-palette-collapse"
              onClick={() => setPaletteCollapsed((c) => !c)}
              title={paletteCollapsed ? 'Expand plugin list' : 'Collapse plugin list'}
              aria-label={paletteCollapsed ? 'Expand plugin list' : 'Collapse plugin list'}
            >
              ▾
            </button>
          </div>
          <p className="palette-help">
            Drag onto the grid to place a card, or double-click to add it instantly
            {screen.layout_mode === 'simple' ? ' at Medium size.' : '.'}
          </p>
          <div className="palette-items">
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
          </div>
        </aside>

        <h2 className="designer-canvas-title">Designer</h2>
        <div
          className="designer-grid-wrap"
          ref={containerRef}
          style={{
            backgroundImage: gridlineBackground(screen.columns, screen.row_height, screen.gap, width),
            // Custom (no preset) keeps the default CSS flex:1 fill-available-
            // space behavior untouched. A preset instead lets flexbox size
            // (and, if the viewport is short, shrink) the canvas from its
            // own aspect-ratio rather than forcing it to fill the remaining
            // column -- min-height/overflow-y from the stylesheet still
            // apply, so it can never grow past the viewport the way a fixed
            // min-height once did.
            ...(screen.aspect_ratio
              ? { flex: '0 1 auto', aspectRatio: screen.aspect_ratio.replace(':', ' / ') }
              : {}),
          }}
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
              apiFetch={apiFetch}
              onSave={(values) => handleSaveCardSettings(selectedCard, values)}
              onCancel={() => setSelectedCardId(null)}
            />
          </div>
        </div>
      )}

      {screenSettingsOpen && (
        <div className="modal-scrim">
          <div className="modal-panel">
            <ScreenSettingsPanel
              screen={screen}
              onSave={handleSaveScreenSettings}
              onCancel={() => setScreenSettingsOpen(false)}
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
  // Only needed for a `dynamic` configSchema field's own discover call
  // (issue #99) -- every other save/load path here already goes through
  // the parent's own withSaveStatus-wrapped handlers instead.
  apiFetch: ReturnType<typeof useApiFetch>;
  onSave: (values: {
    data_plugin_instance_id: number | null;
    data_plugin_instance_ids?: number[];
    config: unknown;
    theme_override: Partial<ThemeTokens> | null;
    header_text: string | null;
    header_halign: 'left' | 'center' | 'right' | null;
    content_halign: 'left' | 'center' | 'right' | null;
    content_valign: 'top' | 'center' | 'bottom' | null;
  }) => Promise<void>;
  onCancel: () => void;
}

const HEADER_HALIGNS = ['left', 'center', 'right'] as const;
const CONTENT_HALIGNS = ['left', 'center', 'right'] as const;
const CONTENT_VALIGNS = ['top', 'center', 'bottom'] as const;

// Card-level overrides stay narrow by design (see architecture.md
// "Theme Cascade"): a card can nudge its own background and accent, not
// take over the whole display's look (font, spacing, etc.).
const CARD_OVERRIDE_FIELDS: (keyof ThemeTokens)[] = ['cardBackground', 'accentColor', 'opacity'];

// The mirror image of CARD_OVERRIDE_FIELDS -- font is exactly the thing
// card overrides deliberately exclude, and is the one thing a screen
// override is for (issue #87): a screen can pick its own voice without
// needing a whole new theme, but still can't touch color/spacing.
const SCREEN_OVERRIDE_FIELDS: (keyof ThemeTokens)[] = ['fontFamily', 'headingFontFamily'];

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
  dynamicOptions,
  dynamicLoading,
}: {
  fieldKey: string;
  field: ConfigField;
  value: unknown;
  onChange: (v: unknown) => void;
  // Only meaningful when field.dynamic is set (issue #99) -- the
  // CardSettingsPanel-fetched options for a 'select' field sourced from
  // the card's own bound data plugin instance, instead of a fixed
  // `options` list.
  dynamicOptions?: { value: string; label: string }[];
  dynamicLoading?: boolean;
}) {
  switch (field.type) {
    case 'textarea':
      return (
        <label className="field" key={fieldKey}>
          <span className="kicker">{field.label}</span>
          <textarea
            rows={4}
            value={typeof value === 'string' ? value : ''}
            onChange={(e) => onChange(e.target.value)}
          />
          {field.helpText && <span className="field-help">{field.helpText}</span>}
        </label>
      );
    case 'toggle':
      return (
        <label className="toggle-row" key={fieldKey}>
          <span>{field.label}</span>
          <input type="checkbox" checked={Boolean(value)} onChange={(e) => onChange(e.target.checked)} />
        </label>
      );
    case 'select': {
      const options = field.dynamic ? (dynamicOptions ?? []) : (field.options ?? []);
      return (
        <label className="field" key={fieldKey}>
          <span className="kicker">{field.label}</span>
          <select value={String(value ?? '')} onChange={(e) => onChange(e.target.value)} disabled={field.dynamic && dynamicLoading}>
            <option value="">{field.dynamic && dynamicLoading ? 'Loading…' : 'None'}</option>
            {options.map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </select>
          {field.dynamic && !dynamicLoading && options.length === 0 && (
            <span className="field-help">No entities found -- save a data source on this card first, or check its Domains setting.</span>
          )}
          {field.helpText && <span className="field-help">{field.helpText}</span>}
        </label>
      );
    }
    case 'multi-select': {
      // Same "show what's actually selected" fix as ManifestForm.tsx's
      // multi-select (issue #99) -- a native <select multiple> alone
      // doesn't make the current selection legible once the option list
      // is long.
      const selected = Array.isArray(value) ? value.map(String) : [];
      const labelFor = (v: string) => field.options?.find((o) => o.value === v)?.label ?? v;
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
          {selected.length > 0 && (
            <div className="selected-chips">
              {selected.map((v) => (
                <span className="selected-chip" key={v}>
                  {labelFor(v)}
                  <button
                    type="button"
                    className="selected-chip-remove"
                    aria-label={`Remove ${labelFor(v)}`}
                    onClick={() => onChange(selected.filter((s) => s !== v))}
                  >
                    ×
                  </button>
                </span>
              ))}
            </div>
          )}
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
        <ColorField
          key={fieldKey}
          label={field.label}
          value={typeof value === 'string' ? value : '#000000'}
          onChange={onChange}
          helpText={field.helpText}
        />
      );
    case 'date':
      return (
        <label className="field" key={fieldKey}>
          <span className="kicker">{field.label}</span>
          <input type="date" value={typeof value === 'string' ? value : ''} onChange={(e) => onChange(e.target.value)} />
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

function CardSettingsPanel({ card, uiPlugin, instances, manifests, apiFetch, onSave, onCancel }: CardSettingsPanelProps) {
  const supportsMulti = uiPlugin?.supportsMultiDataSource === true;
  const [dataPluginInstanceId, setDataPluginInstanceId] = useState<number | null>(card.data_plugin_instance_id ?? null);
  const [dataPluginInstanceIds, setDataPluginInstanceIds] = useState<number[]>(
    () => card.data_plugin_instance_ids ?? (card.data_plugin_instance_id != null ? [card.data_plugin_instance_id] : []),
  );
  const configSchema = useMemo(() => uiPlugin?.configSchema ?? {}, [uiPlugin]);
  const [configValues, setConfigValues] = useState<Record<string, unknown>>(() => {
    const initial: Record<string, unknown> = {};
    for (const [key, field] of Object.entries(configSchema)) {
      const existing = (card.config as Record<string, unknown> | undefined)?.[key];
      initial[key] = existing ?? field.default;
    }
    return initial;
  });
  // Options for any `dynamic` configSchema field (issue #99, e.g. a
  // single-entity Home Assistant widget's "which entity" picker) --
  // fetched from the bound instance's own Discover, the same endpoint
  // a data plugin's own Dynamic SetupFields already use. Keyed by
  // configSchema key rather than a single flat value since more than
  // one dynamic field is possible in principle.
  const [dynamicOptions, setDynamicOptions] = useState<Record<string, { value: string; label: string }[]>>({});
  const [dynamicLoading, setDynamicLoading] = useState<Record<string, boolean>>({});
  useEffect(() => {
    const dynamicFields = Object.entries(configSchema).filter(([, field]) => field.dynamic);
    if (dynamicFields.length === 0 || dataPluginInstanceId == null) return;
    let cancelled = false;
    for (const [key, field] of dynamicFields) {
      const discoverField = field.dynamicField ?? key;
      setDynamicLoading((prev) => ({ ...prev, [key]: true }));
      apiFetch(`/api/admin/plugins/instances/${dataPluginInstanceId}/discover?field=${encodeURIComponent(discoverField)}`)
        .then((res) => (res.ok ? res.json() : []))
        .then((options: { value: string; label: string }[]) => {
          if (cancelled) return;
          setDynamicOptions((prev) => ({ ...prev, [key]: options }));
        })
        .catch(() => {
          if (!cancelled) setDynamicOptions((prev) => ({ ...prev, [key]: [] }));
        })
        .finally(() => {
          if (!cancelled) setDynamicLoading((prev) => ({ ...prev, [key]: false }));
        });
    }
    return () => {
      cancelled = true;
    };
  }, [configSchema, dataPluginInstanceId, apiFetch]);

  const [overrideEnabled, setOverrideEnabled] = useState(card.theme_override != null);
  const [themeOverride, setThemeOverride] = useState<Partial<ThemeTokens>>(card.theme_override ?? {});
  const [headerText, setHeaderText] = useState(card.header_text ?? '');
  const [headerHalign, setHeaderHalign] = useState<(typeof HEADER_HALIGNS)[number]>(card.header_halign ?? 'left');
  const [contentHalign, setContentHalign] = useState<(typeof CONTENT_HALIGNS)[number]>(card.content_halign ?? 'center');
  const [contentValign, setContentValign] = useState<(typeof CONTENT_VALIGNS)[number]>(card.content_valign ?? 'center');
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
      const trimmedHeaderText = headerText.trim();
      const headerFields = {
        header_text: trimmedHeaderText === '' ? null : trimmedHeaderText,
        header_halign: trimmedHeaderText === '' ? null : headerHalign,
        // 'center'/'center' is stored as null on both axes -- it's the
        // same as never having set alignment at all (DisplayCard only
        // shrinks a widget to fit once either axis differs from
        // 'center'), so there's no reason to persist it as a distinct value.
        content_halign: contentHalign === 'center' ? null : contentHalign,
        content_valign: contentValign === 'center' ? null : contentValign,
      };
      await onSave(
        supportsMulti
          ? {
              data_plugin_instance_id: dataPluginInstanceIds[0] ?? null,
              data_plugin_instance_ids: dataPluginInstanceIds,
              config: configValues,
              theme_override: overrideEnabled ? themeOverride : null,
              ...headerFields,
            }
          : {
              data_plugin_instance_id: dataPluginInstanceId,
              config: configValues,
              theme_override: overrideEnabled ? themeOverride : null,
              ...headerFields,
            },
      );
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
          <span className="kicker">{supportsMulti ? 'Data sources' : 'Data source'}</span>
          {supportsMulti ? (
            <select
              multiple
              value={dataPluginInstanceIds.map(String)}
              onChange={(e) => setDataPluginInstanceIds(Array.from(e.target.selectedOptions).map((o) => Number(o.value)))}
            >
              {compatibleInstances.map((inst) => (
                <option key={inst.id} value={inst.id}>
                  {inst.instance_name}
                </option>
              ))}
            </select>
          ) : (
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
          )}
          {compatibleInstances.length === 0 && (
            <span className="field-help">No configured plugin instance produces {uiPlugin.dataShape} data yet.</span>
          )}
          {supportsMulti && compatibleInstances.length > 0 && (
            <span className="field-help">Merges events from every selected source onto this one card. Ctrl/Cmd-click to select more than one.</span>
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
            dynamicOptions={dynamicOptions[key]}
            dynamicLoading={dynamicLoading[key]}
          />
        ))
      )}

      <label className="field">
        <span className="kicker">Header text</span>
        <input
          type="text"
          value={headerText}
          onChange={(e) => setHeaderText(e.target.value)}
          placeholder="e.g. Kitchen Calendar"
        />
        <span className="field-help">Optional label shown at the top of the card. Leave blank for no header.</span>
      </label>
      {headerText.trim() !== '' && (
        <div className="field">
          <span className="kicker">Header alignment</span>
          <div className="header-halign-row">
            {HEADER_HALIGNS.map((h) => (
              <button
                type="button"
                key={h}
                className={`header-halign-btn${headerHalign === h ? ' selected' : ''}`}
                onClick={() => setHeaderHalign(h)}
              >
                {h[0].toUpperCase() + h.slice(1)}
              </button>
            ))}
          </div>
        </div>
      )}

      <div className="field">
        <span className="kicker">Content position</span>
        <div className="content-align-grid">
          {CONTENT_VALIGNS.map((v) =>
            CONTENT_HALIGNS.map((h) => (
              <button
                type="button"
                key={`${v}-${h}`}
                className={`content-align-btn${contentValign === v && contentHalign === h ? ' selected' : ''}`}
                title={`${v}, ${h}`}
                aria-label={`Position content ${v} ${h}`}
                onClick={() => {
                  setContentValign(v);
                  setContentHalign(h);
                }}
              >
                <span className="content-align-dot" />
              </button>
            )),
          )}
        </div>
        <span className="field-help">
          Where this widget's content sits within the card, instead of stretching to fill it. Best for a simple
          widget like Clock, Countdown, or Quote of the Day &mdash; a list-based widget (calendar, tasks) will shrink
          to show all of its content rather than scroll.
        </span>
      </div>

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

interface ScreenSettingsPanelProps {
  screen: Screen;
  onSave: (themeOverride: Partial<ThemeTokens> | null) => Promise<void>;
  onCancel: () => void;
}

// The screen-level counterpart to CardSettingsPanel's own override
// toggle, narrowed to SCREEN_OVERRIDE_FIELDS instead of
// CARD_OVERRIDE_FIELDS (issue #87) -- no data source, config schema, or
// position fields here, since a screen isn't a widget instance.
function ScreenSettingsPanel({ screen, onSave, onCancel }: ScreenSettingsPanelProps) {
  const [overrideEnabled, setOverrideEnabled] = useState(screen.theme_override != null);
  const [themeOverride, setThemeOverride] = useState<Partial<ThemeTokens>>(screen.theme_override ?? {});
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await onSave(overrideEnabled ? themeOverride : null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <form className="manifest-form" onSubmit={handleSubmit}>
      <h2>Screen font</h2>
      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}

      <p className="field-help">Give this screen its own font without changing the display's whole theme.</p>

      <label className="toggle-row">
        <span>Override font for this screen</span>
        <input type="checkbox" checked={overrideEnabled} onChange={(e) => setOverrideEnabled(e.target.checked)} />
      </label>
      {overrideEnabled && (
        <ThemeTokenFields
          values={themeOverride}
          onChange={(v) => setThemeOverride((prev) => ({ ...prev, ...v }))}
          fields={SCREEN_OVERRIDE_FIELDS}
        />
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
