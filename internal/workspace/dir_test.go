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
