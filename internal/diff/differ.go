package diff

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pmezard/go-difflib/difflib"

	"github.com/vgromanov/kustomize-mcp/internal/manifest"
)

// Result mirrors mbrt/kustomize-mcp DiffResult JSON shape.
type Result struct {
	Added    []manifest.Metadata `json:"added"`
	Deleted  []manifest.Metadata `json:"deleted"`
	Modified []manifest.Metadata `json:"modified"`
	Replaced []manifest.Metadata `json:"replaced"`
	DiffPath *string             `json:"diff_path,omitempty"`
}

// Dir compares two on-disk directories and writes a unified diff file into workDir.
// Manifest summaries use paths relative to each comparison root (same layout as rendered checkpoints).
func Dir(leftRoot, rightRoot, workDir, workspaceRoot string) (*Result, error) {
	if err := os.MkdirAll(workDir, 0o700); err != nil {
		return nil, err
	}
	leftFiles, err := readTree(leftRoot)
	if err != nil {
		return nil, err
	}
	rightFiles, err := readTree(rightRoot)
	if err != nil {
		return nil, err
	}
	var keys []string
	for k := range leftFiles {
		keys = append(keys, k)
	}
	for k := range rightFiles {
		if _, ok := leftFiles[k]; !ok {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)

	res := &Result{Replaced: []manifest.Metadata{}}
	var diffBuf bytes.Buffer

	for _, rel := range keys {
		lb, lok := leftFiles[rel]
		rb, rok := rightFiles[rel]
		switch {
		case lok && !rok:
			meta, err := manifest.FromRelPath(rel)
			if err != nil {
				return nil, err
			}
			res.Deleted = append(res.Deleted, meta)
		case !lok && rok:
			meta, err := manifest.FromRelPath(rel)
			if err != nil {
				return nil, err
			}
			res.Added = append(res.Added, meta)
		case lok && rok && bytes.Equal(lb, rb):
			// unchanged
		case lok && rok:
			meta, err := manifest.FromRelPath(rel)
			if err != nil {
				return nil, err
			}
			res.Modified = append(res.Modified, meta)
			_, _ = fmt.Fprintf(&diffBuf, "diff --git a/%s b/%s\n", rel, rel)
			ud := difflib.UnifiedDiff{
				A:        difflib.SplitLines(string(lb)),
				B:        difflib.SplitLines(string(rb)),
				FromFile: "a/" + rel,
				ToFile:   "b/" + rel,
				Context:  3,
			}
			text, err := difflib.GetUnifiedDiffString(ud)
			if err != nil {
				return nil, err
			}
			_, _ = diffBuf.WriteString(text)
			if !strings.HasSuffix(text, "\n") && text != "" {
				_, _ = diffBuf.WriteString("\n")
			}
		}
	}

	diffPath := filepath.Join(workDir, "changes.diff")
	if diffBuf.Len() == 0 {
		_, _ = diffBuf.WriteString("# no content differences in files present in both sides\n")
	}
	if err := os.WriteFile(diffPath, diffBuf.Bytes(), 0o600); err != nil {
		return nil, err
	}
	relOut, err := filepath.Rel(workspaceRoot, diffPath)
	if err != nil {
		s := filepath.ToSlash(diffPath)
		res.DiffPath = &s
	} else {
		s := filepath.ToSlash(relOut)
		res.DiffPath = &s
	}
	return res, nil
}

func readTree(root string) (map[string][]byte, error) {
	out := make(map[string][]byte)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".yaml") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[rel] = b
		return nil
	})
	return out, err
}
