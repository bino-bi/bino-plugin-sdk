package pluginsdk

import (
	"context"
	"encoding/json"

	pluginv1 "github.com/bino-bi/bino-plugin-sdk/proto/v1"
)

// HostClient allows a plugin to call back to the bino CLI host for
// DuckDB queries, document access, and dataset retrieval.
type HostClient interface {
	// QueryDuckDB executes a SQL query against the host's DuckDB engine.
	// Returns JSON rows and column names.
	QueryDuckDB(ctx context.Context, sql string) (*QueryResult, error)

	// GetDocument returns a single document by kind and name.
	// Returns nil if not found.
	GetDocument(ctx context.Context, kind, name string) (*Document, error)

	// GetDatasetResult returns a pre-executed dataset result by name.
	// Returns nil if not found.
	GetDatasetResult(ctx context.Context, name string) (*Dataset, error)

	// ListDocuments returns all loaded documents, optionally filtered by kind.
	ListDocuments(ctx context.Context, kindFilter string) ([]Document, error)
}

// QueryResult holds the result of a DuckDB query executed on the host.
type QueryResult struct {
	JSONRows    json.RawMessage // JSON array of row objects
	Columns     []string
	Diagnostics []Diagnostic // Non-fatal warnings from the host query.
}

// grpcHostClient implements HostClient via gRPC calls to the host.
type grpcHostClient struct {
	client pluginv1.BinoHostClient
}

func (c *grpcHostClient) QueryDuckDB(ctx context.Context, sql string) (*QueryResult, error) {
	resp, err := c.client.QueryDuckDB(ctx, &pluginv1.QueryRequest{Sql: sql})
	if err != nil {
		return nil, err
	}
	return &QueryResult{
		JSONRows:    resp.GetJsonRows(),
		Columns:     resp.GetColumns(),
		Diagnostics: diagnosticsFromProto(resp.GetDiagnostics()),
	}, nil
}

func (c *grpcHostClient) GetDocument(ctx context.Context, kind, name string) (*Document, error) {
	resp, err := c.client.GetDocument(ctx, &pluginv1.GetDocumentRequest{
		Kind: kind,
		Name: name,
	})
	if err != nil {
		return nil, err
	}
	if !resp.GetFound() {
		return nil, nil
	}
	d := resp.GetDocument()
	return &Document{
		File:     d.GetFile(),
		Position: int(d.GetPosition()),
		Kind:     d.GetKind(),
		Name:     d.GetName(),
		Raw:      d.GetRaw(),
	}, nil
}

func (c *grpcHostClient) GetDatasetResult(ctx context.Context, name string) (*Dataset, error) {
	resp, err := c.client.GetDatasetResult(ctx, &pluginv1.GetDatasetResultRequest{
		Name: name,
	})
	if err != nil {
		return nil, err
	}
	if !resp.GetFound() {
		return nil, nil
	}
	ds := resp.GetDataset()
	return &Dataset{
		Name:     ds.GetName(),
		JSONRows: ds.GetJsonRows(),
		Columns:  ds.GetColumns(),
	}, nil
}

func (c *grpcHostClient) ListDocuments(ctx context.Context, kindFilter string) ([]Document, error) {
	resp, err := c.client.ListDocuments(ctx, &pluginv1.ListDocumentsRequest{
		KindFilter: kindFilter,
	})
	if err != nil {
		return nil, err
	}
	docs := make([]Document, len(resp.GetDocuments()))
	for i, d := range resp.GetDocuments() {
		docs[i] = Document{
			File:     d.GetFile(),
			Position: int(d.GetPosition()),
			Kind:     d.GetKind(),
			Name:     d.GetName(),
			Raw:      d.GetRaw(),
		}
	}
	return docs, nil
}
