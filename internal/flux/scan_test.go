package flux

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanRenderedDir_empty(t *testing.T) {
	dir := t.TempDir()
	got, err := ScanRenderedDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %d", len(got))
	}
}

func TestScanRenderedDir_fullSpec(t *testing.T) {
	dir := t.TempDir()
	yaml := `apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: apps
  namespace: flux-system
spec:
  path: ./clusters/prod
  targetNamespace: prod
  namePrefix: p-
  nameSuffix: -s
  components:
  - ../../components/monitoring
  patches:
  - patch: |-
      apiVersion: v1
      kind: ConfigMap
      metadata:
        name: cfg
      data:
        k: v
    target:
      kind: ConfigMap
      name: cfg
  patchesStrategicMerge:
  - metadata:
      name: dep
    spec:
      replicas: 3
  patchesJson6902:
  - patch:
    - op: replace
      path: /spec/replicas
      value: 2
    target:
      kind: Deployment
      name: web
  images:
  - name: nginx
    newName: my.registry/nginx
    newTag: "1.27"
  commonMetadata:
    labels:
      env: prod
    annotations:
      note: "from-flux"
`
	if err := os.WriteFile(filepath.Join(dir, "flux.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "other.yaml"), []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: x
`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ScanRenderedDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 flux ks, got %d", len(got))
	}
	s := got[0]
	if s.Name != "apps" || s.Namespace != "flux-system" {
		t.Fatalf("metadata: %+v", s)
	}
	if s.Path != "./clusters/prod" || s.SpecPath != "./clusters/prod" || s.PathAnnotation != "" {
		t.Fatalf("path fields: Path=%q SpecPath=%q PathAnnotation=%q", s.Path, s.SpecPath, s.PathAnnotation)
	}
	if s.TargetNamespace != "prod" || s.NamePrefix != "p-" || s.NameSuffix != "-s" {
		t.Fatalf("transforms: %+v", s)
	}
	if len(s.Components) != 1 || s.Components[0] != "../../components/monitoring" {
		t.Fatalf("components: %v", s.Components)
	}
	if len(s.Patches) != 1 {
		t.Fatalf("patches: %d", len(s.Patches))
	}
	if len(s.PatchesStrategicMerge) != 1 {
		t.Fatalf("patchesSM: %d", len(s.PatchesStrategicMerge))
	}
	if len(s.PatchesJSON6902) != 1 {
		t.Fatalf("patches6902: %d", len(s.PatchesJSON6902))
	}
	if len(s.Images) != 1 || s.Images[0]["name"] != "nginx" {
		t.Fatalf("images: %+v", s.Images)
	}
	if s.CommonMetadata == nil || s.CommonMetadata.Labels["env"] != "prod" {
		t.Fatalf("commonMetadata: %+v", s.CommonMetadata)
	}
}

func TestScanRenderedDir_multiDoc(t *testing.T) {
	dir := t.TempDir()
	yaml := `apiVersion: v1
kind: ConfigMap
metadata:
  name: a
---
apiVersion: kustomize.toolkit.fluxcd.io/v1beta2
kind: Kustomization
metadata:
  name: nested
  namespace: ns1
spec:
  path: infra
`
	if err := os.WriteFile(filepath.Join(dir, "multi.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ScanRenderedDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1, got %d", len(got))
	}
	g := got[0]
	if g.Name != "nested" || g.Path != "infra" || g.SpecPath != "infra" || g.PathAnnotation != "" {
		t.Fatalf("%+v", g)
	}
}

func TestParseFlux_pathAnnotationOverridesSpecPath(t *testing.T) {
	dir := t.TempDir()
	yaml := `apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: apps
  namespace: flux-system
  annotations:
    ` + PathAnnotation + `: deploy/prod
spec:
  path: ./clusters/prod
`
	if err := os.WriteFile(filepath.Join(dir, "fk.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ScanRenderedDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1, got %d", len(got))
	}
	s := got[0]
	if s.Path != "deploy/prod" || s.SpecPath != "./clusters/prod" || s.PathAnnotation != "deploy/prod" {
		t.Fatalf("got Path=%q SpecPath=%q PathAnnotation=%q", s.Path, s.SpecPath, s.PathAnnotation)
	}
}

func TestParseFlux_emptyAnnotationFallsBackToSpecPath(t *testing.T) {
	dir := t.TempDir()
	yaml := `apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: apps
  namespace: flux-system
  annotations:
    ` + PathAnnotation + `: ""
spec:
  path: clusters/stage
`
	if err := os.WriteFile(filepath.Join(dir, "fk.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ScanRenderedDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1, got %d", len(got))
	}
	s := got[0]
	if s.Path != "clusters/stage" || s.SpecPath != "clusters/stage" || s.PathAnnotation != "" {
		t.Fatalf("got Path=%q SpecPath=%q PathAnnotation=%q", s.Path, s.SpecPath, s.PathAnnotation)
	}
}

func TestParseFlux_noPathAnnotation(t *testing.T) {
	dir := t.TempDir()
	yaml := `apiVersion: kustomize.toolkit.fluxcd.io/v1beta2
kind: Kustomization
metadata:
  name: x
  namespace: ns
spec:
  path: overlays/prod
`
	if err := os.WriteFile(filepath.Join(dir, "fk.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ScanRenderedDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1, got %d", len(got))
	}
	s := got[0]
	if s.Path != "overlays/prod" || s.SpecPath != "overlays/prod" || s.PathAnnotation != "" {
		t.Fatalf("got Path=%q SpecPath=%q PathAnnotation=%q", s.Path, s.SpecPath, s.PathAnnotation)
	}
}

func TestScanRenderedDir_skipsTree(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, manifestTreeFilename), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ScanRenderedDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d", len(got))
	}
}

func TestParseFlux_emptyNamespaceNormalizedToDefault(t *testing.T) {
	dir := t.TempDir()
	yaml := `apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: ns-less
spec:
  path: overlays/x
`
	if err := os.WriteFile(filepath.Join(dir, "fk.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ScanRenderedDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1, got %d", len(got))
	}
	s := got[0]
	if s.Namespace != DefaultNamespace {
		t.Fatalf("namespace want %q got %q", DefaultNamespace, s.Namespace)
	}
	if s.Key() != DefaultNamespace+"/ns-less" {
		t.Fatalf("Key want %q got %q", DefaultNamespace+"/ns-less", s.Key())
	}
}

func TestParseFlux_explicitNamespacePreserved(t *testing.T) {
	dir := t.TempDir()
	yaml := `apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: with-ns
  namespace: tenant-a
spec:
  path: overlays/x
`
	if err := os.WriteFile(filepath.Join(dir, "fk.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ScanRenderedDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1, got %d", len(got))
	}
	if got[0].Namespace != "tenant-a" {
		t.Fatalf("namespace want tenant-a, got %q", got[0].Namespace)
	}
}

func TestFluxKustomizationSpec_Key(t *testing.T) {
	s := FluxKustomizationSpec{Namespace: "a", Name: "b"}
	if s.Key() != "a/b" {
		t.Fatal(s.Key())
	}
}
