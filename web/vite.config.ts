import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  build: {
    outDir: 'dist',
  },
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
      // Admin-uploaded files (issue #58) are served by the Go backend
      // too, outside /api/* (a display loading one has no way to
      // attach a Bearer token) -- without this, a request for one in
      // dev falls through to Vite's own SPA fallback (a 200 response,
      // but index.html, not the image), which is a silently-broken
      // background/image regardless of the actual server-side code
      // being correct. Production has no separate dev server, so this
      // only matters for `npm run dev`.
      '/uploads': 'http://localhost:8080',
    },
  },
})
