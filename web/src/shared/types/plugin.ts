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
