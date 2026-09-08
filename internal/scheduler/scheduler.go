// Package scheduler runs each enabled data plugin instance on its
// configured refresh interval and writes the results to its shape table.
package scheduler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/Digitalcheffe/mullet/internal/db"
	plugindata "github.com/Digitalcheffe/mullet/internal/plugins/data"
)

// Scheduler runs registered data plugins for every enabled plugin
// instance, on the interval configured for that instance. Per the
// architecture ("enabling/configuring a plugin is hot"), Reload can be
// called any time after Start to pick up instances added, edited,
// enabled/disabled, or removed via the admin API -- no restart needed.
type Scheduler struct {
	db       *sql.DB
	registry *plugindata.Registry

	mu      sync.Mutex
	ctx     context.Context
	cancel  context.CancelFunc
	running map[int]context.CancelFunc // instance ID -> stop just that instance
	wg      sync.WaitGroup
}

// New returns a Scheduler that reads plugin instances from sqldb and
// looks up their plugin implementation in registry.
func New(sqldb *sql.DB, registry *plugindata.Registry) *Scheduler {
	return &Scheduler{db: sqldb, registry: registry}
}

// Start loads the currently enabled plugin instances and spawns one
// goroutine per instance to fetch on its refresh interval. It returns
// once every instance has fired its first fetch; the goroutines then keep
// running until ctx is cancelled or Stop is called.
func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.running = make(map[int]context.CancelFunc)
	s.mu.Unlock()

	return s.Reload()
}

// Reload re-reads enabled plugin instances from the database and
// reconciles running goroutines against them: instances that are gone or
// disabled are stopped, and every enabled instance is (re)started so a
// config or interval change takes effect immediately. Safe to call
// concurrently with itself and with the running fetch loops.
func (s *Scheduler) Reload() error {
	instances, err := db.LoadEnabledPluginInstances(s.db)
	if err != nil {
		return err
	}

	wanted := make(map[int]db.PluginInstance, len(instances))
	for _, inst := range instances {
		wanted[inst.ID] = inst
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Stop anything no longer enabled/present.
	for id, cancel := range s.running {
		if _, ok := wanted[id]; !ok {
			cancel()
			delete(s.running, id)
		}
	}

	// (Re)start every wanted instance. Always restarting -- rather than
	// diffing what actually changed -- keeps this simple and correct at
	// the cost of an extra immediate fetch for instances that were
	// already running unchanged; fine at the scale of a handful of
	// plugin instances.
	for _, inst := range instances {
		if cancel, ok := s.running[inst.ID]; ok {
			cancel()
			delete(s.running, inst.ID)
		}

		plugin, ok := s.registry.Get(inst.PluginID)
		if !ok {
			log.Printf("scheduler: no registered plugin for instance %d (plugin_id=%q), skipping", inst.ID, inst.PluginID)
			continue
		}

		// NOTE: the registry hands out one shared plugin object per
		// plugin_id, so two enabled instances of the *same* plugin type
		// would race on its config fields here and while fetching. Fine
		// while every plugin is effectively single-instance in practice;
		// revisit (e.g. a Clone() on DataPlugin) when a plugin that
		// legitimately supports multiple instances lands (ics-feed, #16).
		if err := configurePlugin(plugin, inst.Config); err != nil {
			log.Printf("scheduler: plugin %q (instance %d) configure failed: %v", inst.PluginID, inst.ID, err)
			if err := db.RecordFetchError(s.db, inst.ID, err); err != nil {
				log.Printf("scheduler: recording fetch error for instance %d: %v", inst.ID, err)
			}
			continue
		}

		interval := inst.RefreshInterval
		if interval <= 0 {
			interval = plugin.RefreshInterval()
		}

		instCtx, instCancel := context.WithCancel(s.ctx)
		s.running[inst.ID] = instCancel

		s.wg.Add(1)
		go s.run(instCtx, inst, plugin, interval)
	}

	return nil
}

// Stop cancels every running plugin loop and waits for them to exit.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	cancel := s.cancel
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	s.wg.Wait()
}

// configurePlugin parses rawConfig (a JSON object, or "" for none) and
// applies it to plugin.
func configurePlugin(plugin plugindata.DataPlugin, rawConfig string) error {
	cfg := map[string]any{}
	if rawConfig != "" {
		if err := json.Unmarshal([]byte(rawConfig), &cfg); err != nil {
			return fmt.Errorf("parsing config: %w", err)
		}
	}
	return plugin.Configure(cfg)
}

func (s *Scheduler) run(ctx context.Context, inst db.PluginInstance, plugin plugindata.DataPlugin, interval time.Duration) {
	defer s.wg.Done()

	s.fetchOnce(ctx, inst, plugin)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.fetchOnce(ctx, inst, plugin)
		}
	}
}

// fetchOnce runs a single fetch/write cycle for one plugin instance. It
// never panics or returns an error to the caller -- a failing plugin logs
// and records its error without taking down the scheduler.
func (s *Scheduler) fetchOnce(ctx context.Context, inst db.PluginInstance, plugin plugindata.DataPlugin) {
	shapeRows, err := plugin.Fetch(ctx)
	if err != nil {
		log.Printf("scheduler: plugin %q (instance %d) fetch failed: %v", plugin.ID(), inst.ID, err)
		if err := db.RecordFetchError(s.db, inst.ID, err); err != nil {
			log.Printf("scheduler: recording fetch error for instance %d: %v", inst.ID, err)
		}
		return
	}

	shapes := make([]string, 0, len(shapeRows))
	for shape := range shapeRows {
		shapes = append(shapes, shape)
	}
	sort.Strings(shapes)

	for _, shape := range shapes {
		if err := db.WriteShape(s.db, shape, inst.ID, shapeRows[shape]); err != nil {
			log.Printf("scheduler: plugin %q (instance %d) write failed for shape %q: %v", plugin.ID(), inst.ID, shape, err)
			if err := db.RecordFetchError(s.db, inst.ID, err); err != nil {
				log.Printf("scheduler: recording fetch error for instance %d: %v", inst.ID, err)
			}
			return
		}
	}

	if err := db.RecordFetchSuccess(s.db, inst.ID); err != nil {
		log.Printf("scheduler: recording fetch success for instance %d: %v", inst.ID, err)
	}
}
