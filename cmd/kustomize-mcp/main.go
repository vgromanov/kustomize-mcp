// Command kustomize-mcp is an MCP server for Kustomize render, diff, and dependency tools.
// It mirrors the tools from https://github.com/mbrt/kustomize-mcp using Go libraries (krusty)
// instead of shelling out to the kustomize or git CLIs. Unified directory diffs are pure Go.
//
// Helm: the upstream project uses `kustomize build --enable-helm`. Kustomize's Helm builtin
// may run the helm binary when enabled. Set KUSTOMIZE_ENABLE_HELM=1 to turn that on.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/vgromanov/kustomize-mcp/internal/mcpapp"
	"github.com/vgromanov/kustomize-mcp/internal/version"
)

func main() {
	printVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *printVersion {
		fmt.Printf("%s %s\n", version.Name, version.Version)
		os.Exit(0)
	}
	if err := mcpapp.RunStdio(context.Background()); err != nil {
		log.Printf("server: %v", err)
	}
}
