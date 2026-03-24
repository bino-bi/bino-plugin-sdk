package pluginsdk

import (
	"context"
	"fmt"
	"testing"

	pluginv1 "github.com/bino-bi/bino-plugin-sdk/proto/v1"
)

func newTestServer(opts *PluginOpts) *grpcServer {
	return &grpcServer{opts: opts}
}

func TestGRPCServer_Init(t *testing.T) {
	s := newTestServer(&PluginOpts{
		Name:        "test-plugin",
		Version:     "1.0.0",
		Description: "A test plugin",
		Kinds: []Kind{
			{Name: "TestDataSource", Category: KindDataSource, DataSourceType: "test"},
			{Name: "TestConfig", Category: KindConfig},
		},
		DuckDBExtensions: []string{"httpfs"},
		LintRules:        []LintRule{{ID: "test/rule1"}},
		Assets:           []Asset{{URLPath: "/plugins/test/chart.js"}},
		Commands: []Command{
			{Name: "auth", Short: "Authenticate"},
		},
		Hooks: map[string]HookFunc{
			"post-load": func(ctx context.Context, p *HookPayload) (*HookResult, error) {
				return &HookResult{}, nil
			},
		},
	})

	resp, err := s.Init(context.Background(), &pluginv1.InitRequest{
		Config:      map[string]string{"key": "val"},
		ProjectRoot: "/tmp/project",
		BinoVersion: "0.15.0",
	})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if resp.GetName() != "test-plugin" {
		t.Fatalf("got name %q, want %q", resp.GetName(), "test-plugin")
	}
	if resp.GetVersion() != "1.0.0" {
		t.Fatalf("got version %q, want %q", resp.GetVersion(), "1.0.0")
	}
	if len(resp.GetKinds()) != 2 {
		t.Fatalf("expected 2 kinds, got %d", len(resp.GetKinds()))
	}
	if resp.GetKinds()[0].GetKindName() != "TestDataSource" {
		t.Fatalf("got kind %q, want %q", resp.GetKinds()[0].GetKindName(), "TestDataSource")
	}
	if !resp.GetProvidesLinter() {
		t.Fatal("expected ProvidesLinter to be true")
	}
	if !resp.GetProvidesAssets() {
		t.Fatal("expected ProvidesAssets to be true")
	}
	if len(resp.GetCommands()) != 1 {
		t.Fatalf("expected 1 command, got %d", len(resp.GetCommands()))
	}
	if len(resp.GetHooks()) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(resp.GetHooks()))
	}
	if len(resp.GetDuckdbExtensions()) != 1 || resp.GetDuckdbExtensions()[0] != "httpfs" {
		t.Fatalf("expected duckdb_extensions [httpfs], got %v", resp.GetDuckdbExtensions())
	}

	// Verify config was stored.
	if s.config["key"] != "val" {
		t.Fatalf("expected config key=val, got %v", s.config)
	}
}

func TestGRPCServer_GetSchemas(t *testing.T) {
	schema := []byte(`{"type":"object","properties":{"spec":{"type":"object"}}}`)
	s := newTestServer(&PluginOpts{
		Kinds: []Kind{
			{Name: "WithSchema", Schema: schema},
			{Name: "NoSchema"},
		},
	})

	resp, err := s.GetSchemas(context.Background(), &pluginv1.GetSchemasRequest{})
	if err != nil {
		t.Fatalf("GetSchemas failed: %v", err)
	}
	if len(resp.GetSchemas()) != 1 {
		t.Fatalf("expected 1 schema, got %d", len(resp.GetSchemas()))
	}
	if string(resp.GetSchemas()["WithSchema"]) != string(schema) {
		t.Fatal("schema mismatch")
	}
}

func TestGRPCServer_CollectDataSource(t *testing.T) {
	s := newTestServer(&PluginOpts{
		Name: "test",
		CollectDataSource: func(ctx context.Context, req *CollectRequest) (*CollectResponse, error) {
			if req.Name != "my-ds" {
				t.Fatalf("got name %q, want %q", req.Name, "my-ds")
			}
			return &CollectResponse{
				JSONRows:  []byte(`[{"a":1}]`),
				Ephemeral: true,
			}, nil
		},
	})

	resp, err := s.CollectDataSource(context.Background(), &pluginv1.CollectDataSourceRequest{
		Name:    "my-ds",
		RawSpec: []byte(`{"type":"test"}`),
	})
	if err != nil {
		t.Fatalf("CollectDataSource failed: %v", err)
	}
	if string(resp.GetJsonRows()) != `[{"a":1}]` {
		t.Fatalf("got rows %q", string(resp.GetJsonRows()))
	}
	if !resp.GetEphemeral() {
		t.Fatal("expected ephemeral=true")
	}
}

