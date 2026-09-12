import type { UIPlugin, WidgetProps } from '../../shared/types/plugin';
import { cardStyle } from '../shared/cardStyle';
import { daysBetween, parseLocalDate } from '../shared/dateMath';
import './CountdownWidget.css';

interface Config {
  targetDate?: string;
  label?: string;
}

function CountdownComponent({ config, theme }: WidgetProps<unknown>) {
  const cfg = config as Config;
  const label = cfg.label?.trim() || 'Countdown';
  const style = cardStyle(theme);

  if (!cfg.targetDate) {
    return (
      <div className="countdown-widget mullet-card" style={style}>
        <div className="countdown-empty">Set a target date in this card's settings</div>
      </div>
    );
  }

  const days = daysBetween(new Date(), parseLocalDate(cfg.targetDate));

  return (
    <div className="countdown-widget mullet-card" style={style}>
      {days === 0 ? (
        <div className="countdown-today" style={{ color: theme.accentColor }}>
          Today!
        </div>
      ) : (
        <>
          <div className="countdown-number" style={{ color: theme.accentColor }}>
            {Math.abs(days)}
          </div>
          <div className="countdown-caption">{days > 0 ? 'days until' : 'days since'}</div>
        </>
      )}
      <div className="countdown-label">{label}</div>
    </div>
  );
}

export const countdownPlugin: UIPlugin<unknown> = {
  id: 'mullet-countdown',
  name: 'Countdown',
  dataShape: '',
  defaultSize: { w: 2, h: 2 },
  minSize: { w: 2, h: 2 },
  maxSize: { w: 4, h: 4 },
  configSchema: {
    targetDate: {
      type: 'date',
      label: 'Target date',
    },
    label: {
      type: 'text',
      label: 'Label',
      default: 'Countdown',
      helpText: 'e.g. "Summer Trip" or "Grandma\'s Birthday".',
    },
  },
  component: CountdownComponent,
};
