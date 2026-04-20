package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vgromanov/kustomize-mcp/internal/manifest"
)

func TestRenderer_renderRoundTrip(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "apps", "demo")
	if err := os.MkdirAll(app, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "kustomization.yaml"), []byte(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- deploy.yaml
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "deploy.yaml"), []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
spec:
  replicas: 1
  selector:
    matchLabels:
      app: demo
  template:
    metadata:
      labels:
        app: demo
    spec:
      containers:
      - name: demo
        image: nginx:1.25-alpine
`), 0o600); err != nil {
		t.Fatal(err)
	}

	r, err := NewRenderer(root, true, false)
	if err != nil {
		t.Fatal(err)
	}
	ck, err := r.CreateCheckpoint()
	if err != nil {
		t.Fatal(err)
	}
	out, err := r.Render(ck, "apps/demo")
	if err != nil {
		t.Fatal(err)
	}
	if out == "" {
		t.Fatal("empty output path")
	}
	rendered := filepath.Join(root, out)
	st, err := os.Stat(rendered)
	if err != nil || !st.IsDir() {
		t.Fatalf("rendered dir: %v", err)
	}
	entries, err := os.ReadDir(rendered)
	if err != nil {
		t.Fatal(err)
	}
	// expect the manifest file + _tree.json sidecar
	if len(entries) != 2 {
		t.Fatalf("expected 2 files (manifest + _tree.json), got %d", len(entries))
	}
	hasTree := false
	for _, e := range entries {
		if e.Name() == "_tree.json" {
			hasTree = true
		}
	}
	if !hasTree {
		t.Fatal("expected _tree.json sidecar")
	}
}

func TestRenderer_duplicateRenderErrors(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "k")
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
`), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := NewRenderer(root, true, false)
	if err != nil {
		t.Fatal(err)
	}
	ck, err := r.CreateCheckpoint()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Render(ck, "k"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Render(ck, "k"); err == nil {
		t.Fatal("expected error on second render of same path")
	}
}

func TestValidateRelPath(t *testing.T) {
	for _, p := range []string{"", "..", "a/../b", "/abs"} {
		if err := validateRelPath(p); err == nil {
			t.Fatalf("expected error for %q", p)
		}
	}
	if err := validateRelPath("ok/sub"); err != nil {
		t.Fatal(err)
	}
}

func TestRenderer_CheckpointsDir(t *testing.T) {
	root := t.TempDir()
	r, err := NewRenderer(root, true, false)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, ".kustomize-mcp", "checkpoints")
	if r.CheckpointsDir() != want {
		t.Fatalf("got %q want %q", r.CheckpointsDir(), want)
	}
}

func TestRenderer_ClearAll(t *testing.T) {
	root := t.TempDir()
	r, err := NewRenderer(root, true, false)
	if err != nil {
		t.Fatal(err)
	}
	ck, err := r.CreateCheckpoint()
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Clear(nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(r.CheckpointsDir(), ck)); !os.IsNotExist(err) {
		t.Fatalf("checkpoint should be gone: %v", err)
	}
}

func TestRenderer_Clear_invalidID(t *testing.T) {
	root := t.TempDir()
	r, err := NewRenderer(root, true, false)
	if err != nil {
		t.Fatal(err)
	}
	bad := "../x"
	if err := r.Clear(&bad); err == nil {
		t.Fatal("expected error for invalid checkpoint id")
	}
}

func TestRenderer_Render_invalidCheckpointID(t *testing.T) {
	root := t.TempDir()
	r, err := NewRenderer(root, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Render("../bad", "x"); err == nil {
		t.Fatal("expected error")
	}
}

func TestRenderer_Render_invalidRelPath(t *testing.T) {
	root := t.TempDir()
	r, err := NewRenderer(root, true, false)
	if err != nil {
		t.Fatal(err)
	}
	ck, err := r.CreateCheckpoint()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Render(ck, "a/../b"); err == nil {
		t.Fatal("expected error")
	}
}

func TestRenderer_Render_missingSource(t *testing.T) {
	root := t.TempDir()
	r, err := NewRenderer(root, true, false)
	if err != nil {
		t.Fatal(err)
	}
	ck, err := r.CreateCheckpoint()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Render(ck, "nope"); err == nil {
		t.Fatal("expected error for missing dir")
	}
}

func TestRenderer_Render_sourceIsFile(t *testing.T) {
	root := t.TempDir()
	f := filepath.Join(root, "notadir")
	if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := NewRenderer(root, true, false)
	if err != nil {
		t.Fatal(err)
	}
	ck, err := r.CreateCheckpoint()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Render(ck, "notadir"); err == nil {
		t.Fatal("expected error when source is not a directory")
	}
}

func TestRenderer_Render_kustomizeFailureCleansDest(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "bad")
	if err := os.MkdirAll(app, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "kustomization.yaml"), []byte(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- does-not-exist.yaml
`), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := NewRenderer(root, true, false)
	if err != nil {
		t.Fatal(err)
	}
	ck, err := r.CreateCheckpoint()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Render(ck, "bad"); err == nil {
		t.Fatal("expected kustomize error")
	}
	dest := filepath.Join(r.CheckpointsDir(), ck, "bad")
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("partial dest should be removed: %v", err)
	}
}

func TestRenderer_LoadRestrictionsNone(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "k")
	if err := os.MkdirAll(app, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "kustomization.yaml"), []byte(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- cm.yaml
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "cm.yaml"), []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: c
`), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := NewRenderer(root, false, false)
	if err != nil {
		t.Fatal(err)
	}
	ck, err := r.CreateCheckpoint()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Render(ck, "k"); err != nil {
		t.Fatal(err)
	}
}

