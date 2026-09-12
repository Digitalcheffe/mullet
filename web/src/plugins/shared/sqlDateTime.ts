// Shared by any widget reading a shape with a SQL-formatted timestamp
// column (e.g. the "events" shape's start/end) -- "YYYY-MM-DD HH:MM:SS"
// (UTC, per internal/db/data_api.go's normalizeSQLValue) isn't directly
// Date-parseable without a "T" and zone, so this makes it one.
export function parseSQLDateTime(raw: string): Date {
  return new Date(raw.replace(' ', 'T') + 'Z');
}
