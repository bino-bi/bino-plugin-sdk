package pluginsdk

import (
	"context"

	goplugin "github.com/hashicorp/go-plugin"
)

// PluginOpts configures the plugin's capabilities.
type PluginOpts struct {
	Name        string
	Version     string
	Description string

	// Custom kinds this plugin provides.
	Kinds []Kind

	// DuckDB extensions this plugin needs loaded.
	DuckDBExtensions []string

	// DataSource collection handler.
	CollectDataSource func(ctx context.Context, req *CollectRequest) (*CollectResponse, error)

	// Lint rules.
	LintRules []LintRule

	// JS/CSS assets (static list, returned for all render modes).
	Assets []Asset

	// GetAssets is an alternative to the static Assets list that receives the
	// render mode ("build" or "preview") and can return different assets per mode.
	// If set, takes precedence over Assets.
	GetAssets func(ctx context.Context, renderMode string) ([]Asset, error)

	// CLI subcommands.
	Commands []Command

	// Pipeline hooks.
	Hooks map[string]HookFunc

	// Component rendering handler.
	// Called to generate HTML for plugin-provided component kinds.
	RenderComponent func(ctx context.Context, req *RenderRequest) (*RenderResponse, error)
}

// Serve starts the plugin gRPC server and blocks until the host terminates it.
// This should be the last call in the plugin's main().
func Serve(opts *PluginOpts) {
	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: handshake,
		Plugins: map[string]goplugin.Plugin{
			"bino": &binoPlugin{opts: opts},
		},
		GRPCServer: goplugin.DefaultGRPCServer,
	})
}

// handshake must match the host's handshake config exactly.
var handshake = goplugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "BINO_PLUGIN",
	MagicCookieValue: "bino-plugin-v1",
}
