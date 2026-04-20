package kustmcp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vgromanov/kustomize-mcp/internal/flux"
	"github.com/vgromanov/kustomize-mcp/internal/inventory"
)

// RenderRecursiveResult summarizes a recursive render through Flux Kustomizations.
type RenderRecursiveResult struct {
	RootPath           string   `json:"root_path" jsonschema:"workspace-relative Kustomize root that was rendered first"`
	RenderedPaths      []string `json:"rendered_paths,omitempty" jsonschema:"workspace-relative output directories written under the checkpoint"`
	FluxKustomizations []string `json:"flux_kustomizations,omitempty" jsonschema:"flux kustomizations reconciled as namespace/name"`
	Conflicts          int      `json:"conflicts,omitempty" jsonschema:"number of resource identity conflicts across flux subtrees"`
	Warnings           []string `json:"warnings,omitempty" jsonschema:"non-fatal issues such as missing spec.path or cycles"`
}

// RenderRecursive renders rootPath, discovers Flux Kustomization CRDs in rendered output,
// renders each local spec.path target, and repeats until no new Flux Kustomizations appear.
func (s *Server) RenderRecursive(checkpointID, rootPath string) (*RenderRecursiveResult, error) {
	rootOut, err := s.renderer.Render(checkpointID, rootPath)
	if err != nil {
		return nil, err
	}
	ck := filepath.Join(s.renderer.CheckpointsDir(), checkpointID)
	rootAbs := filepath.Join(ck, filepath.FromSlash(rootPath))

	var queue []string
	queue = append(queue, rootAbs)
	scanned := make(map[string]bool)
	seenFlux := make(map[string]bool)

	var rendered []string
	rendered = append(rendered, rootOut)
	var fluxKeys []string
	var warnings []string

	for len(queue) > 0 {
		d := queue[0]
		queue = queue[1:]
		if scanned[d] {
			continue
		}
		scanned[d] = true

		specs, err := flux.ScanRenderedDir(d)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("scan %s: %v", d, err))
			continue
		}
		for _, spec := range specs {
			k := spec.Key()
			if seenFlux[k] {
				warnings = append(warnings, fmt.Sprintf("cycle or duplicate flux kustomization %s skipped", k))
				continue
			}
			if spec.Path == "" {
				warnings = append(warnings, fmt.Sprintf(
					"flux kustomization %s has empty path (spec.path=%q annotation=%q)",
					k, spec.SpecPath, spec.PathAnnotation))
				continue
			}
			src := filepath.Join(s.rootDir, filepath.FromSlash(spec.Path))
			if st, err := os.Stat(src); err != nil || !st.IsDir() {
				warnings = append(warnings, fmt.Sprintf("flux kustomization %s path %s is not a directory", k, spec.Path))
				continue
			}

			nested := filepath.Join(ck, filepath.FromSlash(spec.Namespace), filepath.FromSlash(spec.Name))
			out, err := s.renderer.RenderFlux(checkpointID, spec)
			if err != nil {
				if strings.Contains(err.Error(), "has already been rendered") {
					seenFlux[k] = true
					queue = append(queue, nested)
					continue
				}
				warnings = append(warnings, fmt.Sprintf("flux kustomization %s: %v", k, err))
				continue
			}
			seenFlux[k] = true
			fluxKeys = append(fluxKeys, k)
			rendered = append(rendered, out)
			queue = append(queue, nested)
		}
	}

	merged, err := inventory.Load(ck, nil)
	if err != nil {
		return nil, err
	}
	return &RenderRecursiveResult{
		RootPath:           rootPath,
		RenderedPaths:      rendered,
		FluxKustomizations: fluxKeys,
		Conflicts:          len(merged.Conflicts),
		Warnings:           warnings,
	}, nil
}
