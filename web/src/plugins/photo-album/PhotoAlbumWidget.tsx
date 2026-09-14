import { useEffect, useState } from 'react';
import type { UIPlugin, WidgetProps } from '../../shared/types/plugin';
import { parseBackgroundImages } from '../../shared/themes/tokens';
import { cardStyle } from '../shared/cardStyle';
import './PhotoAlbumWidget.css';

interface Config {
  // One URL per line -- same convention as a theme's slideshow
  // background (issue #92), reusing parseBackgroundImages rather than
  // inventing a second list format.
  images?: string;
  intervalSeconds?: number;
  kenBurns?: boolean;
}

// PhotoAlbumWidget needs no data plugin at all (dataShape: '') -- same
// reason ClockWidget doesn't: its whole config (which images, how
// often) lives in the card's own config, not a fetched shape.
function PhotoAlbumComponent({ config, theme }: WidgetProps<unknown>) {
  const cfg = config as Config;
  const images = parseBackgroundImages(cfg.images ?? '');
  const imagesKey = images.join('\n');
  const intervalSeconds = cfg.intervalSeconds && cfg.intervalSeconds > 0 ? cfg.intervalSeconds : 10;
  const kenBurns = cfg.kenBurns !== false;
  const style = cardStyle(theme);

  const [index, setIndex] = useState(0);
  // Resets to the first slide whenever the configured image list itself
  // changes -- see BackgroundLayer's identical pattern for why this is
  // keyed off the list's content, not its length.
  useEffect(() => {
    setIndex(0);
  }, [imagesKey]);
  useEffect(() => {
    if (images.length <= 1) return;
    const interval = setInterval(() => setIndex((i) => (i + 1) % images.length), intervalSeconds * 1000);
    return () => clearInterval(interval);
  }, [images.length, intervalSeconds]);

  if (images.length === 0) {
    return (
      <div className="photo-album-widget mullet-card photo-album-empty" style={style}>
        Add one or more image URLs in this card's settings.
      </div>
    );
  }

  return (
    <div className={`photo-album-widget mullet-card${kenBurns ? ' photo-album-ken-burns' : ''}`} style={style}>
      {images.map((url, i) => (
        <div
          key={url + i}
          className={`photo-album-slide${i === index ? ' active' : ''}`}
          style={{ backgroundImage: `url(${url})` }}
        />
      ))}
    </div>
  );
}

export const photoAlbumPlugin: UIPlugin<unknown> = {
  id: 'mullet-photo-album',
  name: 'Photo Album',
  dataShape: '',
  defaultSize: { w: 6, h: 5 },
  minSize: { w: 2, h: 2 },
  maxSize: { w: 16, h: 12 },
  configSchema: {
    images: {
      type: 'textarea',
      label: 'Image URLs',
      default: '',
      helpText: 'One image URL per line. Upload images via Themes → Background → Add image(s) to get a URL, or paste any direct image link.',
    },
    intervalSeconds: { type: 'number', label: 'Seconds per photo', default: 10 },
    kenBurns: { type: 'toggle', label: 'Slow pan/zoom effect', default: true },
  },
  component: PhotoAlbumComponent,
};
