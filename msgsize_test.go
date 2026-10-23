package pluginsdk

import (
	"bytes"
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	pluginv1 "github.com/bino-bi/bino-plugin-sdk/proto/v1"
)

// sixMiB is above gRPC's 4 MiB receive default.
const sixMiB = 6 << 20

// bufconnDial serves srv on an in-memory listener and returns a client
// connection to it.
func bufconnDial(t *testing.T, srv *grpc.Server) *grpc.ClientConn {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
	}
	conn, err := grpc.NewClient("passthrough:///bufnet", opts...)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func TestMessageSize_PluginReceivesLargeHookRequest(t *testing.T) {
	var gotLen int
	srv := newGRPCServer(nil)
	pluginv1.RegisterBinoPluginServer(srv, &grpcServer{opts: &PluginOpts{
		Hooks: map[string]HookFunc{
			"post-render-html": func(ctx context.Context, p *HookPayload) (*HookResult, error) {
				gotLen = len(p.HTML)
				return &HookResult{}, nil
			},
		},
	}})
	client := pluginv1.NewBinoPluginClient(bufconnDial(t, srv))

	_, err := client.OnHook(context.Background(), &pluginv1.HookRequest{
		Checkpoint: "post-render-html",
		Payload:    &pluginv1.HookPayload{Html: bytes.Repeat([]byte("x"), sixMiB)},
	})
	if err != nil {
		t.Fatalf("OnHook with a 6 MiB payload failed: %v", err)
	}
	if gotLen != sixMiB {
		t.Fatalf("hook got %d bytes, want %d", gotLen, sixMiB)
	}
}
