# bino Plugin SDK

Go SDK for building [bino](https://cli.bino.bi/) plugins.

> **Early access** — The plugin API is subject to change.

## What is bino?

[bino](https://cli.bino.bi/) is a CLI tool for building pixel-perfect PDF
reports from YAML manifests and SQL queries. It uses DuckDB for analytical SQL
and Chrome headless for PDF rendering. See the
[documentation](https://cli.bino.bi/) for more.

## What plugins can do

A single plugin binary can provide any combination of:

- **Custom DataSources**: fetch data from SaaS APIs, proprietary databases, or
  any external system
- **Visual Components**: ship Web Component JS/CSS for custom chart types or
  widgets
- **Lint Rules**: validate manifests with domain-specific rules (with optional
  access to datasets and host documents)
- **Pipeline Hooks**: react to build events (post-dataset-execute,
  post-render-html)
- **CLI Commands**: add subcommands to `bino plugin exec <name>:<command>`
- **Host Queries**: call back to the bino host to execute DuckDB SQL, fetch
  documents, or retrieve dataset results

## Quick start

```bash
go get github.com/bino-bi/bino-plugin-sdk@latest
```

Create `main.go`:

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"

    sdk "github.com/bino-bi/bino-plugin-sdk"
)

func main() {
    sdk.Serve(&sdk.PluginOpts{
        Name:        "myplugin",
        Version:     "0.1.0",
        Description: "My custom bino plugin",

        // Register a custom DataSource kind.
        Kinds: []sdk.Kind{{
            Name:           "MyDataSource",
            Category:       sdk.KindDataSource,
            DataSourceType: "my_api",
        }},

        // Handle data collection.
        CollectDataSource: func(ctx context.Context, req *sdk.CollectRequest) (*sdk.CollectResponse, error) {
            rows, _ := json.Marshal([]map[string]any{{"id": 1, "value": 42}})
            return &sdk.CollectResponse{JSONRows: rows}, nil
        },

        // Add a lint rule.
        LintRules: []sdk.LintRule{{
            ID:          "require-description",
            Description: "Every document should have a description",
            Check: func(ctx context.Context, docs []sdk.Document) []sdk.Finding {
                // ... inspect docs, return findings
                return nil
            },
        }},
    })
}
```

Build and install:

```bash
go build -o bino-plugin-myplugin .

# Place in project
cp bino-plugin-myplugin <your-project>/.bino/plugins/
```

Add to `bino.toml`:

```toml
[plugins.myplugin]
```

Verify: `bino plugin list`

## API overview

### `sdk.Serve(opts)`

Entry point — starts the gRPC server and blocks until the host terminates the
plugin. Call this as the last line of `main()`.

### `PluginOpts`

| Field | Type | Description |
|-------|------|-------------|
| `Name` | `string` | Plugin name (must match bino.toml key) |
| `Version` | `string` | Semver version |
| `Description` | `string` | One-line description |
| `Kinds` | `[]Kind` | Custom kind registrations |
| `DuckDBExtensions` | `[]string` | DuckDB extensions to load |
| `CollectDataSource` | `func(ctx, *CollectRequest) (*CollectResponse, error)` | DataSource handler |
| `LintRules` | `[]LintRule` | Lint rule definitions |
| `Assets` | `[]Asset` | Static JS/CSS assets |
| `GetAssets` | `func(ctx, renderMode) ([]Asset, error)` | Dynamic asset callback |
| `Commands` | `[]Command` | CLI subcommands |
| `Hooks` | `map[string]HookFunc` | Pipeline hook callbacks |
| `RenderComponent` | `func(ctx, *RenderRequest) (*RenderResponse, error)` | Component HTML renderer; the request carries the spec, the `spec.dataset` rows and the inherited style |
| `OnInit` | `func(ctx, InitInfo) error` | Called once at startup with the plugin config |

### Kind schemas

The host validates every spec of a plugin kind against `Kind.Schema`. Derive the
schema from the Go struct you decode the spec into, so the two cannot drift:

```go
type ChartSpec struct {
    Dataset string   `json:"dataset" jsonschema:"Name of the DataSet to plot"`
    Series  []string `json:"series,omitempty"`
}

Kinds: []sdk.Kind{{Name: "HighchartsChart", Category: sdk.KindComponent,
    Schema: sdk.SchemaFor[ChartSpec]()}},
```

`SchemaFor` emits JSON Schema draft 2020-12, the draft the host uses:

- Property names come from `json` tags.
- Fields with `omitempty` or `omitzero` are optional; all others are required.
- Structs reject unknown properties (`additionalProperties: false`).
- The `jsonschema` struct tag is plain description text only. A tag that starts
  with `WORD=` (for example `description=...`) is rejected.
- Slices are typed `["null","array"]`, so `null` passes for a slice field.
- A `json.RawMessage` field accepts any value. A `[]byte` field is a string or
  `null`, as `encoding/json` writes it in base64.

`SchemaFor` panics when the type cannot be expressed as a schema, for example a
`func` field or a rejected tag. It runs at startup, so the mistake shows at once.

### Host access

Plugin callbacks receive a `HostClient` for querying the bino host:

```go
RenderComponent: func(ctx context.Context, req *sdk.RenderRequest) (*sdk.RenderResponse, error) {
    if req.Host != nil {
        // Query an executed DataSet by its metadata.name.
        result, err := req.Host.QueryDuckDB(ctx, "SELECT region, sum(amount) AS total FROM sales GROUP BY region")
        if err != nil {
            return nil, err
        }
        // List loaded documents.
        docs, err := req.Host.ListDocuments(ctx, "DataSet")
        if err != nil {
            return nil, err
        }
        // ... use result.JSONRows and docs
    }
    // ...
},
```

`QueryDuckDB` runs in a throwaway DuckDB session that holds one view per
executed DataSet of the artefact being rendered, named by `metadata.name`.
DataSources are not visible. DataSets are there from the `post-dataset-execute`
hook onward (`RenderComponent` and later hooks), not during
`CollectDataSource`, and not in lint. The views need a bino release after
v0.100.0. A failing query returns an error, and the result still carries the
host's diagnostics.

Available on: `CollectRequest.Host`, `RenderRequest.Host`, `HookPayload.Host`,
`LintContext.Host`, `InitInfo.Host`, and via `Command.RunWithHost`.

### Component rendering

`RenderRequest` carries what a component needs to draw:

- `Spec`: JSON of the effective spec.
- `Datasets`: the rows of the DataSets named in `spec.dataset`, in spec order.
  DataSource references (`$name`) are not included, and `Columns` may be empty.
- `InheritedStyle`: the name of the ComponentStyle set on the nearest enclosing
  artefact, page or card, or `""`. The host never writes it into `Spec`. Read
  the style itself with `Host.GetDocument(ctx, "ComponentStyle", name)`.

A DataSet named in `spec.dataset` that is missing from `Datasets` must be
fetched with `Host.GetDatasetResult`. The host leaves a DataSet out when it
would make the request too large, or when the host is older and sends no
datasets at all. DataSource rows are neither sent nor served by
`GetDatasetResult`.

```go
RenderComponent: func(ctx context.Context, req *sdk.RenderRequest) (*sdk.RenderResponse, error) {
    var spec struct {
        Dataset string `json:"dataset"`
    }
    if err := json.Unmarshal(req.Spec, &spec); err != nil {
        return nil, err
    }
    var rows []byte
    for _, ds := range req.Datasets {
        if ds.Name == spec.Dataset {
            rows = ds.JSONRows
        }
    }
    if rows == nil && req.Host != nil {
        ds, err := req.Host.GetDatasetResult(ctx, spec.Dataset)
        if err != nil {
            return nil, err
        }
        if ds != nil {
            rows = ds.JSONRows
        }
    }
    return &sdk.RenderResponse{HTML: fmt.Sprintf(
        `<myplugin-chart data-style="%s" data-rows="%s"></myplugin-chart>`,
        html.EscapeString(req.InheritedStyle), html.EscapeString(string(rows)),
    )}, nil
},
```

### Configuration

Users configure a plugin in `bino.toml` with a `config` string map:

```toml
[plugins.myplugin]
config = { theme = "dark", cdn_url = "https://cdn.example.com" }
```

The SDK passes it to `OnInit`, together with the project root, the bino
version and the host client. `OnInit` runs once, before any other callback.
If it returns an error, the plugin fails to load. Keep the values you need
for later calls:

```go
var theme = "light"

sdk.Serve(&sdk.PluginOpts{
    Name: "myplugin",
    OnInit: func(ctx context.Context, info sdk.InitInfo) error {
        if t, ok := info.Config["theme"]; ok {
            if t != "light" && t != "dark" {
                return fmt.Errorf("invalid theme %q", t)
            }
            theme = t
        }
        return nil
    },
    RenderComponent: func(ctx context.Context, req *sdk.RenderRequest) (*sdk.RenderResponse, error) {
        return &sdk.RenderResponse{HTML: `<div class="chart ` + theme + `"></div>`}, nil
    },
    GetAssets: func(ctx context.Context, renderMode string) ([]sdk.Asset, error) {
        return []sdk.Asset{{
            URLPath:  "/plugins/myplugin/" + theme + ".css",
            FilePath: "assets/" + theme + ".css",
        }}, nil
    },
})
```

### Enriched linting

Use `CheckWithContext` for lint rules that need datasets or host access.
`QueryDuckDB` is not available in lint; read the rows from `lc.Datasets`:

```go
LintRules: []sdk.LintRule{{
    ID: "max-rows",
    CheckWithContext: func(ctx context.Context, docs []sdk.Document, lc *sdk.LintContext) []sdk.Finding {
        if !lc.DatasetsAvailable {
            return nil
        }
        var findings []sdk.Finding
        for _, ds := range lc.Datasets {
            var rows []json.RawMessage
            if err := json.Unmarshal(ds.JSONRows, &rows); err != nil {
                continue
            }
            if len(rows) > 10000 {
                findings = append(findings, sdk.Finding{
                    Message:  fmt.Sprintf("dataset %s has %d rows", ds.Name, len(rows)),
                    Severity: sdk.Warning,
                })
            }
        }
        return findings
    },
}},
```

## Sample plugin (soon)

See [`sample-plugin/`](https://github.com/bino-bi/sample-plugin) for a complete
working example demonstrating all capabilities:

- Custom DataSource and Component kinds with JSON Schema
- Data collection with JSON rows
- Component HTML rendering (Web Component)
- JS/CSS asset injection
- Three lint rules (basic + enriched with `CheckWithContext`)
- CLI commands with `Run` and `RunWithHost`
- Pipeline hooks (`post-dataset-execute`, `post-render-html`)
- Host access (`QueryDuckDB`, `ListDocuments`)

## Platform notes

Plugins are native executables — you need a separate binary per platform:

| Platform | Binary name | Notes |
|----------|-------------|-------|
| macOS | `bino-plugin-<name>` | May need `codesign --force --sign -` |
| Linux | `bino-plugin-<name>` | Must be executable (`chmod +x`) |
| Windows | `bino-plugin-<name>.exe` | `.exe` extension required |

bino auto-appends `.exe` on Windows during discovery. Cross-compile with:

```bash
GOOS=linux   GOARCH=amd64 go build -o bino-plugin-myplugin .
GOOS=windows GOARCH=amd64 go build -o bino-plugin-myplugin.exe .
GOOS=darwin  GOARCH=arm64 go build -o bino-plugin-myplugin .
```

## Project structure

```
bino-plugin-sdk/
  proto/v1/plugin.proto    # gRPC service contract (BinoPlugin + BinoHost)
  proto/v1/plugin.pb.go    # Generated message types
  proto/v1/plugin_grpc.pb.go # Generated gRPC stubs
  sdk.go                   # Serve() entry point, PluginOpts
  types.go                 # Public types (Kind, Document, Finding, Asset, ...)
  grpcserver.go            # gRPC server implementation
  hostclient.go            # HostClient for calling back to the bino host
  schema.go                # SchemaFor: JSON Schema from a Go spec struct
  convert.go               # Proto ↔ Go type conversions
```

## Documentation

- **bino CLI docs**: [cli.bino.bi](https://cli.bino.bi/)
- **Plugin guide**: [cli.bino.bi/guides/plugins](https://cli.bino.bi/guides/plugins/)
- **Proto contract**: [`proto/v1/plugin.proto`](./proto/v1/plugin.proto)

## Contributing

Contributions are welcome — see [CONTRIBUTING.md](./CONTRIBUTING.md) for guidelines, development setup, and the proto backward-compatibility policy. Please also review our [Code of Conduct](./CODE_OF_CONDUCT.md) and [Security Policy](./SECURITY.md).

## License

[Apache License 2.0](./LICENSE)