func TestGRPCServer_CollectDataSource_NotImplemented(t *testing.T) {
	s := newTestServer(&PluginOpts{Name: "test"})

	_, err := s.CollectDataSource(context.Background(), &pluginv1.CollectDataSourceRequest{})
	if err == nil {
		t.Fatal("expected error when CollectDataSource not implemented")
	}
}

func TestGRPCServer_Lint(t *testing.T) {
	s := newTestServer(&PluginOpts{
		LintRules: []LintRule{
			{
				ID: "test/always-warn",
				Check: func(ctx context.Context, docs []Document) []Finding {
					var findings []Finding
					for _, d := range docs {
						findings = append(findings, Finding{
							Message:  "warning on " + d.Name,
							File:     d.File,
							Severity: Warning,
						})
					}
					return findings
				},
			},
		},
	})

	resp, err := s.Lint(context.Background(), &pluginv1.LintRequest{
		Documents: []*pluginv1.LintDocument{
			{File: "a.yaml", Name: "ds1"},
			{File: "b.yaml", Name: "ds2"},
		},
	})
	if err != nil {
		t.Fatalf("Lint failed: %v", err)
	}
	if len(resp.GetFindings()) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(resp.GetFindings()))
	}
	if resp.GetFindings()[0].GetRuleId() != "test/always-warn" {
		t.Fatalf("got rule_id %q", resp.GetFindings()[0].GetRuleId())
	}
}

func TestGRPCServer_GetAssets(t *testing.T) {
	s := newTestServer(&PluginOpts{
		Assets: []Asset{
			{URLPath: "/plugins/test/chart.js", Content: []byte("js")},
			{URLPath: "/plugins/test/style.css", Content: []byte("css")},
			{URLPath: "/plugins/test/module.mjs", IsModule: true},
		},
	})

	resp, err := s.GetAssets(context.Background(), &pluginv1.GetAssetsRequest{RenderMode: "build"})
	if err != nil {
		t.Fatalf("GetAssets failed: %v", err)
	}
	if len(resp.GetScripts()) != 2 { // .js and .mjs
		t.Fatalf("expected 2 scripts, got %d", len(resp.GetScripts()))
	}
	if len(resp.GetStyles()) != 1 {
		t.Fatalf("expected 1 style, got %d", len(resp.GetStyles()))
	}
}

func TestGRPCServer_ListCommands(t *testing.T) {
	s := newTestServer(&PluginOpts{
		Commands: []Command{
			{
				Name: "auth", Short: "Authenticate",
				Flags: []Flag{
					{Name: "token", Type: "string", Required: true},
				},
			},
			{Name: "sync", Short: "Sync data"},
		},
	})

	resp, err := s.ListCommands(context.Background(), &pluginv1.ListCommandsRequest{})
	if err != nil {
		t.Fatalf("ListCommands failed: %v", err)
	}
	if len(resp.GetCommands()) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(resp.GetCommands()))
	}
	if resp.GetCommands()[0].GetName() != "auth" {
		t.Fatalf("got command name %q", resp.GetCommands()[0].GetName())
	}
	if len(resp.GetCommands()[0].GetFlags()) != 1 {
		t.Fatalf("expected 1 flag, got %d", len(resp.GetCommands()[0].GetFlags()))
	}
}

func TestGRPCServer_OnHook_Subscribed(t *testing.T) {
	called := false
	s := newTestServer(&PluginOpts{
		Hooks: map[string]HookFunc{
			"post-load": func(ctx context.Context, p *HookPayload) (*HookResult, error) {
				called = true
				return &HookResult{
					Modified: true,
					Payload:  p,
				}, nil
			},
		},
	})

	resp, err := s.OnHook(context.Background(), &pluginv1.HookRequest{
		Checkpoint: "post-load",
		Payload: &pluginv1.HookPayload{
			Metadata: map[string]string{"key": "val"},
		},
	})
	if err != nil {
		t.Fatalf("OnHook failed: %v", err)
	}
	if !called {
		t.Fatal("hook callback not called")
	}
	if !resp.GetModified() {
		t.Fatal("expected modified=true")
	}
}

