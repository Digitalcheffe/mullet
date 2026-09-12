// UI plugin contract. A UI plugin declares what data shape it consumes
// and how much grid space it needs (see architecture.md "UI Plugins").

import type { ComponentType } from 'react';
import type { ThemeTokens } from '../themes/tokens';

export interface GridSize {
  w: number;
  h: number;
}

export interface ConfigField {
  type: 'text' | 'textarea' | 'number' | 'select' | 'multi-select' | 'toggle' | 'color' | 'date';
  label: string;
  default?: unknown;
  options?: { value: string; label: string }[];
  helpText?: string;
}

export interface WidgetProps<TData = unknown> {
  data: TData[];
  config: Record<string, unknown>;
  size: GridSize;
  theme: ThemeTokens;
  // The card's own data plugin instance id(s) (null if it has none set;
  // an array for a multi-source card, see UIPlugin.supportsMultiDataSource
  // below) -- most widgets don't need this, since `data` already carries
  // their one shape's rows. It exists for a widget that needs a
  // *second*, related shape from the same instance(s) (e.g.
  // calendar-agenda looking up a calendar's real name/color via the
  // "calendars" shape, alongside its primary "events" data) -- fetch it
  // the same way the display itself does, with the display's own
  // useShapeData hook, which accepts either form.
  pluginInstanceId: number | number[] | null;
}

export interface UIPlugin<TData = unknown> {
  id: string;
  name: string;
  dataShape: string;
  defaultSize: GridSize;
  minSize: GridSize;
  maxSize?: GridSize;
  configSchema?: Record<string, ConfigField>;
  component: ComponentType<WidgetProps<TData>>;
  // Issue #97: lets the card settings panel offer a multi-select "Data
  // sources" picker instead of the default single dropdown, and the
  // card to bind to more than one data plugin instance at once (merged
  // by the backend into one sorted `data` array). Scoped to
  // calendar-shaped widgets for now (calendar-agenda) rather than every
  // widget type, per the issue's own scoping note.
  supportsMultiDataSource?: boolean;
}
