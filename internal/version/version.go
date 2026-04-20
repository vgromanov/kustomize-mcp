// Package version holds build metadata for the MCP server.
package version

// Name is the MCP server identifier reported in the MCP `initialize` handshake.
const Name = "kustomize-mcp"

// Version is the server semantic version. Overridden at build time via -ldflags.
var Version = "0.1.0"
