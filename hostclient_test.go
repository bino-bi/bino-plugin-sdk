package pluginsdk

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/grpc"

	pluginv1 "github.com/bino-bi/bino-plugin-sdk/proto/v1"
)

// fakeHostClient answers QueryDuckDB with a fixed response.
type fakeHostClient struct {
	pluginv1.BinoHostClient
	queryResp *pluginv1.QueryResponse
}

func (f *fakeHostClient) QueryDuckDB(context.Context, *pluginv1.QueryRequest, ...grpc.CallOption) (*pluginv1.QueryResponse, error) {
	return f.queryResp, nil
}

func TestHostClient_QueryDuckDB_ErrorDiagnostic(t *testing.T) {
	c := &grpcHostClient{client: &fakeHostClient{queryResp: &pluginv1.QueryResponse{
		Diagnostics: []*pluginv1.Diagnostic{
			{Message: "slow query", Severity: pluginv1.Severity_WARNING},
			{Message: "Catalog Error: Table sales does not exist", Severity: pluginv1.Severity_ERROR},
		},
	}}}

	result, err := c.QueryDuckDB(context.Background(), "SELECT * FROM sales")
	if err == nil {
		t.Fatal("expected an error for an ERROR diagnostic")
	}
	if !strings.HasPrefix(err.Error(), "host query: ") || !strings.Contains(err.Error(), "Table sales does not exist") {
		t.Fatalf("got error %q", err)
	}
	if result == nil {
		t.Fatal("expected a result next to the error")
	}
	if len(result.Diagnostics) != 2 {
		t.Fatalf("expected 2 diagnostics kept, got %d", len(result.Diagnostics))
	}
}

func TestHostClient_QueryDuckDB_WarningOnly(t *testing.T) {
	c := &grpcHostClient{client: &fakeHostClient{queryResp: &pluginv1.QueryResponse{
		JsonRows: []byte(`[{"n":1}]`),
		Columns:  []string{"n"},
		Diagnostics: []*pluginv1.Diagnostic{
			{Message: "skipped dataset", Severity: pluginv1.Severity_WARNING},
		},
	}}}

	result, err := c.QueryDuckDB(context.Background(), "SELECT 1 AS n")
	if err != nil {
		t.Fatalf("expected no error for warnings only, got %v", err)
	}
	if string(result.JSONRows) != `[{"n":1}]` {
		t.Fatalf("got rows %q", result.JSONRows)
	}
	if len(result.Diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(result.Diagnostics))
	}
}
