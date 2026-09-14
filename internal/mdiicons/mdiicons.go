// Package mdiicons vendors the Material Design Icons SVG set
// (@mdi/svg, pinned to the snapshot under svg/) as a build-time
// dependency, embedded directly into the server binary -- no runtime
// network access is ever needed to resolve an icon. This is
// deliberate (issue #175): the display must never make any request
// the server doesn't already fully control, so an icon is resolved
// server-side and cached to disk once, rather than fetched live (from
// mullet's own server or anywhere else) each time it's rendered.
package mdiicons

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

//go:embed svg/*.svg
var svgFS embed.FS

// nameRe validates an MDI icon name (the part after "mdi:") before it
// ever touches the filesystem -- matches @mdi/svg's own naming
// convention (lowercase letters, digits, hyphens), rejecting anything
// else outright rather than resolving it, since this name becomes part
// of both an embed.FS lookup path and a cache filename.
var nameRe = regexp.MustCompile(`^[a-z0-9-]+$`)

var (
	mu       sync.Mutex
	cacheDir string
)

// Init sets the directory a resolved icon gets copied into on first
// use -- must be called once at startup (with a directory under the
// same persistent volume as the database/uploads) before Resolve does
// anything useful. Resolve silently no-ops until this has been called.
func Init(dir string) {
	mu.Lock()
	defer mu.Unlock()
	cacheDir = dir
}

// Resolve takes a Home Assistant icon string like "mdi:water-percent"
// and returns the filename (not a full URL -- callers own how that's
// exposed) it's cached under in the configured directory, copying it
// there first if this is the first time it's been seen. Returns
// ("", false) for anything that isn't a resolvable MDI icon reference
// (wrong prefix, unknown/invalid name, Init not called yet) --
// callers should just omit the icon in that case rather than erroring,
// the same "best effort, never blocks the rest of Fetch" treatment
// fetchAreas already gets in the homeassistant plugin.
func Resolve(haIcon string) (filename string, ok bool) {
	name, found := strings.CutPrefix(haIcon, "mdi:")
	if !found || name == "" || !nameRe.MatchString(name) {
		return "", false
	}

	mu.Lock()
	dir := cacheDir
	mu.Unlock()
	if dir == "" {
		return "", false
	}

	filename = name + ".svg"
	cachedPath := filepath.Join(dir, filename)
	if _, err := os.Stat(cachedPath); err == nil {
		return filename, true
	}

	data, err := fs.ReadFile(svgFS, "svg/"+filename)
	if err != nil {
		return "", false
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", false
	}
	// A benign race is possible here if two Fetches resolve the same
	// new icon at once (each plugin instance fetches on its own timer,
	// concurrently) -- both would write the same bytes, so the result
	// is correct either way and isn't worth a lock around the write.
	if err := os.WriteFile(cachedPath, data, 0o644); err != nil {
		return "", false
	}
	return filename, true
}
