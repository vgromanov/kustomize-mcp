package inventory

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"

	"github.com/vgromanov/kustomize-mcp/internal/filter"
	"github.com/vgromanov/kustomize-mcp/internal/manifest"
)

// Load reads the resource tree from a rendered checkpoint directory.
// If path is non-nil only that sub-path is loaded; otherwise all
// rendered paths in the checkpoint are merged.
func Load(checkpointDir string, path *string) (*manifest.ResourceTree, error) {
	if path != nil {
		dir := filepath.Join(checkpointDir, filepath.FromSlash(*path))
		t, err := manifest.ReadTree(dir)
		if err != nil {
			return nil, err
		}
		annotateEntriesWithFlux(t)
		t.Conflicts = detectConflicts(t.Resources)
		t.Total = len(t.Resources)
		return t, nil
	}
	merged := &manifest.ResourceTree{}
	found := false
	err := filepath.WalkDir(checkpointDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if d.Name() != manifest.TreeFilename {
			return nil
		}
		dir := filepath.Dir(path)
		if err := mergeSubtree(merged, dir); err != nil {
			return nil
		}
		found = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("no rendered paths found in checkpoint")
	}
	merged.Conflicts = detectConflicts(merged.Resources)
	merged.Total = len(merged.Resources)
	return merged, nil
}

func mergeSubtree(merged *manifest.ResourceTree, dir string) error {
	tree, err := manifest.ReadTree(dir)
	if err != nil {
		return err
	}
	annotateEntriesWithFlux(tree)
	merged.Resources = append(merged.Resources, tree.Resources...)
	return nil
}

func annotateEntriesWithFlux(tree *manifest.ResourceTree) {
	if tree.FluxKustomization == nil {
		return
	}
	for i := range tree.Resources {
		if tree.Resources[i].FluxKustomization == nil {
			o := *tree.FluxKustomization
			tree.Resources[i].FluxKustomization = &o
		}
	}
}

type resourceIdentity struct {
	apiVersion string
	kind       string
	name       string
	namespace  string
}

func identityOf(e manifest.ResourceEntry) resourceIdentity {
	ns := ""
	if e.Metadata.Namespace != nil {
		ns = *e.Metadata.Namespace
	}
	return resourceIdentity{
		apiVersion: e.Metadata.APIVersion,
		kind:       e.Metadata.Kind,
		name:       e.Metadata.Name,
		namespace:  ns,
	}
}

func fluxOriginKey(o *manifest.FluxOrigin) string {
	if o == nil {
		return ""
	}
	return o.Namespace + "/" + o.Name + "@" + o.SpecPath
}

func canonicalFluxOrigin(e manifest.ResourceEntry) manifest.FluxOrigin {
	if e.FluxKustomization != nil {
		return *e.FluxKustomization
	}
	return manifest.FluxOrigin{
		Name:      "(plain)",
		Namespace: "-",
		SpecPath:  e.Metadata.SourcePath,
	}
}

func detectConflicts(resources []manifest.ResourceEntry) []manifest.ResourceConflict {
	byID := make(map[resourceIdentity][]manifest.ResourceEntry)
	for _, e := range resources {
		id := identityOf(e)
		byID[id] = append(byID[id], e)
	}
	var out []manifest.ResourceConflict
	for id, list := range byID {
		if len(list) < 2 {
			continue
		}
		keys := make(map[string]manifest.FluxOrigin)
		for _, e := range list {
			o := canonicalFluxOrigin(e)
			k := fluxOriginKey(&o)
			if k == "" {
				k = o.SpecPath
			}
			keys[k] = o
		}
		if len(keys) < 2 {
			continue
		}
		origins := make([]manifest.FluxOrigin, 0, len(keys))
		for _, o := range keys {
			origins = append(origins, o)
		}
		slices.SortFunc(origins, func(a, b manifest.FluxOrigin) int {
			if c := compareString(a.Namespace, b.Namespace); c != 0 {
				return c
			}
			if c := compareString(a.Name, b.Name); c != 0 {
				return c
			}
			return compareString(a.SpecPath, b.SpecPath)
		})
		var nsPtr *string
		if id.namespace != "" {
			ns := id.namespace
			nsPtr = &ns
		}
		out = append(out, manifest.ResourceConflict{
			Resource: manifest.Metadata{
				SourcePath: list[0].Metadata.SourcePath,
				APIVersion: id.apiVersion,
				Kind:       id.kind,
				Name:       id.name,
				Namespace:  nsPtr,
			},
			Origins: origins,
		})
	}
	return out
}

func compareString(a, b string) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// Filter returns a copy of tree with only entries matching f.
func Filter(tree *manifest.ResourceTree, f *filter.ResourceFilter) *manifest.ResourceTree {
	filtered := filter.FilterEntries(tree.Resources, f)
	return &manifest.ResourceTree{
		Path:              tree.Path,
		FluxKustomization: tree.FluxKustomization,
		Conflicts:         tree.Conflicts,
		Resources:         filtered,
		Total:             len(filtered),
	}
}
