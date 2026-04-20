package flux

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const fluxAPIPrefix = "kustomize.toolkit.fluxcd.io/"

// PathAnnotation is a workspace-local hint on rendered Flux Kustomization CRDs.
// When non-empty, it overrides spec.path for recursive rendering in this MCP.
const PathAnnotation = "kustomize.toolkit.fluxcd.io/kustomization-path"

// DefaultNamespace mirrors the Kubernetes API server default applied at
// admission time when a namespaced object is submitted without an explicit
// metadata.namespace. Flux Kustomization CRDs in source repositories often
// omit the field; we normalize it here so rendering, dest paths, cycle
// detection, and _tree.json all see the same value.
const DefaultNamespace = "default"

// CommonMetadata mirrors Flux Kustomization spec.commonMetadata.
type CommonMetadata struct {
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// FluxKustomizationSpec holds fields from a Flux Kustomization CRD used for local rendering.
type FluxKustomizationSpec struct {
	Name                  string
	Namespace             string
	Path                  string // effective workspace-relative path: PathAnnotation if set, else SpecPath
	SpecPath              string // raw spec.path from the CRD
	PathAnnotation        string // trimmed kustomize.toolkit.fluxcd.io/kustomization-path when non-empty
	TargetNamespace       string
	NamePrefix            string
	NameSuffix            string
	Patches               []map[string]any
	PatchesStrategicMerge []map[string]any
	PatchesJSON6902       []map[string]any
	Images                []map[string]any
	Components            []string
	CommonMetadata        *CommonMetadata
}

// Key returns a stable id for cycle detection (namespace/name).
func (s FluxKustomizationSpec) Key() string {
	return s.Namespace + "/" + s.Name
}

// ScanRenderedDir reads rendered YAML files in dir (non-recursive) and returns
// all Flux Kustomization resources found (including multi-document files).
func ScanRenderedDir(dir string) ([]FluxKustomizationSpec, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []FluxKustomizationSpec
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if name == manifestTreeFilename {
			continue
		}
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, err
		}
		docs, err := splitYAMLDocuments(data)
		if err != nil {
			return nil, err
		}
		for _, doc := range docs {
			spec, ok := parseFluxKustomization(doc)
			if ok {
				out = append(out, spec)
			}
		}
	}
	return out, nil
}

const manifestTreeFilename = "_tree.json"

func splitYAMLDocuments(data []byte) ([][]byte, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil, nil
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var docs [][]byte
	for {
		var doc map[string]any
		err := dec.Decode(&doc)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		if doc == nil {
			continue
		}
		b, err := yaml.Marshal(doc)
		if err != nil {
			return nil, err
		}
		docs = append(docs, b)
	}
	return docs, nil
}

func parseFluxKustomization(doc []byte) (FluxKustomizationSpec, bool) {
	var root map[string]any
	if err := yaml.Unmarshal(doc, &root); err != nil || root == nil {
		return FluxKustomizationSpec{}, false
	}
	api, _ := root["apiVersion"].(string)
	kind, _ := root["kind"].(string)
	if kind != "Kustomization" || !strings.HasPrefix(api, fluxAPIPrefix) {
		return FluxKustomizationSpec{}, false
	}
	meta, _ := root["metadata"].(map[string]any)
	name, _ := meta["name"].(string)
	ns, _ := meta["namespace"].(string)
	ns = strings.TrimSpace(ns)
	if ns == "" {
		ns = DefaultNamespace
	}
	spec, _ := root["spec"].(map[string]any)
	if spec == nil {
		return FluxKustomizationSpec{
			Name:      name,
			Namespace: ns,
			SpecPath:  "",
		}, true
	}
	pathStr, _ := spec["path"].(string)
	specPath := strings.TrimSpace(pathStr)
	pathAnn := annotationString(meta, PathAnnotation)
	effective := specPath
	if pathAnn != "" {
		effective = pathAnn
	}
	out := FluxKustomizationSpec{
		Name:                  name,
		Namespace:             ns,
		Path:                  effective,
		SpecPath:              specPath,
		PathAnnotation:        pathAnn,
		TargetNamespace:       stringField(spec, "targetNamespace"),
		NamePrefix:            stringField(spec, "namePrefix"),
		NameSuffix:            stringField(spec, "nameSuffix"),
		Components:            stringSliceField(spec, "components"),
		Patches:               mapSliceField(spec, "patches"),
		PatchesStrategicMerge: mapSliceField(spec, "patchesStrategicMerge"),
		PatchesJSON6902:       mapSliceField(spec, "patchesJson6902"),
		Images:                mapSliceField(spec, "images"),
	}
	if cm := parseCommonMetadata(spec); cm != nil {
		out.CommonMetadata = cm
	}
	return out, true
}

func stringField(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return strings.TrimSpace(v)
}

func stringSliceField(m map[string]any, key string) []string {
	raw, ok := m[key].([]any)
	if !ok {
		return nil
	}
	var out []string
	for _, x := range raw {
		if s, ok := x.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

func mapSliceField(m map[string]any, key string) []map[string]any {
	raw, ok := m[key].([]any)
	if !ok {
		return nil
	}
	var out []map[string]any
	for _, x := range raw {
		mm, ok := x.(map[string]any)
		if ok {
			out = append(out, mm)
		}
	}
	return out
}

func parseCommonMetadata(spec map[string]any) *CommonMetadata {
	raw, ok := spec["commonMetadata"].(map[string]any)
	if !ok || raw == nil {
		return nil
	}
	cm := &CommonMetadata{}
	if labels, ok := raw["labels"].(map[string]any); ok {
		cm.Labels = stringifyMap(labels)
	}
	if ann, ok := raw["annotations"].(map[string]any); ok {
		cm.Annotations = stringifyMap(ann)
	}
	if len(cm.Labels) == 0 && len(cm.Annotations) == 0 {
		return nil
	}
	return cm
}

func stringifyMap(m map[string]any) map[string]string {
	out := make(map[string]string)
	for k, v := range m {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}
	return out
}

func annotationString(meta map[string]any, key string) string {
	if meta == nil {
		return ""
	}
	ann, _ := meta["annotations"].(map[string]any)
	if ann == nil {
		return ""
	}
	s, _ := ann[key].(string)
	return strings.TrimSpace(s)
}
