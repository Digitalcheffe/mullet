package data

import (
	"fmt"
	"sort"
	"sync"
)

// Registry holds the compiled-in data plugins, keyed by ID. Plugins
// register themselves at init time; the scheduler and admin API consume
// the registry to run and expose them.
type Registry struct {
	mu      sync.RWMutex
	plugins map[string]DataPlugin
}

// NewRegistry returns an empty plugin registry.
func NewRegistry() *Registry {
	return &Registry{plugins: make(map[string]DataPlugin)}
}

// Register adds a plugin to the registry. It returns an error if a plugin
// with the same ID is already registered.
func (r *Registry) Register(p DataPlugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	id := p.ID()
	if _, exists := r.plugins[id]; exists {
		return fmt.Errorf("plugin %q already registered", id)
	}
	r.plugins[id] = p
	return nil
}

// List returns all registered plugins, sorted by ID for deterministic
// output.
func (r *Registry) List() []DataPlugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]string, 0, len(r.plugins))
	for id := range r.plugins {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	plugins := make([]DataPlugin, len(ids))
	for i, id := range ids {
		plugins[i] = r.plugins[id]
	}
	return plugins
}

// Get returns the plugin registered under id, if any.
func (r *Registry) Get(id string) (DataPlugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.plugins[id]
	return p, ok
}

// Default is the registry compiled-in plugins register themselves into
// via init(), mirroring the database/sql driver pattern: a plugin
// package's init() calls data.Register(New()), and main blank-imports
// the package for that side effect. The server then uses Default rather
// than constructing its own registry.
var Default = NewRegistry()

// Register adds p to Default.
func Register(p DataPlugin) error { return Default.Register(p) }

// List returns every plugin registered in Default.
func List() []DataPlugin { return Default.List() }

// Get returns the plugin registered under id in Default, if any.
func Get(id string) (DataPlugin, bool) { return Default.Get(id) }
