package data

import (
	"fmt"
	"sort"
	"sync"
)

// Registry holds the compiled-in data plugins, keyed by ID. Plugins
// register themselves at init time; the scheduler and admin API consume
// the registry to run and expose them.
//
// Every configured instance of a given plugin type shares the one
// registered object (there is no per-instance Clone -- see the note
// where the scheduler configures a plugin). Get() is safe for
// unsynchronized reads of read-only methods (ID, Name, Manifest,
// DataShapes). Configure()+Fetch() mutate that shared object, so any
// caller doing that -- the scheduler's own tick, or an admin's manual
// "test connection" -- must go through WithPlugin instead, which
// serializes access per plugin ID.
type Registry struct {
	mu      sync.RWMutex
	plugins map[string]DataPlugin
	locks   map[string]*sync.Mutex
}

// NewRegistry returns an empty plugin registry.
func NewRegistry() *Registry {
	return &Registry{
		plugins: make(map[string]DataPlugin),
		locks:   make(map[string]*sync.Mutex),
	}
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
	r.locks[id] = &sync.Mutex{}
	return nil
}

// WithPlugin looks up the plugin registered under id and calls fn with
// it while holding that plugin's lock, so no other WithPlugin(id, ...)
// call -- from any goroutine -- can run concurrently. Returns false if no
// plugin is registered under id, in which case fn is not called.
func (r *Registry) WithPlugin(id string, fn func(DataPlugin) error) (bool, error) {
	r.mu.RLock()
	plugin, ok := r.plugins[id]
	lock := r.locks[id]
	r.mu.RUnlock()
	if !ok {
		return false, nil
	}

	lock.Lock()
	defer lock.Unlock()
	return true, fn(plugin)
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

// WithPlugin calls fn with the plugin registered under id in Default,
// serialized per plugin ID. See Registry.WithPlugin.
func WithPlugin(id string, fn func(DataPlugin) error) (bool, error) {
	return Default.WithPlugin(id, fn)
}
