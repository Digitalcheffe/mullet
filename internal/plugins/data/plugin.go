// Package data defines the DataPlugin contract that every data plugin
// (compiled into the binary) implements, and the manifest that drives the
// admin UI's setup wizard for it.
package data

import (
	"context"
	"time"
)

// DataPlugin is implemented by every compiled-in data plugin. The
// scheduler calls Fetch() on RefreshInterval() and writes each entry of
// the returned map to the shape table named by its key -- a plugin
// declares which shapes it can produce via DataShapes(), and a plugin
// with none (e.g. clock) returns an empty map from Fetch().
type DataPlugin interface {
	ID() string
	Name() string
	Manifest() DataPluginManifest
	DataShapes() []string
	RefreshInterval() time.Duration
	Configure(cfg map[string]any) error
	Fetch(ctx context.Context) (map[string][]any, error)
}

// DataPluginManifest drives the admin UI's setup wizard: SetupFields is
// rendered as a form, and the plugin is configured with whatever the user
// submits.
type DataPluginManifest struct {
	ID                  string
	Name                string
	Description         string
	DataShapes          []string
	SetupFields         []SetupField
	AuthType            string // "none", "api_key", "oauth2"
	OAuthConfig         *OAuthConfig
	RecommendedInterval time.Duration
	MinInterval         time.Duration
}

// SetupField describes one field of a plugin's setup wizard form.
type SetupField struct {
	Key         string
	Label       string
	Type        string // "text", "select", "multi-select", "number", "toggle", "password"
	Required    bool
	Default     any
	Placeholder string
	HelpText    string
	Options     []SelectOption
	// Dynamic marks a "select"/"multi-select" field whose real Options
	// aren't known until the plugin's own Discover (see Discoverable)
	// is called against a specific, already-authorized instance --
	// Options is empty in the manifest for a Dynamic field; the admin UI
	// fetches the live list itself once an instance exists to ask.
	Dynamic bool
}

// SelectOption is one choice in a "select" or "multi-select" SetupField.
type SelectOption struct {
	Value string
	Label string
}

// OAuthConfig describes the OAuth2 flow for a plugin whose AuthType is
// "oauth2".
type OAuthConfig struct {
	AuthURL     string
	TokenURL    string
	Scopes      []string
	TenantField string // which SetupField holds the tenant ID, if applicable
}

// DiscoveredOption is one entry a Discoverable plugin can offer for a
// SetupField whose real choices only exist once the plugin has live
// credentials to ask the provider with -- e.g. "which of your Outlook
// calendars" can't be known until after OAuth, unlike a manifest's
// ordinary static SelectOption list.
type DiscoveredOption struct {
	Value string
	Label string
}

// Discoverable is implemented by a plugin whose manifest declares a
// SetupField with Dynamic: true -- typically one whose real options
// depend on the specific account just authorized (e.g. "which calendars"
// for msgraph-calendar). Optional: most plugins don't implement it, so
// the admin API type-asserts a registered DataPlugin against this
// interface rather than it being part of DataPlugin itself.
type Discoverable interface {
	// Discover returns the live options for field (a SetupField.Key on
	// this plugin's own manifest), using cfg the same way Configure
	// would -- including the framework-injected access token for an
	// OAuth2 plugin, so it can actually call out to the provider.
	Discover(ctx context.Context, field string, cfg map[string]any) ([]DiscoveredOption, error)
}
