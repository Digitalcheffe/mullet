package msgraphtodo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Digitalcheffe/mullet/internal/shapes"
)

// fakeGraphServer mimics just enough of the real Microsoft Graph API for
// this plugin to run its full Configure/Fetch/Discover cycle against --
// see msgraphcalendar's own fakeGraphServer for the general approach.
func fakeGraphServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/me/todo/lists", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Lists request Authorization = %q, want Bearer test-token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"value": []map[string]any{
				{"id": "list-tasks", "displayName": "Tasks"},
				{"id": "list-groceries", "displayName": "Groceries"},
			},
		})
	})

	mux.HandleFunc("/me/todo/lists/list-tasks/tasks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"value": []map[string]any{
				{"id": "task-1", "title": "Finish report", "status": "notStarted", "dueDateTime": map[string]string{"dateTime": "2026-01-15T00:00:00.0000000", "timeZone": "UTC"}},
				{"id": "task-2", "title": "Review PR", "status": "completed"},
			},
		})
	})

	mux.HandleFunc("/me/todo/lists/list-groceries/tasks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"value": []map[string]any{
				{"id": "task-3", "title": "Buy milk", "status": "notStarted"},
			},
		})
	})

	return httptest.NewServer(mux)
}

func TestFetchAllTaskLists(t *testing.T) {
	srv := fakeGraphServer(t)
	defer srv.Close()

	p := New()
	p.baseURL = srv.URL
	if err := p.Configure(map[string]any{"access_token": "test-token"}); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	result, err := p.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	tasks := result["tasks"]
	if len(tasks) != 3 {
		t.Fatalf("got %d tasks, want 3 (2 from Tasks + 1 from Groceries)", len(tasks))
	}

	byID := map[string]shapes.Task{}
	for _, row := range tasks {
		tk := row.(shapes.Task)
		byID[tk.ID] = tk
	}

	report, ok := byID["task-1@list-tasks"]
	if !ok {
		t.Fatalf("tasks = %+v, missing task-1@list-tasks", byID)
	}
	if report.Title != "Finish report" || report.Completed {
		t.Errorf("report = %+v, want Title=Finish report, Completed=false", report)
	}
	if report.DueDate == nil || *report.DueDate != "2026-01-15" {
		t.Errorf("report.DueDate = %v, want 2026-01-15", report.DueDate)
	}
	if report.TaskListExternalID != "list-tasks" || report.TaskListName != "Tasks" {
		t.Errorf("report list = (%q, %q), want (list-tasks, Tasks)", report.TaskListExternalID, report.TaskListName)
	}

	review, ok := byID["task-2@list-tasks"]
	if !ok || !review.Completed {
		t.Errorf("review = %+v (ok=%v), want Completed=true", review, ok)
	}
	if review.DueDate != nil {
		t.Errorf("review.DueDate = %v, want nil (no due date set)", *review.DueDate)
	}

	milk, ok := byID["task-3@list-groceries"]
	if !ok || milk.TaskListName != "Groceries" {
		t.Errorf("milk = %+v (ok=%v), want TaskListName=Groceries", milk, ok)
	}
}

func TestFetchFiltersToSelectedTaskLists(t *testing.T) {
	srv := fakeGraphServer(t)
	defer srv.Close()

	p := New()
	p.baseURL = srv.URL
	if err := p.Configure(map[string]any{"access_token": "test-token", "task_lists": []any{"list-groceries"}}); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	result, err := p.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	tasks := result["tasks"]
	if len(tasks) != 1 {
		t.Fatalf("got %d tasks, want 1 (groceries list only)", len(tasks))
	}
	tk := tasks[0].(shapes.Task)
	if tk.TaskListExternalID != "list-groceries" {
		t.Errorf("TaskListExternalID = %q, want list-groceries", tk.TaskListExternalID)
	}
}

func TestConfigureRequiresAccessToken(t *testing.T) {
	p := New()
	if err := p.Configure(map[string]any{}); err == nil {
		t.Error("Configure with no access_token: expected error, got nil")
	}
}

func TestDiscoverTaskLists(t *testing.T) {
	srv := fakeGraphServer(t)
	defer srv.Close()

	p := New()
	p.baseURL = srv.URL

	options, err := p.Discover(context.Background(), "task_lists", map[string]any{"access_token": "test-token"})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(options) != 2 {
		t.Fatalf("got %d options, want 2", len(options))
	}
	if options[0].Value != "list-tasks" || options[0].Label != "Tasks" {
		t.Errorf("options[0] = %+v, want {list-tasks, Tasks}", options[0])
	}
}

func TestDiscoverUnknownFieldFails(t *testing.T) {
	p := New()
	if _, err := p.Discover(context.Background(), "not-a-real-field", map[string]any{"access_token": "x"}); err == nil {
		t.Error("Discover(unknown field): expected error, got nil")
	}
}

func TestManifestDeclaresOAuth2(t *testing.T) {
	m := New().Manifest()
	if m.AuthType != "oauth2" {
		t.Errorf("AuthType = %q, want oauth2", m.AuthType)
	}
	if m.OAuthConfig == nil || m.OAuthConfig.TenantField != "tenant" {
		t.Errorf("OAuthConfig = %+v, want TenantField=tenant", m.OAuthConfig)
	}
	found := false
	for _, f := range m.SetupFields {
		if f.Key == "task_lists" {
			found = true
			if !f.Dynamic {
				t.Error("task_lists field is not marked Dynamic")
			}
		}
	}
	if !found {
		t.Error("manifest has no \"task_lists\" SetupField")
	}
}