func TestGRPCServer_OnHook_NotSubscribed(t *testing.T) {
	s := newTestServer(&PluginOpts{
		Hooks: map[string]HookFunc{},
	})

	resp, err := s.OnHook(context.Background(), &pluginv1.HookRequest{
		Checkpoint: "unknown-hook",
	})
	if err != nil {
		t.Fatalf("OnHook failed: %v", err)
	}
	if resp.GetModified() {
		t.Fatal("expected modified=false for unsubscribed hook")
	}
}

func TestGRPCServer_Shutdown(t *testing.T) {
	s := newTestServer(&PluginOpts{})
	_, err := s.Shutdown(context.Background(), &pluginv1.ShutdownRequest{})
	if err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// RenderComponent tests
// ---------------------------------------------------------------------------

func TestGRPCServer_RenderComponent(t *testing.T) {
	s := newTestServer(&PluginOpts{
		Name: "test",
		RenderComponent: func(ctx context.Context, req *RenderRequest) (*RenderResponse, error) {
			if req.Kind != "TestChart" {
				t.Fatalf("got kind %q, want TestChart", req.Kind)
			}
			if req.Name != "my-chart" {
				t.Fatalf("got name %q, want my-chart", req.Name)
			}
			if req.RenderMode != "build" {
				t.Fatalf("got render_mode %q, want build", req.RenderMode)
			}
			return &RenderResponse{
				HTML: "<bn-test-chart></bn-test-chart>",
				Diagnostics: []Diagnostic{
					{Message: "chart warning", Severity: Warning},
				},
			}, nil
		},
	})

	resp, err := s.RenderComponent(context.Background(), &pluginv1.RenderComponentRequest{
		Kind:       "TestChart",
		Name:       "my-chart",
		Spec:       []byte(`{"type":"bar"}`),
		RenderMode: "build",
	})
	if err != nil {
		t.Fatalf("RenderComponent failed: %v", err)
	}
	if resp.GetHtml() != "<bn-test-chart></bn-test-chart>" {
		t.Fatalf("got html %q", resp.GetHtml())
	}
	if len(resp.GetDiagnostics()) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(resp.GetDiagnostics()))
	}
	if resp.GetDiagnostics()[0].GetMessage() != "chart warning" {
		t.Fatalf("got diagnostic %q", resp.GetDiagnostics()[0].GetMessage())
	}
}

func TestGRPCServer_RenderComponent_NotImplemented(t *testing.T) {
	s := newTestServer(&PluginOpts{Name: "test"})

	_, err := s.RenderComponent(context.Background(), &pluginv1.RenderComponentRequest{
		Kind: "Unknown",
	})
	if err == nil {
		t.Fatal("expected error when RenderComponent not implemented")
	}
}

// ---------------------------------------------------------------------------
// ExecCommand tests
// ---------------------------------------------------------------------------

// mockExecStream captures ExecCommandOutput messages sent via a streaming RPC.
type mockExecStream struct {
	pluginv1.BinoPlugin_ExecCommandServer
	outputs []*pluginv1.ExecCommandOutput
	ctx     context.Context
}

func (m *mockExecStream) Send(out *pluginv1.ExecCommandOutput) error {
	m.outputs = append(m.outputs, out)
	return nil
}

func (m *mockExecStream) Context() context.Context {
	return m.ctx
}

func TestGRPCServer_ExecCommand_Run(t *testing.T) {
	s := newTestServer(&PluginOpts{
		Commands: []Command{
			{
				Name:  "hello",
				Short: "Say hello",
				Run: func(ctx context.Context, args []string, flags map[string]string) error {
					// Write to stdout which is piped through the stream.
					fmt.Print("hello world")
					return nil
				},
			},
		},
	})

	stream := &mockExecStream{ctx: context.Background()}
	err := s.ExecCommand(&pluginv1.ExecCommandRequest{
		Command: "hello",
	}, stream)
	if err != nil {
		t.Fatalf("ExecCommand failed: %v", err)
	}

	// Find the final message.
	var final *pluginv1.ExecCommandOutput
	for _, o := range stream.outputs {
		if o.GetIsFinal() {
			final = o
		}
	}
	if final == nil {
		t.Fatal("no final message received")
	}
	if final.GetExitCode() != 0 {
		t.Fatalf("expected exit code 0, got %d", final.GetExitCode())
	}
}

