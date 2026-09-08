// UI plugin registry: mirrors the Go backend's compiled-in data plugin
// registry (internal/plugins/data/registry.go) pattern -- every UI
// plugin lives in its own folder under web/src/plugins/ and is listed
// here to make it available to whatever renders a card by ui_plugin_id
// (the grid engine, issue #23; today only the Designer's palette uses a
// separate hardcoded list, since it renders placeholders, not real
// widgets -- see architecture.md "The Designer").
import type { UIPlugin } from '../shared/types/plugin';
import { weatherCurrentPlugin } from './weather-current/WeatherCurrentWidget';
import { weatherForecastPlugin } from './weather-forecast/WeatherForecastWidget';
import { calendarAgendaPlugin } from './calendar-agenda/CalendarAgendaWidget';
import { taskListPlugin } from './task-list/TaskListWidget';
import { mealPlanPlugin } from './meal-plan/MealPlanWidget';
import { clockPlugin } from './clock/ClockWidget';
import { homeStatusPlugin } from './home-status/HomeStatusWidget';
import { serverHealthPlugin } from './server-health/ServerHealthWidget';
import { mediaNowPlayingPlugin } from './media-now-playing/MediaNowPlayingWidget';
import { packageTrackerPlugin } from './package-tracker/PackageTrackerWidget';

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export const uiPlugins: UIPlugin<any>[] = [
  weatherCurrentPlugin,
  weatherForecastPlugin,
  calendarAgendaPlugin,
  taskListPlugin,
  mealPlanPlugin,
  clockPlugin,
  homeStatusPlugin,
  serverHealthPlugin,
  mediaNowPlayingPlugin,
  packageTrackerPlugin,
];

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function getUIPlugin(id: string): UIPlugin<any> | undefined {
  return uiPlugins.find((p) => p.id === id);
}
