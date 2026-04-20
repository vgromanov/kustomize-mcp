package diff

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vgromanov/kustomize-mcp/internal/manifest"
)

func TestDir_addedDeletedModified(t *testing.T) {
	workspace := t.TempDir()
	left := filepath.Join(workspace, "left")
	right := filepath.Join(workspace, "right")
	work := filepath.Join(workspace, "work")

	metaName := manifest.Metadata{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		Name:       "a",
	}.ToFilename()
	sharedName := manifest.Metadata{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		Name:       "shared",
	}.ToFilename()

	if err := os.MkdirAll(filepath.Join(left, "app"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(right, "app"), 0o700); err != nil {
		t.Fatal(err)
	}
	// left: only + shared old
	if err := os.WriteFile(filepath.Join(left, "app", metaName), []byte("a: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(left, "app", sharedName), []byte("x: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// right: shared new + added only-on-right
	onlyRight := manifest.Metadata{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		Name:       "b",
	}.ToFilename()
	if err := os.WriteFile(filepath.Join(right, "app", sharedName), []byte("x: 2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(right, "app", onlyRight), []byte("b: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	res, err := Dir(left, right, work, workspace)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Added) != 1 || res.Added[0].Name != "b" {
		t.Fatalf("Added: %+v", res.Added)
	}
	if len(res.Deleted) != 1 || res.Deleted[0].Name != "a" {
		t.Fatalf("Deleted: %+v", res.Deleted)
	}
	if len(res.Modified) != 1 || res.Modified[0].Name != "shared" {
		t.Fatalf("Modified: %+v", res.Modified)
	}
	if res.DiffPath == nil || !strings.HasSuffix(*res.DiffPath, "changes.diff") {
		t.Fatalf("DiffPath: %v", res.DiffPath)
	}
	b, err := os.ReadFile(filepath.Join(work, "changes.diff"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "shared") {
		t.Fatalf("diff file missing expected content: %s", b)
	}
}

func TestDir_identicalTrees(t *testing.T) {
	workspace := t.TempDir()
	left := filepath.Join(workspace, "l")
	right := filepath.Join(workspace, "r")
	work := filepath.Join(workspace, "w")
	name := manifest.Metadata{APIVersion: "v1", Kind: "ConfigMap", Name: "x"}.ToFilename()
	content := []byte("k: v\n")
	for _, d := range []string{left, right} {
		if err := os.MkdirAll(filepath.Join(d, "p"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "p", name), content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	res, err := Dir(left, right, work, workspace)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Added)+len(res.Deleted)+len(res.Modified) != 0 {
		t.Fatalf("expected no changes, got %+v", res)
	}
}

func TestDir_diffPathAbsoluteWhenRelFails(t *testing.T) {
	left := t.TempDir()
	right := t.TempDir()
	work := t.TempDir()
	name := manifest.Metadata{APIVersion: "v1", Kind: "ConfigMap", Name: "x"}.ToFilename()
	content := []byte("k: v\n")
	for _, d := range []string{left, right} {
		if err := os.MkdirAll(filepath.Join(d, "p"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "p", name), content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// Empty workspace root makes filepath.Rel fail; Dir must record an absolute diff path.
	res, err := Dir(left, right, work, "")
	if err != nil {
		t.Fatal(err)
	}
	if res.DiffPath == nil || !filepath.IsAbs(*res.DiffPath) {
		t.Fatalf("want absolute DiffPath, got %v", res.DiffPath)
	}
}
