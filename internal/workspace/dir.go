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

// AllRoots returns all available workspace roots in priority order.
// When KUSTOMIZE_MCP_ROOT is set, it is the only root returned.
// Otherwise, all file:// roots from the MCP session are returned.
// Falls back to os.Getwd() if no other roots are available.
func AllRoots(ctx context.Context, sess RootSession) ([]string, error) {
	if v := strings.TrimSpace(os.Getenv("KUSTOMIZE_MCP_ROOT")); v != "" {
		abs, err := filepath.Abs(v)
		if err != nil {
			return nil, err
		}
		return []string{abs}, nil
	}
	if sess != nil {
		res, err := sess.ListRoots(ctx, nil)
		if err == nil && res != nil {
			var roots []string
			for _, r := range res.Roots {
				if r == nil {
					continue
				}
				p, err := fileURIPath(r.URI)
				if err == nil && p != "" {
					roots = append(roots, filepath.Clean(p))
				}
			}
			if len(roots) > 0 {
				return roots, nil
			}
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return []string{cwd}, nil
}

// ResolveProject finds the effective workspace root for a project.
//
// If project is an absolute path, it must be equal to or a subdirectory of one of
// the known workspace roots (from AllRoots); the path must exist and be a directory.
//
// Otherwise (relative path), resolution order is:
//  1. project as a subdirectory of each available root (first existing dir wins)
//  2. a root whose path ends with the project segments (multi-root workspace match)
//  3. falls back to primaryRoot/project (may not exist; lets the caller surface the error)
func ResolveProject(ctx context.Context, sess RootSession, project string) (string, error) {
	roots, err := AllRoots(ctx, sess)
	if err != nil {
		return "", err
	}

	project = strings.TrimSpace(project)
	if project == "" {
		return "", fmt.Errorf("project path must be non-empty")
	}

	if filepath.IsAbs(project) {
		abs := filepath.Clean(project)
		if !pathWithinKnownRoots(abs, roots) {
			return "", fmt.Errorf("project absolute path must be equal to or inside an MCP workspace root")
		}
		st, err := os.Stat(abs)
		if err != nil {
			return "", err
		}
		if !st.IsDir() {
			return "", fmt.Errorf("project path must be a directory")
		}
		return abs, nil
	}

	p := filepath.FromSlash(project)

	for _, root := range roots {
		candidate := filepath.Join(root, p)
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate, nil
		}
	}

	projSlash := filepath.ToSlash(project)
	for _, root := range roots {
		rootSlash := filepath.ToSlash(root)
		if strings.HasSuffix(rootSlash, "/"+projSlash) {
			return root, nil
		}
	}

	return filepath.Join(roots[0], p), nil
}

// pathWithinKnownRoots reports whether target is equal to root or strictly inside it.
func pathWithinKnownRoots(target string, roots []string) bool {
	target = filepath.Clean(target)
	for _, root := range roots {
		root = filepath.Clean(root)
		rel, err := filepath.Rel(root, target)
		if err != nil {
			continue
		}
		if rel == "." {
			return true
		}
		if !strings.HasPrefix(rel, "..") {
			return true
		}
	}
	return false
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
