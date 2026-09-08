// Package msgraphtodo fetches Microsoft To-Do tasks via the official
// Microsoft Graph SDK. Scoped to personal (consumer) Microsoft accounts,
// same as msgraphcalendar -- see that package's doc comment.
package msgraphtodo

import (
	"context"
	"fmt"
	"time"

	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/microsoftgraph/msgraph-sdk-go/users"

	"github.com/Digitalcheffe/mullet/internal/plugins/data/msgraphclient"

	plugindata "github.com/Digitalcheffe/mullet/internal/plugins/data"
	"github.com/Digitalcheffe/mullet/internal/shapes"
)

const ID = "msgraph-todo"

// tasksPerList caps each list's page -- see msgraphcalendar's
// eventsPerCalendar for the same rationale (no nextLink pagination yet).
const tasksPerList = int32(250)

func init() {
	if err := plugindata.Register(New()); err != nil {
		panic(err)
	}
}

// Plugin implements plugindata.DataPlugin and plugindata.Discoverable.
type Plugin struct {
	accessToken string
	taskListIDs []string // empty means "all task lists"

	// baseURL overrides the Graph API's base URL -- see
	// msgraphcalendar.Plugin.baseURL.
	baseURL string
}

func New() *Plugin { return &Plugin{} }

func (p *Plugin) ID() string   { return ID }
func (p *Plugin) Name() string { return "Microsoft To Do" }

func (p *Plugin) Manifest() plugindata.DataPluginManifest {
	return plugindata.DataPluginManifest{
		ID:          ID,
		Name:        "Microsoft To Do",
		Description: "Microsoft To-Do tasks via Microsoft Graph. Works with personal Microsoft accounts.",
		DataShapes:  []string{"tasks"},
		AuthType:    "oauth2",
		OAuthConfig: &plugindata.OAuthConfig{
			AuthURL:     "https://login.microsoftonline.com/{tenant}/oauth2/v2.0/authorize",
			TokenURL:    "https://login.microsoftonline.com/{tenant}/oauth2/v2.0/token",
			Scopes:      []string{"Tasks.Read", "offline_access"},
			TenantField: "tenant",
		},
		SetupFields: []plugindata.SetupField{
			{Key: "client_id", Label: "Client ID", Type: "text", Required: true, HelpText: "From your Azure App Registration."},
			{Key: "client_secret", Label: "Client Secret", Type: "password", Required: true},
			{
				Key: "tenant", Label: "Tenant", Type: "text", Required: true, Default: "consumers",
				HelpText: `Use "consumers" for a personal Outlook/Hotmail/Live account.`,
			},
			{
				Key: "task_lists", Label: "Task Lists", Type: "multi-select", Dynamic: true,
				HelpText: "Authorize first, then edit this instance to pick specific lists -- leave empty for all of them.",
			},
		},
		RecommendedInterval: 15 * time.Minute,
		MinInterval:         5 * time.Minute,
	}
}

func (p *Plugin) DataShapes() []string           { return []string{"tasks"} }
func (p *Plugin) RefreshInterval() time.Duration { return 15 * time.Minute }

// Configure mirrors msgraphcalendar.Plugin.Configure -- see its doc
// comment for why client_id/client_secret/tenant aren't read here.
func (p *Plugin) Configure(cfg map[string]any) error {
	token, _ := cfg["access_token"].(string)
	if token == "" {
		return fmt.Errorf("msgraph-todo: access_token is required (instance not authorized yet)")
	}
	p.accessToken = token
	p.taskListIDs = stringSlice(cfg["task_lists"])
	return nil
}

func (p *Plugin) Fetch(ctx context.Context) (map[string][]any, error) {
	client, err := newClient(p.accessToken, p.baseURL)
	if err != nil {
		return nil, err
	}

	lists, err := resolveTaskLists(ctx, client, p.taskListIDs)
	if err != nil {
		return nil, err
	}

	top := tasksPerList
	var rows []any
	for _, list := range lists {
		resp, err := client.Me().Todo().Lists().ByTodoTaskListId(list.id).Tasks().Get(ctx,
			&users.ItemTodoListsItemTasksRequestBuilderGetRequestConfiguration{
				QueryParameters: &users.ItemTodoListsItemTasksRequestBuilderGetQueryParameters{Top: &top},
			})
		if err != nil {
			return nil, fmt.Errorf("fetching tasks for %q: %w", list.name, err)
		}
		for i, tk := range resp.GetValue() {
			row, err := toShape(tk, list, i)
			if err != nil {
				// One malformed task shouldn't take down the whole list's fetch.
				continue
			}
			rows = append(rows, row)
		}
	}

	return map[string][]any{"tasks": rows}, nil
}

