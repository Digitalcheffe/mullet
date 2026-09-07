package api

import (
	"net/http"
	"os"
	"path/filepath"
)

// newSPAHandler serves the built frontend (web/dist) from staticDir: an
// existing file is served as-is, and everything else falls back to
// index.html so React Router can handle /admin/* and /display/* on the
// client side. If staticDir (or its index.html) doesn't exist -- e.g. a
// local `go run` without a frontend build -- requests here just 404,
// which is expected: frontend dev runs through Vite instead.
func newSPAHandler(staticDir string) http.Handler {
	fileServer := http.FileServer(http.Dir(staticDir))
	indexPath := filepath.Join(staticDir, "index.html")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested := filepath.Join(staticDir, filepath.Clean(r.URL.Path))
		if info, err := os.Stat(requested); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, indexPath)
	})
}
