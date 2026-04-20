package deps

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestResolver_forwardDependencies(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "base", "kustomization.yaml"), `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- cm.yaml
`)
	writeFile(t, filepath.Join(root, "base", "cm.yaml"), `apiVersion: v1
kind: ConfigMap
metadata:
  name: b
`)
	writeFile(t, filepath.Join(root, "overlay", "kustomization.yaml"), `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- ../base
`)
	r, err := NewResolver(root)
	if err != nil {
		t.Fatal(err)
	}
	deps, err := r.ComputeDependencies("overlay/kustomization.yaml", false, false)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"base/kustomization.yaml"}
	if !slices.Equal(deps, want) {
		t.Fatalf("deps got %v want %v", deps, want)
	}
}

func TestResolver_recursiveDependencies(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "base", "kustomization.yaml"), `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- cm.yaml
`)
	writeFile(t, filepath.Join(root, "base", "cm.yaml"), `apiVersion: v1
kind: ConfigMap
metadata:
  name: b
`)
	writeFile(t, filepath.Join(root, "overlay", "kustomization.yaml"), `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- ../base
`)
	r, err := NewResolver(root)
	if err != nil {
		t.Fatal(err)
	}
	deps, err := r.ComputeDependencies("overlay/kustomization.yaml", true, false)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"base/cm.yaml", "base/kustomization.yaml"}
	slices.Sort(want)
	got := append([]string(nil), deps...)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Fatalf("recursive deps got %v want %v", got, want)
	}
}

func TestResolver_reverseDependencies(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "base", "kustomization.yaml"), `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- cm.yaml
`)
	writeFile(t, filepath.Join(root, "base", "cm.yaml"), `apiVersion: v1
kind: ConfigMap
metadata:
  name: b
`)
	writeFile(t, filepath.Join(root, "overlay", "kustomization.yaml"), `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- ../base
`)
	r, err := NewResolver(root)
	if err != nil {
		t.Fatal(err)
	}
	deps, err := r.ComputeDependencies("base/cm.yaml", false, true)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"base/kustomization.yaml"}
	slices.Sort(deps)
	if !slices.Equal(deps, want) {
		t.Fatalf("reverse got %v want %v", deps, want)
	}
}

func TestResolver_reverseRecursive(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "base", "kustomization.yaml"), `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- cm.yaml
`)
	writeFile(t, filepath.Join(root, "base", "cm.yaml"), `apiVersion: v1
kind: ConfigMap
metadata:
  name: b
`)
	writeFile(t, filepath.Join(root, "overlay", "kustomization.yaml"), `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- ../base
`)
	r, err := NewResolver(root)
	if err != nil {
		t.Fatal(err)
	}
	deps, err := r.ComputeDependencies("base/cm.yaml", true, true)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"base/kustomization.yaml", "overlay/kustomization.yaml"}
	slices.Sort(deps)
	slices.Sort(want)
	if !slices.Equal(deps, want) {
		t.Fatalf("reverse recursive got %v want %v", deps, want)
	}
}

func TestResolver_notAFile(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "kustomization.yaml"), `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
`)
	r, err := NewResolver(root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.ComputeDependencies(".", false, false)
	if err == nil {
		t.Fatal("expected error for directory path")
	}
}
