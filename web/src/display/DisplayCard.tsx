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

  return (
    <div className="display-card" style={gridStyle}>
      <PluginBody plugin={plugin} card={card} theme={resolvedTheme} />
      {card.header_text && <CardHeaderOverlay card={card} theme={resolvedTheme} />}
    </div>
  );
}

// CardHeaderOverlay labels a card (issue #145) -- e.g. distinguishing two
// Calendar Agenda cards for different rooms. Absolutely positioned over
// PluginBody rather than laid out alongside it, so a widget never has to
// account for a header eating into its own size.
function CardHeaderOverlay({ card, theme }: { card: CardLayout; theme: ThemeTokens }) {
  const valign = card.header_valign ?? 'top';
  const halign = card.header_halign ?? 'left';
  const style: CSSProperties = {
    position: 'absolute',
    left: 0,
    right: 0,
    ...(valign === 'top' && { top: 0 }),
    ...(valign === 'bottom' && { bottom: 0 }),
    ...(valign === 'middle' && { top: '50%', transform: 'translateY(-50%)' }),
    display: 'flex',
    justifyContent: halign === 'left' ? 'flex-start' : halign === 'right' ? 'flex-end' : 'center',
    padding: '0.4em 0.6em',
    color: theme.textColor,
    fontFamily: theme.headingFontFamily,
    pointerEvents: 'none',
  };
  return (
    <div className="display-card-header" style={style}>
      {card.header_text}
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
