// Package data defines the DataPlugin contract that every data plugin
// (compiled into the binary) implements, and the manifest that drives the
// admin UI's setup wizard for it.
package data

import (
	"context"
	"time"
)

// DataPlugin is implemented by every compiled-in data plugin. The
// scheduler calls Fetch() on RefreshInterval() and writes the results to
// the shape table for DataShape().
type DataPlugin interface {
	ID() string
	Name() string
	Manifest() DataPluginManifest
	DataShape() string
	RefreshInterval() time.Duration
	Configure(cfg map[string]any) error
	Fetch(ctx context.Context) ([]any, error)
}

// DataPluginManifest drives the admin UI's setup wizard: SetupFields is
// rendered as a form, and the plugin is configured with whatever the user
// submits.
type DataPluginManifest struct {
	ID                  string
	Name                string
	Description         string
	DataShape           string
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
