package data

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakePlugin struct {
	id string
}

func (f fakePlugin) ID() string   { return f.id }
func (f fakePlugin) Name() string { return f.id }
func (f fakePlugin) Manifest() DataPluginManifest {
	return DataPluginManifest{ID: f.id}
}
func (f fakePlugin) DataShapes() []string           { return []string{"events"} }
func (f fakePlugin) RefreshInterval() time.Duration { return time.Minute }
func (f fakePlugin) Configure(cfg map[string]any) error {
	return nil
}
func (f fakePlugin) Fetch(ctx context.Context) (map[string][]any, error) {
	return nil, nil
}

func TestRegistry(t *testing.T) {
	r := NewRegistry()

	if err := r.Register(fakePlugin{id: "clock"}); err != nil {
		t.Fatalf("Register(clock): %v", err)
	}
	if err := r.Register(fakePlugin{id: "openweathermap"}); err != nil {
		t.Fatalf("Register(openweathermap): %v", err)
	}

	if err := r.Register(fakePlugin{id: "clock"}); err == nil {
		t.Error("Register(clock) again: expected error for duplicate ID, got nil")
	}

	got := r.List()
	if len(got) != 2 {
		t.Fatalf("List() returned %d plugins, want 2", len(got))
	}
	if got[0].ID() != "clock" || got[1].ID() != "openweathermap" {
		t.Errorf("List() = [%s, %s], want sorted [clock, openweathermap]", got[0].ID(), got[1].ID())
	}

	p, ok := r.Get("clock")
	if !ok || p.ID() != "clock" {
		t.Errorf("Get(clock) = %v, %v", p, ok)
	}

	if _, ok := r.Get("missing"); ok {
		t.Error("Get(missing) returned ok=true, want false")
	}
}

func TestWithPluginUnknownID(t *testing.T) {
	r := NewRegistry()

	called := false
	ok, err := r.WithPlugin("missing", func(DataPlugin) error {
		called = true
		return nil
	})
	if ok || err != nil {
		t.Errorf("WithPlugin(missing) = (%v, %v), want (false, nil)", ok, err)
	}
	if called {
		t.Error("fn was called for an unregistered plugin ID")
	}
}

func TestWithPluginPropagatesError(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(fakePlugin{id: "clock"}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	wantErr := fmt.Errorf("boom")
	ok, err := r.WithPlugin("clock", func(DataPlugin) error { return wantErr })
	if !ok || err != wantErr {
		t.Errorf("WithPlugin = (%v, %v), want (true, %v)", ok, err, wantErr)
	}
}

// concurrencyCheckPlugin records whether it was ever entered while
// already inside a call, to verify WithPlugin actually serializes access
// rather than just being a lookup.
type concurrencyCheckPlugin struct {
	fakePlugin
	inside    atomic.Bool
	violation atomic.Bool
}

func (p *concurrencyCheckPlugin) Configure(cfg map[string]any) error {
	if !p.inside.CompareAndSwap(false, true) {
		p.violation.Store(true)
	}
	defer p.inside.Store(false)
	time.Sleep(2 * time.Millisecond) // widen the window a real race would need
	return nil
}

func TestWithPluginSerializesConcurrentCallers(t *testing.T) {
	r := NewRegistry()
	plugin := &concurrencyCheckPlugin{fakePlugin: fakePlugin{id: "clock"}}
	if err := r.Register(plugin); err != nil {
		t.Fatalf("Register: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.WithPlugin("clock", func(p DataPlugin) error {
				return p.Configure(map[string]any{})
			})
		}()
	}
	wg.Wait()

	if plugin.violation.Load() {
		t.Error("WithPlugin allowed two callers inside Configure() at once")
	}
}
