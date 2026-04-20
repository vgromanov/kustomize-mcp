package mcpapp

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestRun_closesWhenClientDisconnects(t *testing.T) {
	t.Setenv("KUSTOMIZE_MCP_ROOT", t.TempDir())

	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()

	errCh := make(chan error, 1)
	t.Setenv("KUSTOMIZE_LOAD_RESTRICTIONS", "true")
	t.Setenv("KUSTOMIZE_ENABLE_HELM", "false")
	go func() {
		errCh <- RunWithStdioDefaults(ctx, st)
	}()

	client := mcp.NewClient(&mcp.Implementation{Name: "probe", Version: "v0.0.1"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := cs.Close(); err != nil {
		t.Fatal(err)
	}

	if err := <-errCh; err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
}
