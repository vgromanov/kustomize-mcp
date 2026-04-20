package mcpapp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/vgromanov/kustomize-mcp/internal/version"
)

// OptionsFromEnv reads tool flags from KUSTOMIZE_LOAD_RESTRICTIONS and KUSTOMIZE_ENABLE_HELM.
func OptionsFromEnv() Options {
	return Options{
		LoadRestrictions: ParseBoolEnv("KUSTOMIZE_LOAD_RESTRICTIONS", true),
		Helm:             ParseBoolEnv("KUSTOMIZE_ENABLE_HELM", false),
	}
}

// Run serves the MCP server on the given transport until the client disconnects or ctx ends.
func Run(ctx context.Context, t mcp.Transport, opts Options) error {
	server := mcp.NewServer(&mcp.Implementation{Name: version.Name, Version: version.Version}, &mcp.ServerOptions{
		Instructions: Instructions,
	})
	Register(server, opts)
	return server.Run(ctx, t)
}

// RunWithStdioDefaults is [Run] with options from [OptionsFromEnv] (used by tests and thin wrappers).
func RunWithStdioDefaults(ctx context.Context, t mcp.Transport) error {
	return Run(ctx, t, OptionsFromEnv())
}

// RunStdio is Run with [mcp.StdioTransport] and options from the environment.
func RunStdio(ctx context.Context) error {
	return RunWithStdioDefaults(ctx, &mcp.StdioTransport{})
}
