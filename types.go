package pluginsdk

import "context"

// KindCategory determines how bino treats this kind.
type KindCategory int

const (
	KindDataSource KindCategory = iota
	KindComponent
	KindConfig
	KindArtifact
)

// Kind describes a custom kind this plugin provides.
type Kind struct {
	Name           string       // e.g., "SalesforceDataSource"
	Category       KindCategory
	DataSourceType string // Only for KindDataSource — routing key
	Schema         []byte // JSON Schema bytes for this kind
}

// CollectRequest is passed to the CollectDataSource callback.
type CollectRequest struct {
	Name        string            // metadata.name from the document
	Spec        []byte            // JSON of the spec section
	Env         map[string]string // Resolved environment variables
	ProjectRoot string
	Host        HostClient // Host access for DuckDB queries, etc. May be nil.
}

// CollectResponse is returned from the CollectDataSource callback.
type CollectResponse struct {
	JSONRows         []byte            // JSON array of row objects
	ColumnTypes      map[string]string // Optional DuckDB column type hints
	Ephemeral        bool              // Always re-fetch (don't cache)
	Diagnostics      []Diagnostic
	DuckDBExpression string // SQL expression registered as DuckDB view directly (overrides JSONRows)
}

// Document represents a manifest document passed to lint rules.
type Document struct {
	File     string
	Position int
	Kind     string
	Name     string
	Raw      []byte // JSON
}

// Finding is a lint finding produced by a plugin rule.
type Finding struct {
	RuleID   string
	Message  string
	File     string
	DocIdx   int
	Path     string // YAML path, e.g., "spec.connection"
	Line     int
	Column   int
	Severity Severity
}

// Severity levels for findings and diagnostics.
type Severity int

const (
	Warning Severity = iota
	Error
	Info
)

// LintRule describes a lint rule provided by the plugin.
type LintRule struct {
	ID          string
	Description string
	Check       func(ctx context.Context, docs []Document) []Finding
	// CheckWithContext is an alternative to Check that receives enriched lint context
	// (datasets, rendered HTML, host access). If set, it takes precedence over Check.
	CheckWithContext func(ctx context.Context, docs []Document, lintCtx *LintContext) []Finding
}

// Asset describes a JS/CSS file provided by the plugin.
type Asset struct {
	URLPath   string // Serving path, e.g., "/plugins/salesforce/chart.js"
	FilePath  string // Path to file on disk
	Content   []byte // Alternative: inline content
	MediaType string // MIME type (e.g., "application/javascript"); auto-detected if empty
	IsModule  bool   // For <script type="module">
}

// Command describes a CLI subcommand provided by the plugin.
type Command struct {
	Name  string
	Short string
	Long  string
	Usage string
	Flags []Flag
	Run   func(ctx context.Context, args []string, flags map[string]string) error
	// RunWithHost is an alternative to Run that receives host access.
	// If set, it takes precedence over Run.
	RunWithHost func(ctx context.Context, args []string, flags map[string]string, host HostClient) error
}

// Flag describes a CLI flag on a plugin command.
type Flag struct {
	Name         string
	Shorthand    string
	Description  string
	DefaultValue string
	Type         string // "string", "bool", "int"
	Required     bool
}

// HookPayload is the data passed to hook callbacks.
type HookPayload struct {
	Documents []Document
	HTML      []byte
	PDFPath   string
	Datasets  []Dataset
	Metadata  map[string]string
	Host      HostClient // Host access for DuckDB queries, etc. May be nil.
}

// Dataset represents a dataset in hook payloads.
type Dataset struct {
	Name     string
	JSONRows []byte
	Columns  []string
}

// HookResult is returned from a hook callback.
type HookResult struct {
	Modified    bool
	Payload     *HookPayload
	Diagnostics []Diagnostic
	Findings    []Finding // Structured lint findings from hook processing.
}

// HookFunc is the callback signature for pipeline hooks.
type HookFunc func(ctx context.Context, payload *HookPayload) (*HookResult, error)

// Diagnostic is a non-fatal message from the plugin.
type Diagnostic struct {
	Source   string   // Plugin name (auto-set if empty)
	Stage    string   // Pipeline stage (e.g., "collect", "lint")
	Message  string
	Severity Severity
}

// RenderRequest is passed to the RenderComponent callback.
type RenderRequest struct {
	Kind       string     // e.g., "ExampleChart"
	Name       string     // metadata.name
	Spec       []byte     // JSON of the effective spec
	RenderMode string     // "build" or "preview"
	Host       HostClient // Host access for DuckDB queries, etc. May be nil.
}

// RenderResponse is returned from the RenderComponent callback.
type RenderResponse struct {
	HTML        string // HTML fragment to insert into layout
	Diagnostics []Diagnostic
}

// LintContext provides enriched context for lint rules beyond just documents.
type LintContext struct {
	// Datasets contains pre-computed dataset results. Nil if not available.
	Datasets []Dataset
	// DatasetsAvailable is true when Datasets is populated.
	DatasetsAvailable bool
	// RenderedHTML contains the rendered HTML output. Nil if not available.
	RenderedHTML []byte
	// Host provides access to the bino CLI host. Nil if not available.
	Host HostClient
}
