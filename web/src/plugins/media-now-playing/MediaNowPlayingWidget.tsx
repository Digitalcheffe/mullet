import type { UIPlugin, WidgetProps } from '../../shared/types/plugin';
import { cardStyle } from '../shared/cardStyle';
import './MediaNowPlayingWidget.css';

// One row from GET /api/data/media_status -- field names match the
// shape_media_status columns verbatim (see internal/shapes/media.go).
export interface MediaStatusRow {
  id: string;
  plugin_instance_id: number;
  player_name: string;
  is_playing: number; // SQLite INTEGER 0/1, not a JSON boolean
  title: string | null;
  artist: string | null;
  album_art_url: string | null;
  fetched_at: string;
}

interface Config {
  showAlbumArt?: boolean;
}

// Multiple configured players can all report rows at once (e.g. two
// Plex/Sonos zones) -- prefer whichever one is actually playing over
// just taking data[0], since an idle player elsewhere shouldn't hide
// the one the household actually cares about right now.
function pickPlayer(data: MediaStatusRow[]): MediaStatusRow | undefined {
  return data.find((d) => d.is_playing) ?? data[0];
}

function MediaNowPlayingComponent({ data, config, size, theme }: WidgetProps<MediaStatusRow>) {
  const cfg = config as Config;
  const showAlbumArt = cfg.showAlbumArt !== false;
  const compact = size.h <= 2 || size.w <= 3;

  const style = cardStyle(theme);
  const player = pickPlayer(data);

  if (!player || (!player.is_playing && !player.title)) {
    return (
      <div className="media-now-playing-widget mnp-empty" style={style}>
        Nothing playing
      </div>
    );
  }

  return (
    <div className={`media-now-playing-widget${compact ? ' mnp-compact' : ''}`} style={style}>
      {showAlbumArt && !compact && (
        <div className="mnp-art">
          {player.album_art_url ? <img src={player.album_art_url} alt="" /> : <span className="mnp-art-placeholder">🎵</span>}
        </div>
      )}
      <div className="mnp-info">
        <div className="mnp-title">{player.title ?? 'Unknown track'}</div>
        {player.artist && <div className="mnp-artist">{player.artist}</div>}
        <div className="mnp-meta" style={{ color: theme.accentColor }}>
          <span className="mnp-play-icon">{player.is_playing ? '▶' : '⏸'}</span>
          <span className="mnp-player-name">{player.player_name}</span>
        </div>
      </div>
    </div>
  );
}

export const mediaNowPlayingPlugin: UIPlugin<MediaStatusRow> = {
  id: 'mullet-media-now-playing',
  name: 'Media (Now Playing)',
  dataShape: 'media_status',
  defaultSize: { w: 4, h: 3 },
  minSize: { w: 2, h: 2 },
  maxSize: { w: 6, h: 6 },
  configSchema: {
    showAlbumArt: { type: 'toggle', label: 'Show album art', default: true },
  },
  component: MediaNowPlayingComponent,
};
