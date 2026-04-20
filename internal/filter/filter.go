package filter

import (
	"github.com/vgromanov/kustomize-mcp/internal/manifest"
)

// ResourceFilter selects resources by exact match on non-nil fields.
// A nil filter or a filter with all nil fields matches everything.
type ResourceFilter struct {
	Kind       *string `json:"kind,omitempty"`
	APIVersion *string `json:"api_version,omitempty"`
	Namespace  *string `json:"namespace,omitempty"`
	Name       *string `json:"name,omitempty"`
}

// Match returns true when the metadata satisfies all non-nil filter fields.
func (f *ResourceFilter) Match(m manifest.Metadata) bool {
	if f == nil {
		return true
	}
	if f.Kind != nil && *f.Kind != m.Kind {
		return false
	}
	if f.APIVersion != nil && *f.APIVersion != m.APIVersion {
		return false
	}
	if f.Name != nil && *f.Name != m.Name {
		return false
	}
	if f.Namespace != nil {
		ns := ""
		if m.Namespace != nil {
			ns = *m.Namespace
		}
		if *f.Namespace != ns {
			return false
		}
	}
	return true
}

// FilterMetadata returns only the entries that match f.
func FilterMetadata(items []manifest.Metadata, f *ResourceFilter) []manifest.Metadata {
	if f == nil {
		return items
	}
	var out []manifest.Metadata
	for _, m := range items {
		if f.Match(m) {
			out = append(out, m)
		}
	}
	return out
}

// FilterEntries returns only the resource entries that match f.
func FilterEntries(items []manifest.ResourceEntry, f *ResourceFilter) []manifest.ResourceEntry {
	if f == nil {
		return items
	}
	var out []manifest.ResourceEntry
	for _, e := range items {
		if f.Match(e.Metadata) {
			out = append(out, e)
		}
	}
	return out
}
