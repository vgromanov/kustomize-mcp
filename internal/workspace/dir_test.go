package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestFileURIPath(t *testing.T) {
	p, err := fileURIPath("file:///Users/foo/bar/project")
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(p) || !strings.Contains(p, "project") {
		t.Fatalf("got %q", p)
	}
}

func TestFileURIPath_errors(t *testing.T) {
	for _, u := range []string{"", "https://x", "file://"} {
		if _, err := fileURIPath(u); err == nil {
			t.Fatalf("expected error for %q", u)
		}
	}
}

func TestPickRootFromRoots(t *testing.T) {
	want := "/tmp/ws"
	p, ok := pickRootFromRoots([]*mcp.Root{
		{URI: "https://bad"},
		nil,
		{URI: "file://" + want},
	})
	if !ok || p != filepath.Clean(want) {
		t.Fatalf("got %q ok=%v", p, ok)
	}
	if _, ok := pickRootFromRoots(nil); ok {
		t.Fatal("expected false for nil roots")
	}
	if _, ok := pickRootFromRoots([]*mcp.Root{{URI: "https://x"}}); ok {
		t.Fatal("expected false when no file roots")
	}
}

func TestDir_KUSTOMIZE_MCP_ROOT(t *testing.T) {
	tmp := t.TempDir()
	sub := filepath.Join(tmp, "ws")
	if err := os.MkdirAll(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KUSTOMIZE_MCP_ROOT", sub)
	t.Chdir(tmp)

	got, err := Dir(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(got) != filepath.Clean(sub) {
		t.Fatalf("got %q want %q", got, sub)
	}
}

func TestDir_fallbackGetwd(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)
	t.Setenv("KUSTOMIZE_MCP_ROOT", "")

	got, err := Dir(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	wd, _ := os.Getwd()
	if filepath.Clean(got) != filepath.Clean(wd) {
		t.Fatalf("got %q want getwd %q", got, wd)
	}
}

type fakeRootSession struct {
	roots []*mcp.Root
	err   error
}

func (f *fakeRootSession) ListRoots(ctx context.Context, _ *mcp.ListRootsParams) (*mcp.ListRootsResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &mcp.ListRootsResult{Roots: f.roots}, nil
}

func TestDir_ListRoots_firstFileURI(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("KUSTOMIZE_MCP_ROOT", "")
	t.Chdir("/")

	uri := "file://" + filepath.ToSlash(tmp)
	got, err := Dir(context.Background(), &fakeRootSession{
		roots: []*mcp.Root{{URI: "https://ignore"}, {URI: uri}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(got) != filepath.Clean(tmp) {
		t.Fatalf("got %q want %q", got, tmp)
	}
}

func TestDir_ListRoots_errorFallsBackToGetwd(t *testing.T) {
	wd := t.TempDir()
	t.Chdir(wd)
	t.Setenv("KUSTOMIZE_MCP_ROOT", "")

	got, err := Dir(context.Background(), &fakeRootSession{err: os.ErrPermission})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(got) != filepath.Clean(wd) {
		t.Fatalf("got %q want getwd %q", got, wd)
	}
}

func TestDir_ListRoots_noUsableFileRootFallsBackToGetwd(t *testing.T) {
	wd := t.TempDir()
	t.Chdir(wd)
	t.Setenv("KUSTOMIZE_MCP_ROOT", "")

	got, err := Dir(context.Background(), &fakeRootSession{
		roots: []*mcp.Root{{URI: "https://example.com/x"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(got) != filepath.Clean(wd) {
		t.Fatalf("got %q want getwd %q", got, wd)
	}
}

func TestAllRoots_env(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("KUSTOMIZE_MCP_ROOT", tmp)

	roots, err := AllRoots(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 1 || filepath.Clean(roots[0]) != filepath.Clean(tmp) {
		t.Fatalf("got %v", roots)
	}
}

func TestAllRoots_multipleSessionRoots(t *testing.T) {
	t.Setenv("KUSTOMIZE_MCP_ROOT", "")
	dirA := t.TempDir()
	dirB := t.TempDir()

	sess := &fakeRootSession{
		roots: []*mcp.Root{
			{URI: "file://" + filepath.ToSlash(dirA)},
			{URI: "file://" + filepath.ToSlash(dirB)},
		},
	}
	roots, err := AllRoots(context.Background(), sess)
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 2 {
		t.Fatalf("expected 2 roots, got %v", roots)
	}
	if filepath.Clean(roots[0]) != filepath.Clean(dirA) {
		t.Fatalf("roots[0] = %q, want %q", roots[0], dirA)
	}
	if filepath.Clean(roots[1]) != filepath.Clean(dirB) {
		t.Fatalf("roots[1] = %q, want %q", roots[1], dirB)
	}
}

func TestAllRoots_fallbackGetwd(t *testing.T) {
	wd := t.TempDir()
	t.Chdir(wd)
	t.Setenv("KUSTOMIZE_MCP_ROOT", "")

	roots, err := AllRoots(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 1 {
		t.Fatalf("expected 1 root, got %v", roots)
	}
	got, _ := os.Getwd()
	if filepath.Clean(roots[0]) != filepath.Clean(got) {
		t.Fatalf("got %q want %q", roots[0], got)
	}
}

func TestResolveProject_subdirectoryMatch(t *testing.T) {
	t.Setenv("KUSTOMIZE_MCP_ROOT", "")
	root := t.TempDir()
	projDir := filepath.Join(root, "project-a")
	if err := os.MkdirAll(projDir, 0o700); err != nil {
		t.Fatal(err)
	}

	sess := &fakeRootSession{
		roots: []*mcp.Root{{URI: "file://" + filepath.ToSlash(root)}},
	}
	got, err := ResolveProject(context.Background(), sess, "project-a")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(got) != filepath.Clean(projDir) {
		t.Fatalf("got %q want %q", got, projDir)
	}
}

func TestResolveProject_suffixMatch(t *testing.T) {
	t.Setenv("KUSTOMIZE_MCP_ROOT", "")
	rootA := t.TempDir()
	rootB := filepath.Join(t.TempDir(), "infra", "clusters-universal")
	if err := os.MkdirAll(rootB, 0o700); err != nil {
		t.Fatal(err)
	}

	sess := &fakeRootSession{
		roots: []*mcp.Root{
			{URI: "file://" + filepath.ToSlash(rootA)},
			{URI: "file://" + filepath.ToSlash(rootB)},
		},
	}
	got, err := ResolveProject(context.Background(), sess, "infra/clusters-universal")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(got) != filepath.Clean(rootB) {
		t.Fatalf("got %q want %q", got, rootB)
	}
}

func TestResolveProject_suffixMatchSingleSegment(t *testing.T) {
	t.Setenv("KUSTOMIZE_MCP_ROOT", "")
	rootA := t.TempDir()
	rootB := filepath.Join(t.TempDir(), "clusters-universal")
	if err := os.MkdirAll(rootB, 0o700); err != nil {
		t.Fatal(err)
	}

	sess := &fakeRootSession{
		roots: []*mcp.Root{
			{URI: "file://" + filepath.ToSlash(rootA)},
			{URI: "file://" + filepath.ToSlash(rootB)},
		},
	}
	got, err := ResolveProject(context.Background(), sess, "clusters-universal")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(got) != filepath.Clean(rootB) {
		t.Fatalf("got %q want %q", got, rootB)
	}
}

func TestResolveProject_subdirectoryPreferredOverSuffix(t *testing.T) {
	t.Setenv("KUSTOMIZE_MCP_ROOT", "")
	rootA := t.TempDir()
	subDir := filepath.Join(rootA, "myproj")
	if err := os.MkdirAll(subDir, 0o700); err != nil {
		t.Fatal(err)
	}
	rootB := filepath.Join(t.TempDir(), "myproj")
	if err := os.MkdirAll(rootB, 0o700); err != nil {
		t.Fatal(err)
	}

	sess := &fakeRootSession{
		roots: []*mcp.Root{
			{URI: "file://" + filepath.ToSlash(rootA)},
			{URI: "file://" + filepath.ToSlash(rootB)},
		},
	}
	got, err := ResolveProject(context.Background(), sess, "myproj")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(got) != filepath.Clean(subDir) {
		t.Fatalf("subdirectory should win: got %q want %q", got, subDir)
	}
}

func TestResolveProject_fallback(t *testing.T) {
	t.Setenv("KUSTOMIZE_MCP_ROOT", "")
	root := t.TempDir()
	sess := &fakeRootSession{
		roots: []*mcp.Root{{URI: "file://" + filepath.ToSlash(root)}},
	}

	got, err := ResolveProject(context.Background(), sess, "does-not-exist")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "does-not-exist")
	if filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("fallback: got %q want %q", got, want)
	}
}
