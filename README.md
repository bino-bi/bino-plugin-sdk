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

- **Custom DataSources** — fetch data from SaaS APIs, proprietary databases, or
  any external system
- **Visual Components** — ship Web Component JS/CSS for custom chart types or
  widgets
- **Lint Rules** — validate manifests with domain-specific rules (with optional
  access to datasets and DuckDB)
- **Pipeline Hooks** — react to build events (post-load, post-dataset,
  post-render-html, post-render-pdf)
- **CLI Commands** — add subcommands to `bino plugin exec <name>:<command>`
- **Host Queries** — call back to the bino host to execute DuckDB SQL, fetch
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
| `RenderComponent` | `func(ctx, *RenderRequest) (*RenderResponse, error)` | Component HTML renderer |

### Host access

Plugin callbacks receive a `HostClient` for querying the bino host:

```go
CollectDataSource: func(ctx context.Context, req *sdk.CollectRequest) (*sdk.CollectResponse, error) {
    if req.Host != nil {
        // Query the host's DuckDB
        result, _ := req.Host.QueryDuckDB(ctx, "SELECT count(*) as n FROM other_source")
        // List loaded documents
        docs, _ := req.Host.ListDocuments(ctx, "DataSource")
    }
    // ...
},
```

Available on: `CollectRequest.Host`, `RenderRequest.Host`, `HookPayload.Host`,
`LintContext.Host`, and via `Command.RunWithHost`.

### Enriched linting

Use `CheckWithContext` for lint rules that need datasets or host access:

```go
LintRules: []sdk.LintRule{{
    ID: "max-rows",
    CheckWithContext: func(ctx context.Context, docs []sdk.Document, lc *sdk.LintContext) []sdk.Finding {
        if lc.Host != nil {
            result, _ := lc.Host.QueryDuckDB(ctx, "SELECT count(*) as n FROM my_table")
            // ... check row count
        }
        return nil
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
- Pipeline hooks (`post-load`, `post-dataset-execute`, `post-render-html`)
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
  convert.go               # Proto ↔ Go type conversions
```

## Documentation

- **bino CLI docs**: [cli.bino.bi](https://cli.bino.bi/)
- **Plugin guide**: [cli.bino.bi/guides/plugins](https://cli.bino.bi/guides/plugins/)
- **Proto contract**: [`proto/v1/plugin.proto`](./proto/v1/plugin.proto)

## Contributing

Contributions are welcome. Please:

1. **Open an issue first** to discuss the change you'd like to make.
2. **Fork the repo** and create a feature branch.
3. **Write tests** — table-driven tests with `t.Run` subtests preferred.
4. **Run checks** before submitting:
   ```bash
   go vet ./...
   golangci-lint run ./...
   go test -race ./...
   ```
5. **Keep the proto contract backward-compatible** — no removed fields, no
   changed field numbers.
6. **Regenerate proto** if you modify `plugin.proto`:
   ```bash
   make proto
   ```

### Requirements

- Go 1.24+
- `protoc` with `protoc-gen-go` and `protoc-gen-go-grpc` for proto generation
- `golangci-lint` for linting

## License

[Apache License 2.0](./LICENSE)
