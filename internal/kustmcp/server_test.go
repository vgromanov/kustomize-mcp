package kustmcp

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestServer_checkpointRenderDiff(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "envs", "a")
	b := filepath.Join(root, "envs", "b")
	for _, dir := range []string{a, b} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "kustomization.yaml"), []byte(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- cm.yaml
`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(a, "cm.yaml"), []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: cfg
data:
  env: a
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b, "cm.yaml"), []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: cfg
data:
  env: b
`), 0o600); err != nil {
		t.Fatal(err)
	}

	srv, err := NewServer(root, true, false)
	if err != nil {
		t.Fatal(err)
	}
	ck, err := srv.CreateCheckpoint()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := srv.Render(ck, "envs/a"); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.Render(ck, "envs/b"); err != nil {
		t.Fatal(err)
	}
	res, err := srv.DiffPaths(ck, "envs/a", "envs/b")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Modified) != 1 {
		t.Fatalf("expected 1 modified manifest, got %+v", res.Modified)
	}
}

func TestServer_dependencies(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("base/kustomization.yaml", `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- r.yaml
`)
	write("base/r.yaml", `apiVersion: v1
kind: ConfigMap
metadata:
  name: r
`)
	write("over/kustomization.yaml", `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- ../base
`)

	srv, err := NewServer(root, true, false)
	if err != nil {
		t.Fatal(err)
	}
	deps, err := srv.Dependencies("over/kustomization.yaml", false, false)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"base/kustomization.yaml"}
	if !slices.Equal(deps, want) {
		t.Fatalf("deps %v want %v", deps, want)
	}
}

func TestServer_diffCheckpoints(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "app")
	if err := os.MkdirAll(app, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "kustomization.yaml"), []byte(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- d.yaml
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "d.yaml"), []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: x
data:
  v: "1"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	srv, err := NewServer(root, true, false)
	if err != nil {
		t.Fatal(err)
	}
	c1, err := srv.CreateCheckpoint()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := srv.Render(c1, "app"); err != nil {
		t.Fatal(err)
	}
	// Change manifest and second checkpoint
	if err := os.WriteFile(filepath.Join(app, "d.yaml"), []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: x
data:
  v: "2"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	c2, err := srv.CreateCheckpoint()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := srv.Render(c2, "app"); err != nil {
		t.Fatal(err)
	}
	res, err := srv.DiffCheckpoints(c1, c2)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Modified) != 1 {
		t.Fatalf("expected 1 modified, got %+v", res.Modified)
	}
}

func TestServer_diffCheckpoints_missingCheckpoint(t *testing.T) {
	root := t.TempDir()
	srv, err := NewServer(root, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := srv.DiffCheckpoints("no-such", "also-no"); err == nil {
		t.Fatal("expected error")
	}
}

func TestServer_diffCheckpoints_emptyCheckpoint(t *testing.T) {
	root := t.TempDir()
	srv, err := NewServer(root, true, false)
	if err != nil {
		t.Fatal(err)
	}
	c1, err := srv.CreateCheckpoint()
	if err != nil {
		t.Fatal(err)
	}
	c2, err := srv.CreateCheckpoint()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := srv.DiffCheckpoints(c1, c2); err == nil {
		t.Fatal("expected error when checkpoints have no rendered paths")
	}
}

func TestServer_diffPaths_autoRender(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a")
	if err := os.MkdirAll(a, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a, "kustomization.yaml"), []byte(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- x.yaml
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a, "x.yaml"), []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: x
`), 0o600); err != nil {
		t.Fatal(err)
	}
	b := filepath.Join(root, "b")
	if err := os.MkdirAll(b, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b, "kustomization.yaml"), []byte(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- y.yaml
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b, "y.yaml"), []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: y
`), 0o600); err != nil {
		t.Fatal(err)
	}

	srv, err := NewServer(root, true, false)
	if err != nil {
		t.Fatal(err)
	}
	ck, err := srv.CreateCheckpoint()
	if err != nil {
		t.Fatal(err)
	}
	// Do not pre-render; DiffPaths should render on demand.
	if _, err := srv.DiffPaths(ck, "a", "b"); err != nil {
		t.Fatal(err)
	}
}

func TestServer_dependencies_invalidPath(t *testing.T) {
	root := t.TempDir()
	srv, err := NewServer(root, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := srv.Dependencies("a/../b/kustomization.yaml", false, false); err == nil {
		t.Fatal("expected error")
	}
}

func TestServer_clearAllCheckpoints(t *testing.T) {
	root := t.TempDir()
	srv, err := NewServer(root, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := srv.CreateCheckpoint(); err != nil {
		t.Fatal(err)
	}
	if err := srv.ClearCheckpoint(nil); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(root, ".kustomize-mcp", "checkpoints"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no checkpoints, got %d", len(entries))
	}
}

func TestServer_clearCheckpoint(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "kustomization.yaml"), []byte(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
`), 0o600); err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(root, true, false)
	if err != nil {
		t.Fatal(err)
	}
	id, err := srv.CreateCheckpoint()
	if err != nil {
		t.Fatal(err)
	}
	s := id
	if err := srv.ClearCheckpoint(&s); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".kustomize-mcp", "checkpoints", id)); !os.IsNotExist(err) {
		t.Fatalf("checkpoint should be removed: %v", err)
	}
}

func TestServer_inventory(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "app")
	if err := os.MkdirAll(app, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "kustomization.yaml"), []byte(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- cm.yaml
- svc.yaml
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "cm.yaml"), []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: cfg
data:
  k: v
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "svc.yaml"), []byte(`apiVersion: v1
kind: Service
metadata:
  name: web
spec:
  ports:
  - port: 80
`), 0o600); err != nil {
		t.Fatal(err)
	}

	srv, err := NewServer(root, true, false)
	if err != nil {
		t.Fatal(err)
	}
	ck, err := srv.CreateCheckpoint()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := srv.Render(ck, "app"); err != nil {
		t.Fatal(err)
	}

	path := "app"
	tree, err := srv.Inventory(ck, &path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if tree.Total != 2 {
		t.Fatalf("expected 2 resources, got %d", tree.Total)
	}
}

func TestServer_trace(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "base")
	overlay := filepath.Join(root, "overlay")
	if err := os.MkdirAll(base, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(overlay, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "kustomization.yaml"), []byte(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- deploy.yaml
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "deploy.yaml"), []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: app
spec:
  replicas: 1
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(overlay, "kustomization.yaml"), []byte(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namePrefix: prod-
resources:
- ../base
`), 0o600); err != nil {
		t.Fatal(err)
	}

	srv, err := NewServer(root, true, false)
	if err != nil {
		t.Fatal(err)
	}
	ck, err := srv.CreateCheckpoint()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := srv.Render(ck, "overlay"); err != nil {
		t.Fatal(err)
	}

	result, err := srv.Trace(ck, "overlay", "Deployment", "prod-app", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Origin == nil {
		t.Fatal("expected origin")
	}
	if result.Resource.Kind != "Deployment" {
		t.Fatalf("expected Deployment, got %s", result.Resource.Kind)
	}
}
