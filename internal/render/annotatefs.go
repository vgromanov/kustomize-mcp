package render

import (
	"path/filepath"

	"gopkg.in/yaml.v3"
	"sigs.k8s.io/kustomize/kyaml/filesys"

	"github.com/vgromanov/kustomize-mcp/internal/flux"
)

var requiredBuildMetadata = []string{"originAnnotations", "transformerAnnotations"}

func isKustomizationFile(name string) bool {
	switch name {
	case "kustomization.yaml", "kustomization.yml", "Kustomization":
		return true
	default:
		return false
	}
}

// annotatingFS wraps a filesys.FileSystem and transparently injects
// buildMetadata: [originAnnotations, transformerAnnotations] into every
// kustomization file that krusty reads. This gives the render pipeline
// access to origin and transformer annotations without requiring users
// to declare them in their kustomization files.
type annotatingFS struct {
	filesys.FileSystem
}

func newAnnotatingFS(inner filesys.FileSystem) filesys.FileSystem {
	return &annotatingFS{FileSystem: inner}
}

// fluxAnnotatingFS wraps a filesystem and injects Flux Kustomization spec fields
// into the kustomization.yaml at kustomizeRootAbs only; other kustomization files
// receive buildMetadata only. Disk files are never modified.
type fluxAnnotatingFS struct {
	filesys.FileSystem
	kustomizeRootAbs string
	spec             flux.FluxKustomizationSpec
}

func newFluxAnnotatingFS(inner filesys.FileSystem, kustomizeRootAbs string, spec flux.FluxKustomizationSpec) filesys.FileSystem {
	return &fluxAnnotatingFS{
		FileSystem:       inner,
		kustomizeRootAbs: filepath.Clean(kustomizeRootAbs),
		spec:             spec,
	}
}

func (fs *fluxAnnotatingFS) ReadFile(path string) ([]byte, error) {
	data, err := fs.FileSystem.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if !isKustomizationFile(filepath.Base(path)) {
		return data, nil
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return injectBuildMetadataSafe(data)
	}
	dir := filepath.Clean(filepath.Dir(abs))
	if dir == fs.kustomizeRootAbs {
		data, err = injectFluxFields(data, &fs.spec)
		if err != nil {
			return nil, err
		}
	}
	return injectBuildMetadataSafe(data)
}

func injectBuildMetadataSafe(data []byte) ([]byte, error) {
	out, err := injectBuildMetadata(data)
	if err != nil {
		return data, nil
	}
	return out, nil
}

func (fs *annotatingFS) ReadFile(path string) ([]byte, error) {
	data, err := fs.FileSystem.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if isKustomizationFile(filepath.Base(path)) {
		patched, patchErr := injectBuildMetadata(data)
		if patchErr != nil {
			return data, nil
		}
		return patched, nil
	}
	return data, nil
}

// injectBuildMetadata ensures the YAML document contains
// buildMetadata with both originAnnotations and transformerAnnotations.
func injectBuildMetadata(data []byte) ([]byte, error) {
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	if doc == nil {
		return data, nil
	}

	existing := toStringSlice(doc["buildMetadata"])
	merged := mergeStringSlice(existing, requiredBuildMetadata)
	if len(merged) == len(existing) && sliceEqual(merged, existing) {
		return data, nil
	}
	doc["buildMetadata"] = merged
	return yaml.Marshal(doc)
}

func toStringSlice(v any) []string {
	if v == nil {
		return nil
	}
	sl, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(sl))
	for _, item := range sl {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func mergeStringSlice(base, additions []string) []string {
	set := make(map[string]bool, len(base))
	for _, s := range base {
		set[s] = true
	}
	out := make([]string, len(base))
	copy(out, base)
	for _, s := range additions {
		if !set[s] {
			out = append(out, s)
			set[s] = true
		}
	}
	return out
}

func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// userBuildMetadata reads the original kustomization file (before injection)
// and returns which buildMetadata options the user explicitly declared.
func userBuildMetadata(fSys filesys.FileSystem, kustomizationDir string) (wantsOrigin, wantsTransformer bool) {
	for _, name := range []string{"kustomization.yaml", "kustomization.yml", "Kustomization"} {
		path := filepath.Join(kustomizationDir, name)
		data, err := fSys.ReadFile(path)
		if err != nil {
			continue
		}
		var doc map[string]any
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return false, false
		}
		for _, s := range toStringSlice(doc["buildMetadata"]) {
			switch s {
			case "originAnnotations":
				wantsOrigin = true
			case "transformerAnnotations":
				wantsTransformer = true
			}
		}
		return
	}
	return false, false
}

