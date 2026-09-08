package clock

import (
	"context"
	"testing"

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
	if len(p.DataShapes()) != 0 {
		t.Errorf("DataShapes() = %v, want empty (no contract)", p.DataShapes())
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
	if len(rows) != 0 {
		t.Errorf("Fetch returned %d shapes, want 0", len(rows))
	}
}
