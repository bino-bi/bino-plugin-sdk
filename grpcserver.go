package pluginsdk

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	goplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	pluginv1 "github.com/bino-bi/bino-plugin-sdk/proto/v1"
)

// binoPlugin implements go-plugin's GRPCPlugin interface for the server side.
type binoPlugin struct {
	goplugin.Plugin
	opts *PluginOpts
}

func (p *binoPlugin) GRPCServer(broker *goplugin.GRPCBroker, s *grpc.Server) error {
	pluginv1.RegisterBinoPluginServer(s, &grpcServer{opts: p.opts, broker: broker})
	return nil
}

func (p *binoPlugin) GRPCClient(_ context.Context, _ *goplugin.GRPCBroker, _ *grpc.ClientConn) (any, error) {
	// SDK doesn't use the client side.
	return nil, nil
}

// grpcServer implements the BinoPlugin gRPC service.
type grpcServer struct {
	pluginv1.UnimplementedBinoPluginServer
	opts        *PluginOpts
	config      map[string]string
	projectRoot string
	binoVersion string
	broker      *goplugin.GRPCBroker
	hostClient  HostClient // set during Init if host service is available
}

func (s *grpcServer) Init(_ context.Context, req *pluginv1.InitRequest) (*pluginv1.PluginManifest, error) {
	s.config = req.GetConfig()
	s.projectRoot = req.GetProjectRoot()
	s.binoVersion = req.GetBinoVersion()

	// Dial the host service if a broker ID was provided.
	if hostID := req.GetHostServiceId(); hostID != 0 && s.broker != nil {
		conn, err := s.broker.Dial(hostID)
		if err != nil {
			// Non-fatal: plugin can work without host service.
			_, _ = fmt.Fprintf(os.Stderr, "warning: could not connect to host service: %v\n", err)
		} else {
			s.hostClient = &grpcHostClient{client: pluginv1.NewBinoHostClient(conn)}
		}
	}

	manifest := &pluginv1.PluginManifest{
		Name:             s.opts.Name,
		Version:          s.opts.Version,
		Description:      s.opts.Description,
		DuckdbExtensions: s.opts.DuckDBExtensions,
		ProvidesLinter:   len(s.opts.LintRules) > 0,
		ProvidesAssets:   len(s.opts.Assets) > 0 || s.opts.GetAssets != nil,
	}

	for _, k := range s.opts.Kinds {
		manifest.Kinds = append(manifest.Kinds, &pluginv1.KindRegistration{
			KindName:       k.Name,
			Category:       pluginv1.KindCategory(k.Category),
			DatasourceType: k.DataSourceType,
		})
	}

	for _, cmd := range s.opts.Commands {
		manifest.Commands = append(manifest.Commands, commandToProto(cmd))
	}

	for checkpoint := range s.opts.Hooks {
		manifest.Hooks = append(manifest.Hooks, checkpoint)
	}

	return manifest, nil
}

func (s *grpcServer) Shutdown(context.Context, *pluginv1.ShutdownRequest) (*pluginv1.ShutdownResponse, error) {
	return &pluginv1.ShutdownResponse{}, nil
}

func (s *grpcServer) GetSchemas(context.Context, *pluginv1.GetSchemasRequest) (*pluginv1.GetSchemasResponse, error) {
	schemas := make(map[string][]byte)
	for _, k := range s.opts.Kinds {
		if len(k.Schema) > 0 {
			schemas[k.Name] = k.Schema
		}
	}
	return &pluginv1.GetSchemasResponse{Schemas: schemas}, nil
}

