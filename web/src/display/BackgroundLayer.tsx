import { useEffect, useState } from 'react';
import { parseBackgroundImages, type ThemeTokens } from '../shared/themes/tokens';
import './BackgroundLayer.css';

interface Props {
  background: ThemeTokens['background'];
}

// Crossfades between slides on a fixed timer -- rotation speed isn't
// configurable (issue #92 doesn't ask for it), just long enough to
// read as ambient rather than distracting.
const SLIDE_MS = 20 * 1000;

// Renders an image-type background with the ambient motion issue #92
// asks for: a slow Ken Burns zoom/pan (CSS-only, see BackgroundLayer.css),
// a crossfade between images when there's more than one, and a
// legibility scrim so text/cards on top stay readable regardless of
// what's in the photo. Solid/gradient backgrounds don't render this at
// all -- DisplayApp keeps setting those directly via backgroundCSS, same
// as before this existed.
export default function BackgroundLayer({ background }: Props) {
  const images = background.type === 'image' ? parseBackgroundImages(background.value) : [];
  const imagesKey = images.join('\n');
  const [index, setIndex] = useState(0);

  // Resets to the first slide whenever the configured image list itself
  // changes (not just on every re-render) -- keyed off the list's own
  // content rather than length, so replacing image 1 of 1 with a
  // different single image also restarts at slide 0 instead of leaving
  // `index` pointed at a URL that's no longer in the list.
  useEffect(() => {
    setIndex(0);
  }, [imagesKey]);

  useEffect(() => {
    if (images.length <= 1) return;
    const interval = setInterval(() => setIndex((i) => (i + 1) % images.length), SLIDE_MS);
    return () => clearInterval(interval);
  }, [images.length]);

  if (images.length === 0) return null;

  return (
    <div className="background-layer">
      {images.map((url, i) => (
        <div
          key={url + i}
          className={`background-layer-slide${i === index ? ' active' : ''}`}
          style={{ backgroundImage: `url(${url})` }}
        />
      ))}
      <div className="background-layer-scrim" />
    </div>
  );
}