func TestValidateCheckpointID(t *testing.T) {
	for _, id := range []string{"", ".", "..", "a/b"} {
		if err := validateCheckpointID(id); err == nil {
			t.Fatalf("expected error for %q", id)
		}
	}
	if err := validateCheckpointID(string([]byte{'a', filepath.Separator, 'b'})); err == nil {
		t.Fatal("expected error when checkpoint id contains path separator")
	}
	if err := validateCheckpointID("  ckp-1  "); err != nil {
		t.Fatal(err)
	}
}

func TestRenderer_treeJSON_baseOverlay(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "base")
	overlay := filepath.Join(root, "overlay")
	if err := os.MkdirAll(base, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(overlay, 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, base, "kustomization.yaml", `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- deploy.yaml
`)
	writeTestFile(t, base, "deploy.yaml", `apiVersion: apps/v1
kind: Deployment
metadata:
  name: app
spec:
  replicas: 1
`)
	writeTestFile(t, overlay, "kustomization.yaml", `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namePrefix: prod-
resources:
- ../base
`)

	r, err := NewRenderer(root, true, false)
	if err != nil {
		t.Fatal(err)
	}
	ck, err := r.CreateCheckpoint()
	if err != nil {
		t.Fatal(err)
	}
	out, err := r.Render(ck, "overlay")
	if err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(root, out)
	tree, err := manifest.ReadTree(dest)
	if err != nil {
		t.Fatalf("reading _tree.json: %v", err)
	}
	if tree.Total != 1 {
		t.Fatalf("expected 1 resource, got %d", tree.Total)
	}
	entry := tree.Resources[0]
	if entry.Metadata.Kind != "Deployment" {
		t.Fatalf("expected Deployment, got %s", entry.Metadata.Kind)
	}
	if entry.Metadata.Name != "prod-app" {
		t.Fatalf("expected prod-app, got %s", entry.Metadata.Name)
	}
	if entry.Origin == nil {
		t.Fatal("expected origin to be populated")
	}
	if !strings.HasPrefix(entry.Origin.Path, "base/") && !strings.Contains(entry.Origin.Path, "deploy.yaml") {
		t.Fatalf("origin path should reference base/deploy.yaml, got %s", entry.Origin.Path)
	}

	// Verify rendered YAML does NOT contain injected annotations
	entries, err := os.ReadDir(dest)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() == manifest.TreeFilename {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dest, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "config.kubernetes.io/origin") {
			t.Fatalf("rendered YAML should not contain injected origin annotation")
		}
	}
}

func TestRenderer_treeJSON_userBuildMetadataPreserved(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "app")
	if err := os.MkdirAll(app, 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, app, "kustomization.yaml", `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
buildMetadata: [originAnnotations]
resources:
- svc.yaml
`)
	writeTestFile(t, app, "svc.yaml", `apiVersion: v1
kind: Service
metadata:
  name: web
spec:
  ports:
  - port: 80
`)

	r, err := NewRenderer(root, true, false)
	if err != nil {
		t.Fatal(err)
	}
	ck, err := r.CreateCheckpoint()
	if err != nil {
		t.Fatal(err)
	}
	out, err := r.Render(ck, "app")
	if err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(root, out)
	entries, err := os.ReadDir(dest)
	if err != nil {
		t.Fatal(err)
	}
	foundOrigin := false
	for _, e := range entries {
		if e.Name() == manifest.TreeFilename {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dest, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "config.kubernetes.io/origin") {
			foundOrigin = true
		}
		if strings.Contains(string(data), "alpha.config.kubernetes.io/transformations") {
			t.Fatal("transformer annotations should be stripped when user only requested origin")
		}
	}
	if !foundOrigin {
		t.Fatal("user requested originAnnotations but origin annotation not found in rendered YAML")
	}
}

func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