// injectFluxFields merges Flux Kustomization spec fields into a kustomization document
// (same semantics as fluxcd/pkg/kustomize Generator.WriteFile for these fields).
func injectFluxFields(data []byte, spec *flux.FluxKustomizationSpec) ([]byte, error) {
	if spec == nil {
		return data, nil
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	if doc == nil {
		return data, nil
	}
	if spec.TargetNamespace != "" {
		doc["namespace"] = spec.TargetNamespace
	}
	if spec.NamePrefix != "" {
		doc["namePrefix"] = spec.NamePrefix
	}
	if spec.NameSuffix != "" {
		doc["nameSuffix"] = spec.NameSuffix
	}
	if len(spec.Patches) > 0 {
		doc["patches"] = appendUniquePatches(doc["patches"], spec.Patches)
	}
	if len(spec.PatchesStrategicMerge) > 0 {
		doc["patchesStrategicMerge"] = appendUniqueMaps(doc["patchesStrategicMerge"], spec.PatchesStrategicMerge)
	}
	if len(spec.PatchesJSON6902) > 0 {
		doc["patchesJson6902"] = appendUniqueMaps(doc["patchesJson6902"], spec.PatchesJSON6902)
	}
	if len(spec.Components) > 0 {
		doc["components"] = appendUniqueStrings(doc["components"], spec.Components)
	}
	if len(spec.Images) > 0 {
		doc["images"] = mergeKustomizeImages(doc["images"], spec.Images)
	}
	return yaml.Marshal(doc)
}

func mapsToAnySlice(in []map[string]any) []any {
	out := make([]any, 0, len(in))
	for _, m := range in {
		out = append(out, m)
	}
	return out
}

func appendAnySlice(existing any, add []any) []any {
	var out []any
	if existing != nil {
		if sl, ok := existing.([]any); ok {
			out = append(out, sl...)
		}
	}
	out = append(out, add...)
	return out
}

func appendUniqueStrings(existing any, add []string) []any {
	out := stringSliceToAny(existing)
	seen := make(map[string]bool)
	for _, x := range out {
		if s, ok := x.(string); ok {
			seen[s] = true
		}
	}
	for _, s := range add {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func stringSliceToAny(existing any) []any {
	var out []any
	if existing == nil {
		return out
	}
	if sl, ok := existing.([]any); ok {
		out = append(out, sl...)
	}
	return out
}

func appendUniquePatches(existing any, add []map[string]any) []any {
	var out []any
	if existing != nil {
		if sl, ok := existing.([]any); ok {
			out = append(out, sl...)
		}
	}
	for _, p := range add {
		if patchListContainsMap(out, p) {
			continue
		}
		out = append(out, cloneStringMap(p))
	}
	return out
}

func appendUniqueMaps(existing any, add []map[string]any) []any {
	var out []any
	if existing != nil {
		if sl, ok := existing.([]any); ok {
			out = append(out, sl...)
		}
	}
	for _, p := range add {
		if mapSliceContains(out, p) {
			continue
		}
		out = append(out, cloneStringMap(p))
	}
	return out
}

func patchListContainsMap(list []any, want map[string]any) bool {
	for _, x := range list {
		if m, ok := x.(map[string]any); ok && mapsEqualNormalized(m, want) {
			return true
		}
	}
	return false
}

func mapSliceContains(list []any, want map[string]any) bool {
	for _, x := range list {
		if m, ok := x.(map[string]any); ok && mapsEqualNormalized(m, want) {
			return true
		}
	}
	return false
}

func mapsEqualNormalized(a, b map[string]any) bool {
	ab, _ := yaml.Marshal(a)
	bb, _ := yaml.Marshal(b)
	return string(ab) == string(bb)
}

func mergeKustomizeImages(existing any, fluxImages []map[string]any) []any {
	images := make([]map[string]any, 0)
	if existing != nil {
		if sl, ok := existing.([]any); ok {
			for _, x := range sl {
				if m, ok := x.(map[string]any); ok {
					images = append(images, m)
				}
			}
		}
	}
	for _, fi := range fluxImages {
		name, _ := fi["name"].(string)
		if name == "" {
			continue
		}
		replaced := false
		for i := range images {
			if n, _ := images[i]["name"].(string); n == name {
				images[i] = cloneStringMap(fi)
				replaced = true
				break
			}
		}
		if !replaced {
			images = append(images, cloneStringMap(fi))
		}
	}
	out := make([]any, len(images))
	for i := range images {
		out[i] = images[i]
	}
	return out
}

func cloneStringMap(m map[string]any) map[string]any {
	cp := make(map[string]any, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}
