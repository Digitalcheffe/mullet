package mdiicons

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveCachesKnownIcon(t *testing.T) {
	dir := t.TempDir()
	Init(dir)
	t.Cleanup(func() { Init("") })

	filename, ok := Resolve("mdi:water-percent")
	if !ok || filename != "water-percent.svg" {
		t.Fatalf("Resolve(mdi:water-percent) = (%q, %v), want (water-percent.svg, true)", filename, ok)
	}

	data, err := os.ReadFile(filepath.Join(dir, filename))
	if err != nil {
		t.Fatalf("reading cached file: %v", err)
	}
	if len(data) == 0 {
		t.Error("cached file is empty")
	}

	// Second call should hit the already-cached path, not re-read the
	// embedded FS -- same result either way, just exercising that branch.
	filename2, ok2 := Resolve("mdi:water-percent")
	if !ok2 || filename2 != filename {
		t.Errorf("second Resolve = (%q, %v), want same result", filename2, ok2)
	}
}

func TestResolveRejectsUnknownOrInvalidNames(t *testing.T) {
	Init(t.TempDir())
	t.Cleanup(func() { Init("") })

	cases := []string{
		"mdi:this-icon-does-not-exist-xyz",
		"mdi:",
		"not-mdi:water-percent",
		"water-percent", // missing prefix entirely
		"mdi:../../../etc/passwd",
		"mdi:Water_Percent", // wrong case/underscore, not @mdi/svg's naming
	}
	for _, c := range cases {
		if filename, ok := Resolve(c); ok {
			t.Errorf("Resolve(%q) = (%q, true), want ok=false", c, filename)
		}
	}
}

func TestResolveNoOpsBeforeInit(t *testing.T) {
	Init("") // simulate never having been configured
	if filename, ok := Resolve("mdi:water-percent"); ok {
		t.Errorf("Resolve before Init = (%q, true), want ok=false", filename)
	}
}
