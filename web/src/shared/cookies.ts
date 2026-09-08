// Minimal document.cookie helpers -- there's no auth/security use of
// cookies anywhere in this app (sessions are a bearer JWT kept in
// memory, see AuthContext), so this is deliberately not a full cookie
// library. First user: RegisterPage remembering a browser client's own
// generated client_id across reloads.

export function getCookie(name: string): string | null {
  const match = document.cookie.match(new RegExp('(?:^|; )' + name.replace(/([.$?*|{}()[\]\\/+^])/g, '\\$1') + '=([^;]*)'));
  return match ? decodeURIComponent(match[1]) : null;
}

export function setCookie(name: string, value: string, maxAgeDays: number): void {
  document.cookie = `${name}=${encodeURIComponent(value)}; max-age=${maxAgeDays * 86400}; path=/; SameSite=Lax`;
}

export function deleteCookie(name: string): void {
  document.cookie = `${name}=; max-age=0; path=/; SameSite=Lax`;
}
