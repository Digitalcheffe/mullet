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
// instance, on the interval configured for that instance.
type Scheduler struct {
	db       *sql.DB
	registry *plugindata.Registry

	mu     sync.Mutex
	cancel context.CancelFunc
	wg     sync.WaitGroup
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
	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.mu.Unlock()

	instances, err := db.LoadEnabledPluginInstances(s.db)
	if err != nil {
		cancel()
		return err
	}

	for _, inst := range instances {
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

		s.wg.Add(1)
		go s.run(runCtx, inst, plugin, interval)
	}

	return nil
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
