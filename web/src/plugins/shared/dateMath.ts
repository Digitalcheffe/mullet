// Whole calendar days between two Dates, in the viewer's local timezone --
// used anywhere a widget shows "N days" (birthdays, countdowns) rather than
// a fractional/UTC difference that could round oddly near midnight.
export function daysBetween(from: Date, to: Date): number {
  const a = new Date(from.getFullYear(), from.getMonth(), from.getDate());
  const b = new Date(to.getFullYear(), to.getMonth(), to.getDate());
  return Math.round((b.getTime() - a.getTime()) / 86_400_000);
}

// Parses a plain "YYYY-MM-DD" (as produced by <input type="date">) as a
// local calendar date -- new Date(raw) would parse it as UTC midnight,
// which can land on the wrong day once shifted to the viewer's timezone.
export function parseLocalDate(raw: string): Date {
  const [y, m, d] = raw.split('-').map(Number);
  return new Date(y, m - 1, d);
}
