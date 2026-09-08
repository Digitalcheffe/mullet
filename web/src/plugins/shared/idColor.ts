// A small fixed palette plus a deterministic string hash, so something
// with no explicit color of its own (e.g. a calendar a plugin didn't
// set a color for) still gets a color that's stable across renders and
// visually distinct from its neighbors, without needing every producer
// to supply one.
const PALETTE = ['#4285F4', '#EA4335', '#34A853', '#FBBC04', '#A142F4', '#00ACC1', '#FF7043', '#9E9D24'];

export function colorForID(id: string | number): string {
  const s = String(id);
  let hash = 0;
  for (let i = 0; i < s.length; i++) {
    hash = (hash * 31 + s.charCodeAt(i)) | 0;
  }
  return PALETTE[Math.abs(hash) % PALETTE.length];
}
