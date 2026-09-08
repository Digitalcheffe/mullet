// Package clock implements the simplest possible data plugin: no external
// API, no auth, no data shape. It exists to prove the plugin lifecycle
// (registration, scheduling, fetching) end-to-end with zero external
// dependencies.
package clock

import (
	"context"
	"time"

	plugindata "github.com/Digitalcheffe/mullet/internal/plugins/data"
)

// ID is this plugin's registry key.
const ID = "clock"

func init() {
	if err := plugindata.Register(New()); err != nil {
		panic(err)
	}
}

// Plugin reports the current time. It writes to no shape -- the clock UI
// plugin reads the browser's local time directly -- so Fetch exists only
// to satisfy and exercise the DataPlugin interface.
type Plugin struct{}

// New returns a clock plugin instance.
func New() *Plugin { return &Plugin{} }

func (p *Plugin) ID() string   { return ID }
func (p *Plugin) Name() string { return "Clock" }

func (p *Plugin) Manifest() plugindata.DataPluginManifest {
	return plugindata.DataPluginManifest{
		ID:                  ID,
		Name:                "Clock",
		Description:         "System clock. No external data fetch, no configuration.",
		DataShape:           "",
		AuthType:            "none",
		RecommendedInterval: time.Minute,
		MinInterval:         time.Second,
	}
}

// DataShape is empty: clock has no framework or custom contract.
func (p *Plugin) DataShape() string { return "" }

func (p *Plugin) RefreshInterval() time.Duration { return time.Minute }

// Configure is a no-op: the manifest declares zero setup fields.
func (p *Plugin) Configure(cfg map[string]any) error { return nil }

// Fetch returns the current time. The scheduler skips the write path for
// plugins with an empty DataShape, so this value isn't persisted -- it
// demonstrates the interface working, which is this plugin's whole job.
func (p *Plugin) Fetch(ctx context.Context) ([]any, error) {
	return []any{time.Now()}, nil
}
