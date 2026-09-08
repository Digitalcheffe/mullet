import './ConnectOverlay.css';

interface Props {
  mode: 'connecting' | 'reconnecting' | 'not-found';
  slug: string;
}

// ConnectOverlay covers two distinct states with one component: a
// blocking full-screen message before any layout has ever loaded
// (`connecting`, or `not-found` for a slug with no matching display),
// and a translucent banner over the last-known layout once a
// previously working display goes unreachable (`reconnecting`) --
// DisplayApp picks the mode from whether it already has a layout to
// show underneath.
export default function ConnectOverlay({ mode, slug }: Props) {
  if (mode === 'reconnecting') {
    return (
      <div className="connect-overlay connect-overlay-banner">
        <span className="connect-spinner" />
        Reconnecting…
      </div>
    );
  }

  return (
    <div className="connect-overlay connect-overlay-full">
      <span className="connect-spinner" />
      <p>{mode === 'not-found' ? `No display found for "${slug}"` : 'Connecting…'}</p>
    </div>
  );
}
