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
  const data = useShapeData(plugin.dataShape, card.data_plugin_instance_id);
  const Component = plugin.component;
  return (
    <Component
      data={data}
      config={card.config}
      size={{ w: card.w, h: card.h }}
      theme={theme}
      pluginInstanceId={card.data_plugin_instance_id}
    />
  );
}
