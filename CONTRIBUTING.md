# Contributing to bino-plugin-sdk

Thank you for your interest in contributing to the bino Plugin SDK! This document provides guidelines and information for contributors.

## How to Contribute

### Reporting Bugs

- Use the [issue tracker](https://github.com/bino-bi/bino-plugin-sdk/issues) to report bugs.
- Include your Go version (`go version`), OS, and SDK version (from `go.mod`).
- Provide a minimal plugin that reproduces the issue when possible.

### Suggesting Features

- Open a [discussion](https://github.com/bino-bi/bino-plugin-sdk/discussions) to propose new features before writing code.
- Describe the use case and how the feature fits into the plugin model (DataSources, Components, Lint Rules, Hooks, Commands, Host access).

### Submitting Changes

1. Fork the repository and create a branch from `main`.
2. If you add code, add tests. Run the test suite:
   ```bash
   go test -v -race ./...
   ```
3. Run the linter:
   ```bash
   golangci-lint run ./...
   ```
4. Ensure your commit messages are clear and descriptive.
5. Open a pull request against `main`.

## Development Setup

### Prerequisites

- Go 1.24 or later
- `protoc` with `protoc-gen-go` and `protoc-gen-go-grpc` (only if modifying `proto/v1/plugin.proto`)
- `golangci-lint`

### Building

```bash
go build ./...
```

### Testing

```bash
go test -v -race ./...
go test -v -race -coverprofile=coverage.out ./...
```

## Proto Contract

The gRPC contract in `proto/v1/plugin.proto` is consumed by both the bino host and every plugin built against this SDK. Changes must be **backward compatible**:

- Do not remove fields.
- Do not change field numbers.
- Do not change existing field types.
- New fields must be optional and have sensible zero-value defaults.

After modifying the proto file, regenerate the stubs:

```bash
make proto
```

Commit the regenerated `plugin.pb.go` and `plugin_grpc.pb.go` in the same PR.

## Code Style

- Follow standard Go conventions (`gofmt`, `go vet`).
- Use `golangci-lint` with the project's configuration.
- Prefer table-driven tests with `t.Run` subtests.
- Keep functions focused and well-documented.

## License

bino-plugin-sdk is licensed under the **Apache License 2.0** — see the [LICENSE](LICENSE) file. By contributing, you agree that your contributions will be licensed under the same terms (inbound=outbound).