func (s *grpcServer) CollectDataSource(ctx context.Context, req *pluginv1.CollectDataSourceRequest) (*pluginv1.CollectDataSourceResponse, error) {
	if s.opts.CollectDataSource == nil {
		return nil, fmt.Errorf("plugin %q does not implement CollectDataSource", s.opts.Name)
	}

	resp, err := s.opts.CollectDataSource(ctx, &CollectRequest{
		Name:        req.GetName(),
		Spec:        req.GetRawSpec(),
		Env:         req.GetEnv(),
		ProjectRoot: req.GetProjectRoot(),
		Host:        s.hostClient,
	})
	if err != nil {
		return nil, err
	}

	return &pluginv1.CollectDataSourceResponse{
		JsonRows:         resp.JSONRows,
		ColumnTypes:      resp.ColumnTypes,
		Ephemeral:        resp.Ephemeral,
		Diagnostics:      diagnosticsToProto(resp.Diagnostics),
		DuckdbExpression: resp.DuckDBExpression,
	}, nil
}

func (s *grpcServer) Lint(ctx context.Context, req *pluginv1.LintRequest) (*pluginv1.LintResponse, error) {
	docs := documentsFromProto(req.GetDocuments())

	// Build enriched lint context from the request.
	lintCtx := &LintContext{
		DatasetsAvailable: req.GetDatasetsAvailable(),
		RenderedHTML:      req.GetRenderedHtml(),
		Host:              s.hostClient,
	}
	for _, ds := range req.GetDatasets() {
		lintCtx.Datasets = append(lintCtx.Datasets, Dataset{
			Name:     ds.GetName(),
			JSONRows: ds.GetJsonRows(),
			Columns:  ds.GetColumns(),
		})
	}

	var findings []*pluginv1.LintFinding
	for _, rule := range s.opts.LintRules {
		var results []Finding
		if rule.CheckWithContext != nil {
			results = rule.CheckWithContext(ctx, docs, lintCtx)
		} else if rule.Check != nil {
			results = rule.Check(ctx, docs)
		}
		for _, f := range results {
			findings = append(findings, findingToProto(f, rule.ID))
		}
	}

	return &pluginv1.LintResponse{Findings: findings}, nil
}

func (s *grpcServer) GetAssets(ctx context.Context, req *pluginv1.GetAssetsRequest) (*pluginv1.GetAssetsResponse, error) {
	var assets []Asset
	if s.opts.GetAssets != nil {
		var err error
		assets, err = s.opts.GetAssets(ctx, req.GetRenderMode())
		if err != nil {
			return nil, err
		}
	} else {
		assets = s.opts.Assets
	}

	resp := &pluginv1.GetAssetsResponse{}
	for _, a := range assets {
		asset := assetToProto(a)
		if isScript(a.URLPath) {
			resp.Scripts = append(resp.Scripts, asset)
		} else {
			resp.Styles = append(resp.Styles, asset)
		}
	}
	return resp, nil
}

func (s *grpcServer) ListCommands(context.Context, *pluginv1.ListCommandsRequest) (*pluginv1.ListCommandsResponse, error) {
	resp := &pluginv1.ListCommandsResponse{}
	for _, cmd := range s.opts.Commands {
		resp.Commands = append(resp.Commands, commandToProto(cmd))
	}
	return resp, nil
}

