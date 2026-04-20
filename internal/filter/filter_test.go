package filter

import (
	"testing"

	"github.com/vgromanov/kustomize-mcp/internal/manifest"
)

func ptr(s string) *string { return &s }

func TestMatch_nilFilter(t *testing.T) {
	var f *ResourceFilter
	if !f.Match(manifest.Metadata{Kind: "Deployment"}) {
		t.Fatal("nil filter should match everything")
	}
}

func TestMatch_emptyFilter(t *testing.T) {
	f := &ResourceFilter{}
	if !f.Match(manifest.Metadata{Kind: "Deployment"}) {
		t.Fatal("empty filter should match everything")
	}
}

func TestMatch_kindMatch(t *testing.T) {
	f := &ResourceFilter{Kind: ptr("Deployment")}
	if !f.Match(manifest.Metadata{Kind: "Deployment"}) {
		t.Fatal("should match")
	}
	if f.Match(manifest.Metadata{Kind: "Service"}) {
		t.Fatal("should not match different kind")
	}
}

func TestMatch_namespaceMatch(t *testing.T) {
	ns := "prod"
	f := &ResourceFilter{Namespace: ptr("prod")}
	if !f.Match(manifest.Metadata{Kind: "Deployment", Namespace: &ns}) {
		t.Fatal("should match")
	}
	if f.Match(manifest.Metadata{Kind: "Deployment"}) {
		t.Fatal("should not match nil namespace")
	}
}

func TestMatch_multipleFields(t *testing.T) {
	ns := "prod"
	f := &ResourceFilter{Kind: ptr("Deployment"), Name: ptr("app"), Namespace: ptr("prod")}
	if !f.Match(manifest.Metadata{Kind: "Deployment", Name: "app", Namespace: &ns}) {
		t.Fatal("should match")
	}
	if f.Match(manifest.Metadata{Kind: "Deployment", Name: "other", Namespace: &ns}) {
		t.Fatal("should not match different name")
	}
}

func TestFilterMetadata(t *testing.T) {
	items := []manifest.Metadata{
		{Kind: "Deployment", Name: "a"},
		{Kind: "Service", Name: "b"},
		{Kind: "Deployment", Name: "c"},
	}
	f := &ResourceFilter{Kind: ptr("Deployment")}
	got := FilterMetadata(items, f)
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
}

func TestFilterMetadata_nilFilter(t *testing.T) {
	items := []manifest.Metadata{
		{Kind: "Deployment", Name: "a"},
	}
	got := FilterMetadata(items, nil)
	if len(got) != 1 {
		t.Fatalf("nil filter should return all items")
	}
}

func TestFilterEntries(t *testing.T) {
	entries := []manifest.ResourceEntry{
		{Metadata: manifest.Metadata{Kind: "Deployment", Name: "a"}},
		{Metadata: manifest.Metadata{Kind: "Service", Name: "b"}},
	}
	f := &ResourceFilter{Kind: ptr("Service")}
	got := FilterEntries(entries, f)
	if len(got) != 1 || got[0].Metadata.Name != "b" {
		t.Fatalf("expected 1 Service entry, got %v", got)
	}
}
