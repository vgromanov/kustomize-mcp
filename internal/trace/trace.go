package trace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pmezard/go-difflib/difflib"

	"github.com/vgromanov/kustomize-mcp/internal/manifest"
)

// TraceResult describes a single resource's origin and the transformers
// that modified it, optionally including a field-level diff when
// comparing two rendered versions.
type TraceResult struct {
	Resource        manifest.Metadata         `json:"resource"`
	Origin          *manifest.ResourceOrigin  `json:"origin,omitempty"`
	Transformations []manifest.ResourceOrigin `json:"transformations,omitempty"`
	FieldDiff       *string                   `json:"field_diff,omitempty"`
}

// Lookup finds a resource in the tree and returns its trace (origin +
// transformations). This is the fast Tier-1 trace — no extra renders.
func Lookup(tree *manifest.ResourceTree, kind, name string, namespace *string) (*TraceResult, error) {
	entry, err := findEntry(tree, kind, name, namespace)
	if err != nil {
		return nil, err
	}
	return &TraceResult{
		Resource:        entry.Metadata,
		Origin:          entry.Origin,
		Transformations: entry.Transformations,
	}, nil
}

// Compare looks up a resource in two trees and produces a trace that
// includes a field-level unified diff between the two rendered versions.
func Compare(leftDir, rightDir string, leftTree, rightTree *manifest.ResourceTree, kind, name string, namespace *string) (*TraceResult, error) {
	leftEntry, err := findEntry(leftTree, kind, name, namespace)
	if err != nil {
		return nil, fmt.Errorf("left: %w", err)
	}
	rightEntry, err := findEntry(rightTree, kind, name, namespace)
	if err != nil {
		return nil, fmt.Errorf("right: %w", err)
	}

	filename := rightEntry.Metadata.ToFilename()
	leftYAML, err := os.ReadFile(filepath.Join(leftDir, filename))
	if err != nil {
		return nil, fmt.Errorf("reading left manifest: %w", err)
	}
	rightYAML, err := os.ReadFile(filepath.Join(rightDir, filename))
	if err != nil {
		return nil, fmt.Errorf("reading right manifest: %w", err)
	}

	var fieldDiff *string
	if string(leftYAML) != string(rightYAML) {
		ud := difflib.UnifiedDiff{
			A:        difflib.SplitLines(string(leftYAML)),
			B:        difflib.SplitLines(string(rightYAML)),
			FromFile: "before",
			ToFile:   "after",
			Context:  3,
		}
		text, err := difflib.GetUnifiedDiffString(ud)
		if err != nil {
			return nil, err
		}
		if !strings.HasSuffix(text, "\n") && text != "" {
			text += "\n"
		}
		fieldDiff = &text
	}

	result := &TraceResult{
		Resource: rightEntry.Metadata,
		Origin:   rightEntry.Origin,
	}
	if leftEntry.Origin != nil && rightEntry.Origin != nil &&
		leftEntry.Origin.Path != rightEntry.Origin.Path {
		result.Origin = rightEntry.Origin
	} else if rightEntry.Origin != nil {
		result.Origin = rightEntry.Origin
	}
	result.Transformations = rightEntry.Transformations
	result.FieldDiff = fieldDiff
	return result, nil
}

func findEntry(tree *manifest.ResourceTree, kind, name string, namespace *string) (*manifest.ResourceEntry, error) {
	ns := ""
	if namespace != nil {
		ns = *namespace
	}
	for i := range tree.Resources {
		e := &tree.Resources[i]
		entryNS := ""
		if e.Metadata.Namespace != nil {
			entryNS = *e.Metadata.Namespace
		}
		if e.Metadata.Kind == kind && e.Metadata.Name == name && entryNS == ns {
			return e, nil
		}
	}
	desc := kind + "/" + name
	if ns != "" {
		desc = kind + "/" + ns + "/" + name
	}
	return nil, fmt.Errorf("resource %s not found in tree for path %s", desc, tree.Path)
}
