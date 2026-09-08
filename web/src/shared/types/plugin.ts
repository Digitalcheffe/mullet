// UI plugin contract. A UI plugin declares what data shape it consumes
// and how much grid space it needs (see architecture.md "UI Plugins").

import type { ComponentType } from 'react';
import type { ThemeTokens } from '../themes/tokens';

export interface GridSize {
  w: number;
  h: number;
}

export interface ConfigField {
  type: 'text' | 'number' | 'select' | 'multi-select' | 'toggle' | 'color';
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
  // The card's own data_plugin_instance_id (null if it has none set) --
  // most widgets don't need this, since `data` already carries their one
  // shape's rows. It exists for a widget that needs a *second*, related
  // shape from the same instance (e.g. calendar-agenda looking up a
  // calendar's real name/color via the "calendars" shape, alongside its
  // primary "events" data) -- fetch it the same way the display itself
  // does, with the display's own useShapeData hook.
  pluginInstanceId: number | null;
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
}
