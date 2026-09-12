import type { UIPlugin, WidgetProps } from '../../shared/types/plugin';
import { cardStyle } from '../shared/cardStyle';
import { QUOTES } from './quotes';
import './QuoteOfTheDayWidget.css';

// Deterministic by calendar day (the viewer's local "today", matching the
// day-boundary convention used elsewhere -- see BirthdaysWidget) so the
// same quote holds all day rather than changing on every refresh poll.
function quoteForDay(date: Date): (typeof QUOTES)[number] {
  const key = `${date.getFullYear()}-${date.getMonth()}-${date.getDate()}`;
  let hash = 0;
  for (let i = 0; i < key.length; i++) {
    hash = (hash * 31 + key.charCodeAt(i)) >>> 0;
  }
  return QUOTES[hash % QUOTES.length];
}

function QuoteOfTheDayComponent({ theme }: WidgetProps<unknown>) {
  const quote = quoteForDay(new Date());
  const style = cardStyle(theme);

  return (
    <div className="quote-widget mullet-card" style={style}>
      <p className="quote-text">&ldquo;{quote.text}&rdquo;</p>
      <p className="quote-author" style={{ color: theme.accentColor }}>
        &mdash; {quote.author}
      </p>
    </div>
  );
}

export const quoteOfTheDayPlugin: UIPlugin<unknown> = {
  id: 'mullet-quote-of-the-day',
  name: 'Quote of the Day',
  dataShape: '',
  defaultSize: { w: 4, h: 2 },
  minSize: { w: 2, h: 2 },
  maxSize: { w: 8, h: 4 },
  component: QuoteOfTheDayComponent,
};
