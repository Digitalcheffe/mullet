import type { CSSProperties } from 'react';
import type { ThemeTokens } from '../shared/themes/tokens';
import { getUIPlugin } from '../plugins/registry';
import type { CardLayout } from './useDisplayLayout';
import { useShapeData } from './useShapeData';

interface Props {
  card: CardLayout;
  theme: ThemeTokens;
}

// DisplayCard resolves a card's ui_plugin_id against the compiled-in UI
// plugin registry and renders it with live data. Falls back to a plain
// placeholder for an unknown ID rather than throwing -- a card can
// outlive the plugin that created it (e.g. after a rename or a plugin
// removed from a build), and one bad card shouldn't take the whole
// display down.
export default function DisplayCard({ card, theme }: Props) {
  const plugin = getUIPlugin(card.ui_plugin_id);
  const resolvedTheme: ThemeTokens = { ...theme, ...(card.theme_override ?? {}) };
  const gridStyle = {
    gridColumn: `${card.x + 1} / span ${card.w}`,
    gridRow: `${card.y + 1} / span ${card.h}`,
  };

  if (!plugin) {
    return (
      <div className="display-card display-card-unknown" style={gridStyle}>
        Unknown widget "{card.ui_plugin_id}"
      </div>
    );
  }

  const valign = card.header_valign ?? 'top';
  const showHeader = Boolean(card.header_text);

  return (
    <div className="display-card" style={gridStyle}>
      {showHeader && valign === 'top' && <CardHeaderBar card={card} theme={resolvedTheme} placement="top" />}
      <div className="display-card-body">
        <PluginBody plugin={plugin} card={card} theme={resolvedTheme} />
      </div>
      {showHeader && valign === 'bottom' && <CardHeaderBar card={card} theme={resolvedTheme} placement="bottom" />}
      {showHeader && valign === 'middle' && <CardHeaderOverlay card={card} theme={resolvedTheme} />}
    </div>
  );
}

// CardHeaderBar labels a card (issue #145) -- e.g. distinguishing two
// Calendar Agenda cards for different rooms. A real flex item taking its
// own layout space above/below the widget's body, not an overlay, so it
// never collides with the widget's own content (a top-aligned header
// used to sit directly on top of a Calendar Agenda card's first day
// row).
function CardHeaderBar({ card, theme, placement }: { card: CardLayout; theme: ThemeTokens; placement: 'top' | 'bottom' }) {
  const halign = card.header_halign ?? 'left';
  const style: CSSProperties = {
    justifyContent: halign === 'left' ? 'flex-start' : halign === 'right' ? 'flex-end' : 'center',
    color: theme.textColor,
    fontFamily: theme.headingFontFamily,
    background: theme.cardBackground,
  };
  return (
    <div className={`display-card-header-bar display-card-header-bar--${placement}`} style={style}>
      {card.header_text}
    </div>
  );
}

// CardHeaderOverlay is the middle-aligned case only -- there's no
// "reserved space" equivalent of centering a label over a card's middle
// without splitting the widget in half, so it stays an absolutely
// positioned watermark, with its own background chip so it stays legible
// over whatever the widget happens to render underneath it.
function CardHeaderOverlay({ card, theme }: { card: CardLayout; theme: ThemeTokens }) {
  const halign = card.header_halign ?? 'left';
  const style: CSSProperties = {
    justifyContent: halign === 'left' ? 'flex-start' : halign === 'right' ? 'flex-end' : 'center',
    color: theme.textColor,
    fontFamily: theme.headingFontFamily,
  };
  return (
    <div className="display-card-header-overlay" style={style}>
      <span className="display-card-header-chip">{card.header_text}</span>
    </div>
  );
}

function PluginBody({
  plugin,
  card,
  theme,
}: {
  plugin: NonNullable<ReturnType<typeof getUIPlugin>>;
  card: CardLayout;
  theme: ThemeTokens;
}) {
  const pluginInstanceId =
    card.data_plugin_instance_ids && card.data_plugin_instance_ids.length > 0
      ? card.data_plugin_instance_ids
      : card.data_plugin_instance_id;
  const data = useShapeData(plugin.dataShape, pluginInstanceId);
  const Component = plugin.component;
  return (
    <Component
      data={data}
      config={card.config}
      size={{ w: card.w, h: card.h }}
      theme={theme}
      pluginInstanceId={pluginInstanceId}
    />
  );
}
