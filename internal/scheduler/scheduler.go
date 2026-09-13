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
	"github.com/Digitalcheffe/mullet/internal/oauth"
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

		if err := s.configureInstance(s.ctx, inst); err != nil {
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
// API's "test connection" action. Routes through the same fetchOnce as
// the scheduler's own ticks, which resolves and applies this instance's
// config immediately before Fetch inside one Registry.WithPlugin lock --
// so a manual test can never run against a different instance's config,
// nor interleave with a scheduled run (or another test) on the plugin
// type's one shared object.
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

// resolveConfig parses inst's stored config JSON and, for an OAuth2-type
// plugin, injects a fresh access token under the well-known
// "access_token" config key -- EnsureFreshToken transparently refreshes
// the stored token if it's expiring soon, so a plugin's own Configure
// never has to think about token lifetime, only about reading the token
// out of cfg like any other credential. Touches only inst and the
// database -- never the shared plugin object -- so it's safe to call
// without going through Registry.WithPlugin.
func (s *Scheduler) resolveConfig(ctx context.Context, inst db.PluginInstance) (map[string]any, error) {
	cfg := map[string]any{}
	if inst.Config != "" {
		if err := json.Unmarshal([]byte(inst.Config), &cfg); err != nil {
			return nil, fmt.Errorf("parsing config: %w", err)
		}
	}

	plugin, ok := s.registry.Get(inst.PluginID)
	if !ok {
		return nil, fmt.Errorf("plugin %q not registered", inst.PluginID)
	}
	if manifest := plugin.Manifest(); manifest.AuthType == "oauth2" {
		oauthCfg, err := oauth.ConfigFromManifest(manifest, inst.Config, "")
		if err != nil {
			return nil, fmt.Errorf("building oauth config: %w", err)
		}
		token, err := oauth.EnsureFreshToken(ctx, s.db, inst.ID, oauthCfg)
		if err != nil {
			return nil, fmt.Errorf("getting oauth token: %w", err)
		}
		cfg["access_token"] = token
	}
	return cfg, nil
}

// configureInstance resolves inst's config and applies it to its plugin,
// serialized via Registry.WithPlugin. Used by Reload() as an early,
// fail-fast check (skip starting an instance's goroutine at all if its
// config is invalid) -- fetchOnce below is what actually matters for
// correctness on every subsequent tick, since it re-resolves and
// re-applies config immediately before every Fetch rather than relying
// on whatever this call last left the shared plugin object holding.
func (s *Scheduler) configureInstance(ctx context.Context, inst db.PluginInstance) error {
	cfg, err := s.resolveConfig(ctx, inst)
	if err != nil {
		return err
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

// fetchOnce runs a single configure+fetch+write cycle for one plugin
// instance. Configure and Fetch run inside one Registry.WithPlugin lock,
// immediately back to back, rather than as two separate locked calls --
// every instance of a plugin type shares one mutable plugin object (see
// registry.go), so configuring and fetching non-atomically would leave a
// window where a *different* instance's Configure() could land in
// between this instance's own Configure() and Fetch(), silently making
// this fetch run against that other instance's settings instead of its
// own. It never panics -- a failing plugin logs and records its error
// without taking down the scheduler -- but does return that error, for
// TestInstance's benefit.
func (s *Scheduler) fetchOnce(ctx context.Context, inst db.PluginInstance) error {
	cfg, err := s.resolveConfig(ctx, inst)
	if err != nil {
		log.Printf("scheduler: plugin %q (instance %d) configure failed: %v", inst.PluginID, inst.ID, err)
		if rerr := db.RecordFetchError(s.db, inst.ID, err); rerr != nil {
			log.Printf("scheduler: recording fetch error for instance %d: %v", inst.ID, rerr)
		}
		return err
	}

	var shapeRows map[string][]any

	ok, fetchErr := s.registry.WithPlugin(inst.PluginID, func(p plugindata.DataPlugin) error {
		if err := p.Configure(cfg); err != nil {
			return err
		}
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
		log.Printf("scheduler: plugin %q (instance %d) configure/fetch failed: %v", inst.PluginID, inst.ID, fetchErr)
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
