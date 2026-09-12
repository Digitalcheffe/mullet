// Package logging configures where the process's log output goes
// (issue #113). It's deliberately thin: every log.Printf/Fatal/Println
// call across the codebase (cmd/server/main.go, the scheduler,
// middleware's access log, oauth/plugin handlers, ...) already goes
// through the standard library's shared default logger, so pointing
// that logger's output at a file here is enough to make the Settings
// destination take effect everywhere at once -- no per-call-site
// migration needed, and no future call site can accidentally bypass it
// by forgetting to import a wrapper.
package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
)

var (
	mu          sync.Mutex
	currentFile *os.File
)

// Configure points the shared logger at path in addition to stdout --
// stdout always keeps receiving output so `docker logs`/the systemd
// journal keep working unchanged even when a file is also configured.
// An empty path reverts to stdout only. On failure (e.g. path isn't
// writable), the previous destination is left in place and an error is
// returned for the caller to report -- this never partially applies a
// broken destination.
func Configure(path string) error {
	mu.Lock()
	defer mu.Unlock()

	if path == "" {
		closeCurrent()
		log.SetOutput(os.Stdout)
		return nil
	}

	// The Docker image's default (/data/logs/mullet.log) puts the log in
	// its own subdirectory of the mounted volume, which won't exist on
	// a fresh deployment -- and an admin typing a path via Settings has
	// the same problem for any path whose parent doesn't already exist.
	// Creating it here (rather than failing over to stdout) is what
	// actually makes both cases work without an extra manual step.
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating log directory %q: %w", dir, err)
		}
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening log file %q: %w", path, err)
	}

	closeCurrent()
	currentFile = f
	log.SetOutput(io.MultiWriter(os.Stdout, f))
	return nil
}

// closeCurrent closes the previously opened log file, if any. Callers
// must hold mu.
func closeCurrent() {
	if currentFile != nil {
		currentFile.Close()
		currentFile = nil
	}
}