func (s *grpcServer) ExecCommand(req *pluginv1.ExecCommandRequest, stream pluginv1.BinoPlugin_ExecCommandServer) error {
	var cmd *Command
	for i := range s.opts.Commands {
		if s.opts.Commands[i].Name == req.GetCommand() {
			cmd = &s.opts.Commands[i]
			break
		}
	}
	if cmd == nil {
		return stream.Send(&pluginv1.ExecCommandOutput{
			Stderr:   []byte(fmt.Sprintf("unknown command: %s\n", req.GetCommand())),
			ExitCode: 1,
			IsFinal:  true,
		})
	}

	// Change to the workdir if specified.
	if wd := req.GetWorkdir(); wd != "" {
		if err := os.Chdir(wd); err != nil {
			return stream.Send(&pluginv1.ExecCommandOutput{
				Stderr:   []byte(fmt.Sprintf("chdir %s: %v\n", wd, err)),
				ExitCode: 1,
				IsFinal:  true,
			})
		}
	}

	// Redirect os.Stdout/Stderr to pipes so fmt.Print* output from the
	// command callback is captured and streamed through gRPC to the host.
	origStdout, origStderr := os.Stdout, os.Stderr
	stdoutR, stdoutW, _ := os.Pipe()
	stderrR, stderrW, _ := os.Pipe()
	os.Stdout = stdoutW
	os.Stderr = stderrW

	// Stream pipe output to host using a WaitGroup for proper synchronization.
	var pipeWg sync.WaitGroup
	pipeWg.Add(2)
	go func() {
		defer pipeWg.Done()
		buf := make([]byte, 4096)
		for {
			n, err := stdoutR.Read(buf)
			if n > 0 {
				_ = stream.Send(&pluginv1.ExecCommandOutput{
					Stdout: append([]byte(nil), buf[:n]...),
				})
			}
			if err != nil {
				return
			}
		}
	}()
	go func() {
		defer pipeWg.Done()
		buf := make([]byte, 4096)
		for {
			n, err := stderrR.Read(buf)
			if n > 0 {
				_ = stream.Send(&pluginv1.ExecCommandOutput{
					Stderr: append([]byte(nil), buf[:n]...),
				})
			}
			if err != nil {
				return
			}
		}
	}()

	// Run the command (prefer RunWithHost if available).
	var cmdErr error
	if cmd.RunWithHost != nil {
		cmdErr = cmd.RunWithHost(stream.Context(), req.GetArgs(), req.GetFlags(), s.hostClient)
	} else {
		cmdErr = cmd.Run(stream.Context(), req.GetArgs(), req.GetFlags())
	}

	// Restore and close pipes so the reader goroutines exit.
	os.Stdout = origStdout
	os.Stderr = origStderr
	_ = stdoutW.Close()
	_ = stderrW.Close()
	pipeWg.Wait() // wait for both stdout and stderr readers to finish

	exitCode := int32(0)
	if cmdErr != nil {
		_ = stream.Send(&pluginv1.ExecCommandOutput{
			Stderr: []byte(cmdErr.Error() + "\n"),
		})
		exitCode = 1
	}

	return stream.Send(&pluginv1.ExecCommandOutput{
		ExitCode: exitCode,
		IsFinal:  true,
	})
}

func (s *grpcServer) OnHook(ctx context.Context, req *pluginv1.HookRequest) (*pluginv1.HookResponse, error) {
	hookFunc, ok := s.opts.Hooks[req.GetCheckpoint()]
	if !ok {
		return &pluginv1.HookResponse{Modified: false}, nil
	}

	payload := hookPayloadFromProto(req.GetPayload())
	payload.Host = s.hostClient // Make host available to hook callbacks.
	result, err := hookFunc(ctx, payload)
	if err != nil {
		return nil, err
	}

	resp := &pluginv1.HookResponse{
		Modified:    result.Modified,
		Payload:     hookPayloadToProto(result.Payload),
		Diagnostics: diagnosticsToProto(result.Diagnostics),
	}
	for _, f := range result.Findings {
		resp.Findings = append(resp.Findings, findingToProto(f, f.RuleID))
	}
	return resp, nil
}

func (s *grpcServer) RenderComponent(ctx context.Context, req *pluginv1.RenderComponentRequest) (*pluginv1.RenderComponentResponse, error) {
	if s.opts.RenderComponent == nil {
		return nil, fmt.Errorf("plugin %q does not implement RenderComponent", s.opts.Name)
	}

	resp, err := s.opts.RenderComponent(ctx, &RenderRequest{
		Kind:       req.GetKind(),
		Name:       req.GetName(),
		Spec:       req.GetSpec(),
		RenderMode: req.GetRenderMode(),
		Host:       s.hostClient,
	})
	if err != nil {
		return nil, err
	}

	return &pluginv1.RenderComponentResponse{
		Html:        resp.HTML,
		Diagnostics: diagnosticsToProto(resp.Diagnostics),
	}, nil
}

// isScript returns true if the URL path looks like a JavaScript file.
func isScript(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".js") || strings.HasSuffix(lower, ".mjs")
}
