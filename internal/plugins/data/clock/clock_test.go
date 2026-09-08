package clock

import (
	"context"
	"testing"
	"time"

	plugindata "github.com/Digitalcheffe/mullet/internal/plugins/data"
)

func TestClockRegistersItselfAtInit(t *testing.T) {
	p, ok := plugindata.Get(ID)
	if !ok {
		t.Fatal("clock plugin not found in Default registry; init() should have registered it")
	}
	if p.ID() != ID {
		t.Errorf("registered plugin ID = %q, want %q", p.ID(), ID)
	}
}

func TestClockPlugin(t *testing.T) {
	p := New()

	if p.ID() != "clock" {
		t.Errorf("ID() = %q, want %q", p.ID(), "clock")
	}
	if p.DataShape() != "" {
		t.Errorf("DataShape() = %q, want empty (no contract)", p.DataShape())
	}
	if len(p.Manifest().SetupFields) != 0 {
		t.Errorf("Manifest().SetupFields has %d fields, want 0", len(p.Manifest().SetupFields))
	}
	if err := p.Configure(map[string]any{}); err != nil {
		t.Errorf("Configure() = %v, want nil", err)
	}

	rows, err := p.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("Fetch returned %d rows, want 1", len(rows))
	}
	got, ok := rows[0].(time.Time)
	if !ok {
		t.Fatalf("Fetch row is %T, want time.Time", rows[0])
	}
	if time.Since(got) > time.Second {
		t.Errorf("Fetch returned a stale time: %v", got)
	}
}
