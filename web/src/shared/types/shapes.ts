// TypeScript mirrors of the framework's Go data shapes. Kept in sync with
// internal/shapes as contracts are implemented (see issue #4).

export interface CalendarEvent {
  id: string;
  calendarId: number;
  title: string;
  start: string;
  end: string | null;
  allDay: boolean;
  location: string | null;
  description: string | null;
}

export interface Task {
  id: string;
  taskListId: number;
  title: string;
  completed: boolean;
  dueDate: string | null;
  sortOrder: number;
}

export interface WeatherCurrent {
  id: string;
  temp: number;
  feelsLike: number | null;
  condition: string;
  icon: string;
  humidity: number | null;
  high: number | null;
  low: number | null;
  sunrise: string | null;
  sunset: string | null;
}

export interface WeatherForecast {
  id: string;
  date: string;
  high: number;
  low: number;
  condition: string;
  icon: string;
  precipChance: number | null;
}
