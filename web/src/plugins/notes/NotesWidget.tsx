import { useEffect } from 'react';
import type { UIPlugin, WidgetProps } from '../../shared/types/plugin';
import { cardStyle } from '../shared/cardStyle';
import { FONT_OPTIONS, ensureFontLoaded } from '../../shared/themes/fontOptions';
import './NotesWidget.css';

interface Config {
  text?: string;
  fontFamily?: string;
  fontSize?: 'small' | 'medium' | 'large' | 'xlarge';
  align?: 'left' | 'center' | 'right';
}

// A fixed curated scale rather than a free-text/numeric px field -- same
// "curated beats free-text" reasoning as FONT_OPTIONS itself (issue #87):
// a typo'd number degrades silently, a named size always renders as
// something reasonable.
const FONT_SIZE_PX: Record<NonNullable<Config['fontSize']>, string> = {
  small: '18px',
  medium: '28px',
  large: '40px',
  xlarge: '56px',
};

const justifyForAlign: Record<NonNullable<Config['align']>, string> = {
  left: 'flex-start',
  center: 'center',
  right: 'flex-end',
};

// NotesWidget needs no data source at all -- dataShape: '' means the
// display never even fetches for it (see useShapeData.ts), same as the
// clock widget.
function NotesComponent({ config, theme }: WidgetProps<unknown>) {
  const cfg = config as Config;
  const text = cfg.text ?? '';
  const fontFamily = cfg.fontFamily || FONT_OPTIONS[0].value;
  const align = cfg.align ?? 'center';

  // Deliberately its own font choice, not the screen/theme's -- a sticky
  // note is meant to stand out, and tying it to whatever font the rest
  // of the display happens to use would defeat that (issue #88).
  useEffect(() => {
    ensureFontLoaded(fontFamily);
  }, [fontFamily]);

  const style = cardStyle(theme);

  return (
    <div
      className="notes-widget mullet-card"
      style={{
        ...style,
        fontFamily,
        fontSize: FONT_SIZE_PX[cfg.fontSize ?? 'medium'],
        textAlign: align,
        justifyContent: justifyForAlign[align],
      }}
    >
      {text ? (
        <span className="notes-text">{text}</span>
      ) : (
        <span className="notes-placeholder">Add your note text in this card&rsquo;s settings.</span>
      )}
    </div>
  );
}

export const notesPlugin: UIPlugin<unknown> = {
  id: 'mullet-notes',
  name: 'Text / Notes',
  dataShape: '',
  defaultSize: { w: 3, h: 2 },
  minSize: { w: 2, h: 1 },
  maxSize: { w: 6, h: 6 },
  configSchema: {
    text: {
      type: 'textarea',
      label: 'Note text',
      default: '',
      helpText: 'Shown as-is, line breaks included -- e.g. "Don\'t forget your lunch money!"',
    },
    fontFamily: {
      type: 'select',
      label: 'Font',
      default: FONT_OPTIONS[0].value,
      options: FONT_OPTIONS.map((f) => ({ value: f.value, label: f.label })),
      helpText: 'Independent of the screen/theme font.',
    },
    fontSize: {
      type: 'select',
      label: 'Text size',
      default: 'medium',
      options: [
        { value: 'small', label: 'Small' },
        { value: 'medium', label: 'Medium' },
        { value: 'large', label: 'Large' },
        { value: 'xlarge', label: 'Extra large' },
      ],
    },
    align: {
      type: 'select',
      label: 'Alignment',
      default: 'center',
      options: [
        { value: 'left', label: 'Left' },
        { value: 'center', label: 'Center' },
        { value: 'right', label: 'Right' },
      ],
    },
  },
  component: NotesComponent,
};
