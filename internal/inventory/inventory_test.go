package inventory

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vgromanov/kustomize-mcp/internal/manifest"
)

func TestLoad_recursiveTrees(t *testing.T) {
	ck := t.TempDir()
	app := filepath.Join(ck, "app")
	ns := filepath.Join(ck, "flux-system", "fk")
	for _, d := range []string{app, ns} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	treeApp := &manifest.ResourceTree{
		Path: "app",
		Resources: []manifest.ResourceEntry{
			{Metadata: manifest.Metadata{SourcePath: "app", APIVersion: "v1", Kind: "ConfigMap", Name: "a"}},
		},
	}
	if err := manifest.WriteTree(app, treeApp); err != nil {
		t.Fatal(err)
	}
	origin := &manifest.FluxOrigin{Name: "fk", Namespace: "flux-system", SpecPath: "child"}
	treeNs := &manifest.ResourceTree{
		Path:              "flux-system/fk",
		FluxKustomization: origin,
		Resources: []manifest.ResourceEntry{
			{Metadata: manifest.Metadata{SourcePath: "flux-system/fk", APIVersion: "v1", Kind: "ConfigMap", Name: "b"}},
		},
	}
	if err := manifest.WriteTree(ns, treeNs); err != nil {
		t.Fatal(err)
	}

	merged, err := Load(ck, nil)
	if err != nil {
		t.Fatal(err)
	}
	if merged.Total != 2 {
		t.Fatalf("total: %d", merged.Total)
	}
}

func TestLoad_conflictAcrossFlux(t *testing.T) {
	ck := t.TempDir()
	a := filepath.Join(ck, "ns-a", "fk1")
	b := filepath.Join(ck, "ns-b", "fk2")
	for _, d := range []string{a, b} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	cm := manifest.ResourceEntry{
		Metadata: manifest.Metadata{
			SourcePath: "ns-a/fk1",
			APIVersion: "v1",
			Kind:       "ConfigMap",
			Name:       "shared",
			Namespace:  ptr("default"),
		},
		FluxKustomization: &manifest.FluxOrigin{Name: "fk1", Namespace: "ns-a", SpecPath: "child"},
	}
	treeA := &manifest.ResourceTree{Path: "ns-a/fk1", Resources: []manifest.ResourceEntry{cm}}
	if err := manifest.WriteTree(a, treeA); err != nil {
		t.Fatal(err)
	}
	cm2 := manifest.ResourceEntry{
		Metadata: manifest.Metadata{
			SourcePath: "ns-b/fk2",
			APIVersion: "v1",
			Kind:       "ConfigMap",
			Name:       "shared",
			Namespace:  ptr("default"),
		},
		FluxKustomization: &manifest.FluxOrigin{Name: "fk2", Namespace: "ns-b", SpecPath: "child"},
	}
	treeB := &manifest.ResourceTree{Path: "ns-b/fk2", Resources: []manifest.ResourceEntry{cm2}}
	if err := manifest.WriteTree(b, treeB); err != nil {
		t.Fatal(err)
	}

	merged, err := Load(ck, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(merged.Conflicts) != 1 {
		t.Fatalf("conflicts: %+v", merged.Conflicts)
	}
	if len(merged.Conflicts[0].Origins) != 2 {
		t.Fatalf("origins: %+v", merged.Conflicts[0].Origins)
	}
}

func ptr(s string) *string { return &s }
