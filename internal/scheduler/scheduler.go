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

		if _, ok := s.registry.Get(inst.PluginID); !ok {
			log.Printf("scheduler: no registered plugin for instance %d (plugin_id=%q), skipping", inst.ID, inst.PluginID)
			continue
		}

		if err := s.configureInstance(inst); err != nil {
			log.Printf("scheduler: plugin %q (instance %d) configure failed: %v", inst.PluginID, inst.ID, err)
			if err := db.RecordFetchError(s.db, inst.ID, err); err != nil {
				log.Printf("scheduler: recording fetch error for instance %d: %v", inst.ID, err)
			}
			continue
		}

		interval := inst.RefreshInterval
		if interval <= 0 {
			if plugin, ok := s.registry.Get(inst.PluginID); ok {
				interval = plugin.RefreshInterval()
			}
		}

		instCtx, instCancel := context.WithCancel(s.ctx)
		s.running[inst.ID] = instCancel

		s.wg.Add(1)
		go s.run(instCtx, inst, interval)
	}

	return nil
}

// TestInstance runs one configure+fetch+write cycle for instanceID
// immediately and synchronously, returning the outcome. Backs the admin
// API's "test connection" action. Goes through the same
// Registry.WithPlugin serialization as the scheduler's own ticks, so a
// manual test can never interleave Configure()/Fetch() with a scheduled
// run (or another test) on the plugin type's one shared object.
func (s *Scheduler) TestInstance(ctx context.Context, instanceID int) error {
	status, err := db.GetPluginInstance(s.db, instanceID)
	if err != nil {
		return err
	}

	inst := db.PluginInstance{
		ID:              status.ID,
		PluginID:        status.PluginID,
		Config:          status.Config,
		RefreshInterval: status.RefreshInterval,
	}

	if err := s.configureInstance(inst); err != nil {
		if rerr := db.RecordFetchError(s.db, inst.ID, err); rerr != nil {
			log.Printf("scheduler: recording fetch error for instance %d: %v", inst.ID, rerr)
		}
		return err
	}

	return s.fetchOnce(ctx, inst)
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

// configureInstance parses inst's stored config JSON and applies it to
// its plugin, serialized via Registry.WithPlugin.
func (s *Scheduler) configureInstance(inst db.PluginInstance) error {
	cfg := map[string]any{}
	if inst.Config != "" {
		if err := json.Unmarshal([]byte(inst.Config), &cfg); err != nil {
			return fmt.Errorf("parsing config: %w", err)
		}
	}

	ok, err := s.registry.WithPlugin(inst.PluginID, func(p plugindata.DataPlugin) error {
		return p.Configure(cfg)
	})
	if !ok {
		return fmt.Errorf("plugin %q not registered", inst.PluginID)
	}
	return err
}

func (s *Scheduler) run(ctx context.Context, inst db.PluginInstance, interval time.Duration) {
	defer s.wg.Done()

	s.fetchOnce(ctx, inst)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.fetchOnce(ctx, inst)
		}
	}
}

// fetchOnce runs a single fetch/write cycle for one plugin instance,
// serialized via Registry.WithPlugin. It never panics -- a failing
// plugin logs and records its error without taking down the scheduler --
// but does return that error, for TestInstance's benefit.
func (s *Scheduler) fetchOnce(ctx context.Context, inst db.PluginInstance) error {
	var shapeRows map[string][]any

	ok, fetchErr := s.registry.WithPlugin(inst.PluginID, func(p plugindata.DataPlugin) error {
		var err error
		shapeRows, err = p.Fetch(ctx)
		return err
	})
	if !ok {
		err := fmt.Errorf("plugin %q not registered", inst.PluginID)
		log.Printf("scheduler: %v (instance %d)", err, inst.ID)
		if rerr := db.RecordFetchError(s.db, inst.ID, err); rerr != nil {
			log.Printf("scheduler: recording fetch error for instance %d: %v", inst.ID, rerr)
		}
		return err
	}
	if fetchErr != nil {
		log.Printf("scheduler: plugin %q (instance %d) fetch failed: %v", inst.PluginID, inst.ID, fetchErr)
		if rerr := db.RecordFetchError(s.db, inst.ID, fetchErr); rerr != nil {
			log.Printf("scheduler: recording fetch error for instance %d: %v", inst.ID, rerr)
		}
		return fetchErr
	}

	shapes := make([]string, 0, len(shapeRows))
	for shape := range shapeRows {
		shapes = append(shapes, shape)
	}
	sort.Strings(shapes)

	for _, shape := range shapes {
		if err := db.WriteShape(s.db, shape, inst.ID, shapeRows[shape]); err != nil {
			log.Printf("scheduler: plugin %q (instance %d) write failed for shape %q: %v", inst.PluginID, inst.ID, shape, err)
			if rerr := db.RecordFetchError(s.db, inst.ID, err); rerr != nil {
				log.Printf("scheduler: recording fetch error for instance %d: %v", inst.ID, rerr)
			}
			return err
		}
	}

	if err := db.RecordFetchSuccess(s.db, inst.ID); err != nil {
		log.Printf("scheduler: recording fetch success for instance %d: %v", inst.ID, err)
	}
	return nil
}
