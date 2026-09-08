// Maps a shapes.WeatherCurrent/WeatherForecast row's `condition` string to
// a display glyph. Deliberately keyed on `condition`, not `icon`: the two
// weather data plugins don't share an icon vocabulary (openweathermap
// passes through its own icon codes like "02d"; open-meteo emits simple
// keywords like "clouds") but both normalize `condition` to the same
// small set of English category words (Clear, Clouds, Rain, ...), so
// that's the one field a UI plugin can actually trust to mean the same
// thing regardless of which data plugin produced the row.
const GLYPHS: Record<string, string> = {
  clear: '☀️',
  clouds: '☁️',
  drizzle: '🌦️',
  rain: '🌧️',
  snow: '🌨️',
  thunderstorm: '⛈️',
  fog: '🌫️',
  mist: '🌫️',
  haze: '🌫️',
  smoke: '🌫️',
};

const DEFAULT_GLYPH = '🌡️';

export function conditionGlyph(condition: string | undefined | null): string {
  if (!condition) return DEFAULT_GLYPH;
  return GLYPHS[condition.toLowerCase()] ?? DEFAULT_GLYPH;
}
