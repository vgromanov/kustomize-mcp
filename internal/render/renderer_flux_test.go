package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vgromanov/kustomize-mcp/internal/flux"
	"github.com/vgromanov/kustomize-mcp/internal/manifest"
)

func TestRenderer_RenderFlux_commonMetadata(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "apps", "demo")
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
  name: cfg
data:
  k: v
`), 0o600); err != nil {
		t.Fatal(err)
	}

	rnd, err := NewRenderer(root, true, false)
	if err != nil {
		t.Fatal(err)
	}
	ck, err := rnd.CreateCheckpoint()
	if err != nil {
		t.Fatal(err)
	}
	spec := flux.FluxKustomizationSpec{
		Name:      "fk",
		Namespace: "flux-system",
		Path:      "apps/demo",
		CommonMetadata: &flux.CommonMetadata{
			Labels:      map[string]string{"team": "a"},
			Annotations: map[string]string{"ann": "x"},
		},
	}
	out, err := rnd.RenderFlux(ck, spec)
	if err != nil {
		t.Fatal(err)
	}
	if out == "" {
		t.Fatal("empty out")
	}
	dest := filepath.Join(rnd.CheckpointsDir(), ck, "flux-system", "fk")
	tree, err := manifest.ReadTree(dest)
	if err != nil {
		t.Fatal(err)
	}
	if tree.FluxKustomization == nil || tree.FluxKustomization.Name != "fk" {
		t.Fatalf("flux tree: %+v", tree.FluxKustomization)
	}
	data, err := os.ReadFile(filepath.Join(dest, "v1+ConfigMap++cfg.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "team:") || !strings.Contains(s, "ann:") {
		t.Fatalf("commonMetadata missing: %s", s)
	}
}
