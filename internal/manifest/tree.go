package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const TreeFilename = "_tree.json"

// ResourceIdentifier identifies a Kubernetes-like object by GVK + name.
type ResourceIdentifier struct {
	APIVersion string `json:"api_version,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Name       string `json:"name,omitempty"`
	Namespace  string `json:"namespace,omitempty"`
}

// ResourceOrigin describes where a resource came from or what configured it.
// Fields mirror krusty's resource.Origin but use workspace-relative paths.
type ResourceOrigin struct {
	Path         string              `json:"path,omitempty"`
	Repo         string              `json:"repo,omitempty"`
	Ref          string              `json:"ref,omitempty"`
	ConfiguredIn string              `json:"configured_in,omitempty"`
	ConfiguredBy *ResourceIdentifier `json:"configured_by,omitempty"`
}

// FluxOrigin identifies the Flux Kustomization that produced a render subtree.
type FluxOrigin struct {
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
	SpecPath     string `json:"spec_path"`               // effective workspace-relative path used for render
	DeclaredPath string `json:"declared_path,omitempty"` // raw spec.path when kustomization-path annotation overrode it
}

// ResourceConflict reports the same GVK+namespace+name from multiple Flux Kustomizations.
type ResourceConflict struct {
	Resource Metadata     `json:"resource"`
	Origins  []FluxOrigin `json:"origins"`
}

// ResourceEntry holds a rendered resource's metadata together with its
// origin and the ordered list of transformers that modified it.
type ResourceEntry struct {
	Metadata          Metadata         `json:"metadata"`
	Origin            *ResourceOrigin  `json:"origin,omitempty"`
	Transformations   []ResourceOrigin `json:"transformations,omitempty"`
	FluxKustomization *FluxOrigin      `json:"flux_kustomization,omitempty"`
}

// ResourceTree is the sidecar written next to rendered manifests.
type ResourceTree struct {
	Path              string             `json:"path"`
	FluxKustomization *FluxOrigin        `json:"flux_kustomization,omitempty"`
	Resources         []ResourceEntry    `json:"resources"`
	Conflicts         []ResourceConflict `json:"conflicts,omitempty"`
	Total             int                `json:"total"`
}

// WriteTree serializes a ResourceTree as JSON into dir/_tree.json.
func WriteTree(dir string, tree *ResourceTree) error {
	tree.Total = len(tree.Resources)
	data, err := json.MarshalIndent(tree, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, TreeFilename), data, 0o600)
}

// ReadTree loads a ResourceTree from dir/_tree.json.
func ReadTree(dir string) (*ResourceTree, error) {
	data, err := os.ReadFile(filepath.Join(dir, TreeFilename))
	if err != nil {
		return nil, err
	}
	var tree ResourceTree
	if err := json.Unmarshal(data, &tree); err != nil {
		return nil, err
	}
	return &tree, nil
}
