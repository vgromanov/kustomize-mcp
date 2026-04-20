package workspace

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RootSession is the part of an MCP server session used to resolve the workspace folder.
type RootSession interface {
	ListRoots(ctx context.Context, params *mcp.ListRootsParams) (*mcp.ListRootsResult, error)
}

// Dir returns the absolute filesystem path to use as the Kustomize workspace.
//
// Order of precedence:
//  1. KUSTOMIZE_MCP_ROOT env (explicit override for clients without roots)
//  2. First file:// root from the MCP session (Cursor / VS Code opened folder)
//  3. os.Getwd()
func Dir(ctx context.Context, sess RootSession) (string, error) {
	if v := strings.TrimSpace(os.Getenv("KUSTOMIZE_MCP_ROOT")); v != "" {
		return filepath.Abs(v)
	}
	if sess != nil {
		res, err := sess.ListRoots(ctx, nil)
		if err == nil && res != nil {
			if p, ok := pickRootFromRoots(res.Roots); ok {
				return p, nil
			}
		}
	}
	return os.Getwd()
}

// pickRootFromRoots returns the first usable file:// root path.
func pickRootFromRoots(roots []*mcp.Root) (string, bool) {
	for _, root := range roots {
		if root == nil {
			continue
		}
		p, err := fileURIPath(root.URI)
		if err == nil && p != "" {
			return filepath.Clean(p), true
		}
	}
	return "", false
}

// fileURIPath converts a file:// URI to an absolute local path (same rules as MCP SDK fileRoot).
func fileURIPath(uri string) (string, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", err
	}
	if u.Scheme != "file" {
		return "", fmt.Errorf("not a file URI")
	}
	if u.Path == "" {
		return "", fmt.Errorf("empty path")
	}
	p := filepath.Clean(filepath.FromSlash(u.Path))
	if !filepath.IsAbs(p) {
		return "", fmt.Errorf("not an absolute path")
	}
	return p, nil
}
