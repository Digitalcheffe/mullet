package data

import (
	"context"
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
