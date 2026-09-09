// Triggers a browser file download for a JSON-serializable value --
// small enough, and used in few enough places (theme export today), that
// this isn't a general download utility module, just this one shape.
export function downloadJSON(filename: string, data: unknown): void {
  const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

// slugify turns a theme name into a filesystem-safe base filename --
// "Dark Glass" -> "dark-glass". Falls back to a generic name for a
// title that's entirely punctuation/whitespace (or empty, e.g. a new
// theme that hasn't been named yet).
export function slugify(name: string): string {
  const slug = name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/(^-|-$)/g, '');
  return slug || 'theme';
}
