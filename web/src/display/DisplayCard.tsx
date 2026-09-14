import type { CSSProperties } from 'react';
import type { ThemeTokens } from '../shared/themes/tokens';
import { getUIPlugin } from '../plugins/registry';
import type { CardLayout } from './useDisplayLayout';
import { useShapeData } from './useShapeData';

interface Props {
  card: CardLayout;
  theme: ThemeTokens;
  // Seconds of animation-delay for this card's entrance (issue #86) --
  // 0 if omitted, so existing callers (none currently) aren't required
  // to pass it.
  enterDelay?: number;
}

const HALIGN_TO_JUSTIFY: Record<'left' | 'center' | 'right', CSSProperties['justifyContent']> = {
  left: 'flex-start',
  center: 'center',
  right: 'flex-end',
};
const VALIGN_TO_ALIGN: Record<'top' | 'center' | 'bottom', CSSProperties['alignItems']> = {
  top: 'flex-start',
  center: 'center',
  bottom: 'flex-end',
};

// DisplayCard resolves a card's ui_plugin_id against the compiled-in UI
// plugin registry and renders it with live data. Falls back to a plain
// placeholder for an unknown ID rather than throwing -- a card can
// outlive the plugin that created it (e.g. after a rename or a plugin
// removed from a build), and one bad card shouldn't take the whole
// display down.
export default function DisplayCard({ card, theme, enterDelay }: Props) {
  const plugin = getUIPlugin(card.ui_plugin_id);
  const resolvedTheme: ThemeTokens = { ...theme, ...(card.theme_override ?? {}) };
  const gridStyle: CSSProperties = {
    gridColumn: `${card.x + 1} / span ${card.w}`,
    gridRow: `${card.y + 1} / span ${card.h}`,
    ...(enterDelay ? { '--card-enter-delay': `${enterDelay}s` } : {}),
  } as CSSProperties;

  if (!plugin) {
    return (
      <div className="display-card display-card-unknown" style={gridStyle}>
        Unknown widget "{card.ui_plugin_id}"
      </div>
    );
  }

  // Content alignment (issue #149) only kicks in once either axis is
  // set to something other than the default 'center' -- 'center'/
  // 'center' (or both absent) renders identically to before this
  // existed, matching every card that never opts in.
  const hasContentAlign = (card.content_halign && card.content_halign !== 'center') || (card.content_valign && card.content_valign !== 'center');
  const bodyStyle: CSSProperties | undefined = hasContentAlign
    ? {
        display: 'flex',
        justifyContent: HALIGN_TO_JUSTIFY[card.content_halign ?? 'center'],
        alignItems: VALIGN_TO_ALIGN[card.content_valign ?? 'center'],
      }
    : undefined;

  return (
    <div className="display-card" style={gridStyle}>
      {card.header_text && <CardHeader card={card} theme={resolvedTheme} />}
      <div className={`display-card-body${hasContentAlign ? ' display-card-body-aligned' : ''}`} style={bodyStyle}>
        <PluginBody plugin={plugin} card={card} theme={resolvedTheme} />
      </div>
    </div>
  );
}

// CardHeader labels a card (issue #145) -- e.g. distinguishing two
// Calendar Agenda cards for different rooms. Always its own reserved row
// at the top of the card (issue #154, dropping the earlier top/middle/
// bottom option -- vertical placement isn't configurable, only
// header_halign is), plain text with no background or shared card
// chrome. The widget's body simply gets whatever space remains.
function CardHeader({ card, theme }: { card: CardLayout; theme: ThemeTokens }) {
  const halign = card.header_halign ?? 'left';
  const style: CSSProperties = {
    display: 'flex',
    justifyContent: halign === 'left' ? 'flex-start' : halign === 'right' ? 'flex-end' : 'center',
    padding: '0.35em 0.6em',
    color: theme.textColor,
    fontFamily: theme.headingFontFamily,
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