// Discover lists the account's task lists, for the admin UI's
// "task_lists" multi-select -- mirrors msgraphcalendar.Plugin.Discover.
func (p *Plugin) Discover(ctx context.Context, field string, cfg map[string]any) ([]plugindata.DiscoveredOption, error) {
	if field != "task_lists" {
		return nil, fmt.Errorf("msgraph-todo: no such discoverable field %q", field)
	}
	token, _ := cfg["access_token"].(string)
	if token == "" {
		return nil, fmt.Errorf("msgraph-todo: access_token is required")
	}
	client, err := newClient(token, p.baseURL)
	if err != nil {
		return nil, err
	}
	lists, err := resolveTaskLists(ctx, client, nil)
	if err != nil {
		return nil, err
	}
	options := make([]plugindata.DiscoveredOption, len(lists))
	for i, l := range lists {
		options[i] = plugindata.DiscoveredOption{Value: l.id, Label: l.name}
	}
	return options, nil
}

func newClient(accessToken, baseURL string) (*msgraphsdk.GraphServiceClient, error) {
	if baseURL != "" {
		return msgraphclient.NewWithBaseURL(accessToken, baseURL)
	}
	return msgraphclient.New(accessToken)
}

type taskListRef struct {
	id, name string
}

// resolveTaskLists mirrors msgraphcalendar.resolveCalendars -- see its
// doc comment for why it always lists everything before filtering.
func resolveTaskLists(ctx context.Context, client *msgraphsdk.GraphServiceClient, selectedIDs []string) ([]taskListRef, error) {
	resp, err := client.Me().Todo().Lists().Get(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("listing task lists: %w", err)
	}

	selected := make(map[string]bool, len(selectedIDs))
	for _, id := range selectedIDs {
		selected[id] = true
	}

	var refs []taskListRef
	for _, l := range resp.GetValue() {
		var id string
		if l.GetId() != nil {
			id = *l.GetId()
		}
		if len(selected) > 0 && !selected[id] {
			continue
		}
		var name string
		if l.GetDisplayName() != nil {
			name = *l.GetDisplayName()
		}
		refs = append(refs, taskListRef{id: id, name: name})
	}
	return refs, nil
}

func toShape(tk models.TodoTaskable, list taskListRef, sortOrder int) (shapes.Task, error) {
	var id string
	if tk.GetId() != nil {
		id = *tk.GetId()
	}
	var title string
	if tk.GetTitle() != nil {
		title = *tk.GetTitle()
	}

	completed := false
	if status := tk.GetStatus(); status != nil {
		completed = *status == models.COMPLETED_TASKSTATUS
	}

	var dueDate *string
	if due := tk.GetDueDateTime(); due != nil && due.GetDateTime() != nil && *due.GetDateTime() != "" {
		if t, err := parseGraphTime(*due.GetDateTime()); err == nil {
			d := t.Format("2006-01-02")
			dueDate = &d
		}
	}

	priority := "normal"
	if imp := tk.GetImportance(); imp != nil {
		priority = imp.String() // "low"/"normal"/"high", matching shapes.Task.Priority
	}

	return shapes.Task{
		// Namespaced by list, matching msgraphcalendar's event IDs.
		ID:                 fmt.Sprintf("%s@%s", id, list.id),
		Title:              title,
		Completed:          completed,
		DueDate:            dueDate,
		SortOrder:          sortOrder,
		Priority:           priority,
		TaskListExternalID: list.id,
		TaskListName:       list.name,
	}, nil
}

var graphTimeLayouts = []string{
	"2006-01-02T15:04:05.0000000",
	"2006-01-02T15:04:05",
	time.RFC3339,
}

func parseGraphTime(raw string) (time.Time, error) {
	for _, layout := range graphTimeLayouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized datetime format %q", raw)
}

func stringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}