func TestGRPCServer_ExecCommand_RunWithHost(t *testing.T) {
	hostCalled := false
	s := newTestServer(&PluginOpts{
		Commands: []Command{
			{
				Name:  "query",
				Short: "Run a query",
				RunWithHost: func(ctx context.Context, args []string, flags map[string]string, host HostClient) error {
					hostCalled = true
					if host != nil {
						t.Fatal("expected nil host in test (no broker)")
					}
					return nil
				},
			},
		},
	})

	stream := &mockExecStream{ctx: context.Background()}
	err := s.ExecCommand(&pluginv1.ExecCommandRequest{
		Command: "query",
	}, stream)
	if err != nil {
		t.Fatalf("ExecCommand failed: %v", err)
	}
	if !hostCalled {
		t.Fatal("RunWithHost callback not called")
	}
}

func TestGRPCServer_ExecCommand_UnknownCommand(t *testing.T) {
	s := newTestServer(&PluginOpts{})

	stream := &mockExecStream{ctx: context.Background()}
	err := s.ExecCommand(&pluginv1.ExecCommandRequest{
		Command: "nonexistent",
	}, stream)
	if err != nil {
		t.Fatalf("ExecCommand failed: %v", err)
	}

	if len(stream.outputs) != 1 {
		t.Fatalf("expected 1 output, got %d", len(stream.outputs))
	}
	if stream.outputs[0].GetExitCode() != 1 {
		t.Fatalf("expected exit code 1, got %d", stream.outputs[0].GetExitCode())
	}
	if !stream.outputs[0].GetIsFinal() {
		t.Fatal("expected final message")
	}
}

// ---------------------------------------------------------------------------
// GetAssets callback tests
// ---------------------------------------------------------------------------

func TestGRPCServer_GetAssets_Callback(t *testing.T) {
	s := newTestServer(&PluginOpts{
		GetAssets: func(ctx context.Context, renderMode string) ([]Asset, error) {
			if renderMode == "build" {
				return []Asset{{URLPath: "/build.js"}}, nil
			}
			return []Asset{{URLPath: "/preview.js"}, {URLPath: "/debug.css"}}, nil
		},
	})

	// Build mode.
	resp, err := s.GetAssets(context.Background(), &pluginv1.GetAssetsRequest{RenderMode: "build"})
	if err != nil {
		t.Fatalf("GetAssets(build) failed: %v", err)
	}
	if len(resp.GetScripts()) != 1 || resp.GetScripts()[0].GetUrlPath() != "/build.js" {
		t.Fatalf("build: expected 1 script /build.js, got %v", resp.GetScripts())
	}

	// Preview mode.
	resp, err = s.GetAssets(context.Background(), &pluginv1.GetAssetsRequest{RenderMode: "preview"})
	if err != nil {
		t.Fatalf("GetAssets(preview) failed: %v", err)
	}
	if len(resp.GetScripts()) != 1 {
		t.Fatalf("preview: expected 1 script, got %d", len(resp.GetScripts()))
	}
	if len(resp.GetStyles()) != 1 {
		t.Fatalf("preview: expected 1 style, got %d", len(resp.GetStyles()))
	}
}

// ---------------------------------------------------------------------------
// Lint with CheckWithContext tests
// ---------------------------------------------------------------------------

func TestGRPCServer_Lint_CheckWithContext(t *testing.T) {
	s := newTestServer(&PluginOpts{
		LintRules: []LintRule{
			{
				ID: "test/context-check",
				CheckWithContext: func(ctx context.Context, docs []Document, lintCtx *LintContext) []Finding {
					if !lintCtx.DatasetsAvailable {
						return nil
					}
					return []Finding{{Message: "found " + lintCtx.Datasets[0].Name}}
				},
			},
		},
	})

	resp, err := s.Lint(context.Background(), &pluginv1.LintRequest{
		Documents:         []*pluginv1.LintDocument{{Name: "doc1"}},
		DatasetsAvailable: true,
		Datasets: []*pluginv1.DatasetPayload{
			{Name: "sales", JsonRows: []byte(`[{"x":1}]`)},
		},
	})
	if err != nil {
		t.Fatalf("Lint failed: %v", err)
	}
	if len(resp.GetFindings()) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(resp.GetFindings()))
	}
	if resp.GetFindings()[0].GetMessage() != "found sales" {
		t.Fatalf("got message %q", resp.GetFindings()[0].GetMessage())
	}
}

func TestIsScript(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/plugins/test/chart.js", true},
		{"/plugins/test/module.mjs", true},
		{"/plugins/test/style.css", false},
		{"/plugins/test/CHART.JS", true},
		{"", false},
	}
	for _, tt := range tests {
		if got := isScript(tt.path); got != tt.want {
			t.Errorf("isScript(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}
