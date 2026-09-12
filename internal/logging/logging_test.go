package logging

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// resetToStdout restores the shared logger to its default (stdout-only)
// state after a test -- Configure changes process-wide state (the
// standard library's default logger), so every test that calls it must
// clean up or it'll bleed into whatever test runs next in this binary.
func resetToStdout(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		if err := Configure(""); err != nil {
			t.Errorf("resetting to stdout: %v", err)
		}
	})
}

func TestConfigureWritesToFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mullet.log")
	resetToStdout(t) // registered after TempDir's own cleanup, so it closes the file first (Windows can't delete an open file)

	if err := Configure(path); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	log.Print("CONFIGURE_TEST_MESSAGE")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	if !strings.Contains(string(data), "CONFIGURE_TEST_MESSAGE") {
		t.Errorf("log file = %q, want it to contain the logged message", data)
	}
}

func TestConfigureEmptyPathRevertsToStdout(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mullet.log")
	resetToStdout(t)

	if err := Configure(path); err != nil {
		t.Fatalf("Configure(path): %v", err)
	}
	if err := Configure(""); err != nil {
		t.Fatalf("Configure(\"\"): %v", err)
	}
	if log.Writer() != os.Stdout {
		t.Error("log.Writer() after Configure(\"\") is not os.Stdout")
	}
}

func TestConfigureInvalidPathLeavesPreviousDestinationInPlace(t *testing.T) {
	goodPath := filepath.Join(t.TempDir(), "mullet.log")
	// A path inside a directory that doesn't exist can never be opened
	// -- os.OpenFile doesn't create parent directories.
	badPath := filepath.Join(t.TempDir(), "does-not-exist", "mullet.log")
	resetToStdout(t)

	if err := Configure(goodPath); err != nil {
		t.Fatalf("Configure(goodPath): %v", err)
	}
	writerBefore := log.Writer()

	if err := Configure(badPath); err == nil {
		t.Fatal("Configure(badPath) = nil error, want one (the parent directory doesn't exist)")
	}

	if log.Writer() != writerBefore {
		t.Error("a failed Configure call changed the active log destination -- it should leave the previous one in place")
	}
}

func TestConfigureSwitchingFilesRoutesToTheNewOne(t *testing.T) {
	first := filepath.Join(t.TempDir(), "first.log")
	second := filepath.Join(t.TempDir(), "second.log")
	resetToStdout(t)

	if err := Configure(first); err != nil {
		t.Fatalf("Configure(first): %v", err)
	}
	log.Print("MESSAGE_ONE")

	if err := Configure(second); err != nil {
		t.Fatalf("Configure(second): %v", err)
	}
	log.Print("MESSAGE_TWO")

	firstData, _ := os.ReadFile(first)
	secondData, _ := os.ReadFile(second)
	if !strings.Contains(string(firstData), "MESSAGE_ONE") {
		t.Errorf("first log file = %q, want MESSAGE_ONE", firstData)
	}
	if strings.Contains(string(firstData), "MESSAGE_TWO") {
		t.Errorf("first log file = %q, want it to stop receiving messages after switching", firstData)
	}
	if !strings.Contains(string(secondData), "MESSAGE_TWO") {
		t.Errorf("second log file = %q, want MESSAGE_TWO", secondData)
	}
}
