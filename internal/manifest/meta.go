package manifest

import (
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Metadata describes a rendered manifest file (matches mbrt/kustomize-mcp naming scheme).
type Metadata struct {
	SourcePath string  `json:"source_path"`
	APIVersion string  `json:"api_version"`
	Kind       string  `json:"kind"`
	Name       string  `json:"name"`
	Namespace  *string `json:"namespace,omitempty"`
}

// ToFilename encodes metadata into a single filesystem-safe filename.
func (m Metadata) ToFilename() string {
	ns := ""
	if m.Namespace != nil {
		ns = *m.Namespace
	}
	parts := []string{
		strings.ReplaceAll(m.APIVersion, "/", "#"),
		m.Kind,
		ns,
		m.Name,
	}
	return strings.Join(parts, "+") + ".yaml"
}

// FromYAMLDoc parses the first Kubernetes-like document from YAML bytes.
func FromYAMLDoc(sourcePath string, doc []byte) (Metadata, error) {
	var root map[string]any
	if err := yaml.Unmarshal(doc, &root); err != nil {
		return Metadata{}, err
	}
	return FromMap(sourcePath, root)
}

// FromMap builds metadata from a decoded manifest map.
func FromMap(sourcePath string, manifest map[string]any) (Metadata, error) {
	apiVersion, _ := manifest["apiVersion"].(string)
	kind, _ := manifest["kind"].(string)
	meta, _ := manifest["metadata"].(map[string]any)
	name, _ := meta["name"].(string)
	var nsPtr *string
	if meta != nil {
		if ns, ok := meta["namespace"].(string); ok && ns != "" {
			nsPtr = &ns
		}
	}
	return Metadata{
		SourcePath: sourcePath,
		APIVersion: apiVersion,
		Kind:       kind,
		Name:       name,
		Namespace:  nsPtr,
	}, nil
}

// FromRelPath parses Metadata from a relative path to a rendered manifest file.
func FromRelPath(rel string) (Metadata, error) {
	if !strings.HasSuffix(rel, ".yaml") {
		return Metadata{}, fmt.Errorf("filename must end with .yaml")
	}
	dir := filepath.ToSlash(filepath.Dir(rel))
	base := filepath.Base(rel)
	base = strings.TrimSuffix(base, ".yaml")
	parts := strings.Split(base, "+")
	if len(parts) != 4 {
		return Metadata{}, fmt.Errorf("unexpected manifest filename: %s", base)
	}
	apiVersion := strings.ReplaceAll(parts[0], "#", "/")
	kind := parts[1]
	var nsPtr *string
	if parts[2] != "" {
		ns := parts[2]
		nsPtr = &ns
	}
	name := parts[3]
	return Metadata{
		SourcePath: dir,
		APIVersion: apiVersion,
		Kind:       kind,
		Name:       name,
		Namespace:  nsPtr,
	}, nil
}
