package manifest

import (
	"strings"
	"testing"
)

func TestMetadata_ToFilename_roundTrip(t *testing.T) {
	ns := "prod"
	m := Metadata{
		SourcePath: "apps/demo",
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Name:       "web",
		Namespace:  &ns,
	}
	name := m.ToFilename()
	if want := "apps#v1+Deployment+prod+web.yaml"; name != want {
		t.Fatalf("ToFilename: got %q want %q", name, want)
	}
	got, err := FromRelPath("apps/demo/" + name)
	if err != nil {
		t.Fatal(err)
	}
	if got.APIVersion != m.APIVersion || got.Kind != m.Kind || got.Name != m.Name {
		t.Fatalf("FromRelPath metadata: got %+v want %+v", got, m)
	}
	if got.Namespace == nil || *got.Namespace != ns {
		t.Fatalf("namespace: got %v want %q", got.Namespace, ns)
	}
	// SourcePath is parent dir of the file in the encoding scheme.
	if got.SourcePath != "apps/demo" {
		t.Fatalf("SourcePath: got %q", got.SourcePath)
	}
}

func TestMetadata_ToFilename_noNamespace(t *testing.T) {
	m := Metadata{SourcePath: "x", APIVersion: "v1", Kind: "ConfigMap", Name: "cfg"}
	name := m.ToFilename()
	if !strings.HasSuffix(name, "+ConfigMap++cfg.yaml") {
		t.Fatalf("unexpected name %q", name)
	}
	_, err := FromRelPath("x/" + name)
	if err != nil {
		t.Fatal(err)
	}
}

func TestFromYAMLDoc(t *testing.T) {
	doc := []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: foo
  namespace: ns1
data:
  k: v
`)
	meta, err := FromYAMLDoc("overlay/prod", doc)
	if err != nil {
		t.Fatal(err)
	}
	if meta.APIVersion != "v1" || meta.Kind != "ConfigMap" || meta.Name != "foo" {
		t.Fatalf("got %+v", meta)
	}
	if meta.Namespace == nil || *meta.Namespace != "ns1" {
		t.Fatalf("namespace: %+v", meta.Namespace)
	}
}

func TestFromRelPath_errors(t *testing.T) {
	for _, p := range []string{"a.json", "bad.yaml", "a/b/notfourparts.yaml"} {
		if _, err := FromRelPath(p); err == nil {
			t.Fatalf("expected error for %q", p)
		}
	}
}
