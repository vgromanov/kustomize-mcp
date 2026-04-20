package deps

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// Resolver walks the workspace and resolves Kustomization dependency edges (pure YAML + paths).
type Resolver struct {
	root            string
	files           []string
	kustomizationAt map[string]string // directory -> path to kustomization file
	scanned         bool
}

// NewResolver prepares a resolver; the tree is indexed on first use (dependencies, etc.)
// so MCP startup does not walk huge workspace roots (e.g. $HOME).
func NewResolver(rootDir string) (*Resolver, error) {
	root, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, err
	}
	return &Resolver{root: root, kustomizationAt: make(map[string]string)}, nil
}

func (r *Resolver) ensureScanned() error {
	if r.scanned {
		return nil
	}
	if err := r.rescan(); err != nil {
		return err
	}
	r.scanned = true
	return nil
}

func isKustomizationFile(name string) bool {
	switch name {
	case "kustomization.yaml", "kustomization.yml", "Kustomization":
		return true
	default:
		return false
	}
}

func (r *Resolver) rescan() error {
	r.files = r.files[:0]
	clear(r.kustomizationAt)
	return filepath.WalkDir(r.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// macOS and other systems may deny traversal of e.g. ~/.Trash; do not fail MCP startup.
			if path == r.root {
				return err
			}
			perms := os.IsPermission(err) || errors.Is(err, fs.ErrPermission) ||
				strings.Contains(strings.ToLower(err.Error()), "not permitted")
			if perms {
				if d != nil && d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(r.root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		r.files = append(r.files, rel)
		if isKustomizationFile(d.Name()) {
			dir := pathDirSlash(rel)
			r.kustomizationAt[dir] = rel
		}
		return nil
	})
}

// KustomizationPaths returns paths to Kustomization files (relative to root).
func (r *Resolver) KustomizationPaths() []string {
	if err := r.ensureScanned(); err != nil {
		return nil
	}
	out := make([]string, 0, len(r.kustomizationAt))
	for _, p := range r.kustomizationAt {
		out = append(out, p)
	}
	slices.Sort(out)
	return out
}

// ComputeDependencies returns dependency paths relative to root.
func pathDirSlash(p string) string {
	p = filepath.ToSlash(p)
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[:i]
	}
	return "."
}

func (r *Resolver) ComputeDependencies(kustomizationFileRel string, recursive, reverse bool) ([]string, error) {
	if err := r.ensureScanned(); err != nil {
		return nil, err
	}
	if reverse {
		return r.reverseDeps(kustomizationFileRel, recursive)
	}
	path := filepath.ToSlash(filepath.Clean(kustomizationFileRel))
	full := filepath.Join(r.root, filepath.FromSlash(path))
	st, err := os.Stat(full)
	if err != nil {
		return nil, err
	}
	if st.IsDir() {
		return nil, fmt.Errorf("path %s is not a file", path)
	}
	k, err := r.parseKustomization(full)
	if err != nil {
		return nil, err
	}
	base := filepath.Dir(full)
	paths := r.collectPathsInKustomization(base, k)
	if recursive {
		visited := map[string]bool{full: true}
		paths = r.expandRecursive(paths, visited)
	}
	return r.toRelPaths(paths), nil
}

func (r *Resolver) parseKustomization(fullPath string) (map[string]any, error) {
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, err
	}
	var k map[string]any
	if err := yaml.Unmarshal(data, &k); err != nil {
		return nil, err
	}
	if k == nil {
		return nil, fmt.Errorf("file %s is not a valid Kustomization file", fullPath)
	}
	api, _ := k["apiVersion"].(string)
	kind, _ := k["kind"].(string)
	if !strings.HasPrefix(api, "kustomize.config.k8s.io/") || (kind != "Kustomization" && kind != "Component") {
		return nil, fmt.Errorf("file %s is not a valid Kustomization file", fullPath)
	}
	return k, nil
}

func (r *Resolver) fileSet() map[string]bool {
	m := make(map[string]bool, len(r.files))
	for _, f := range r.files {
		m[f] = true
	}
	return m
}

func (r *Resolver) collectPathsInKustomization(basePath string, item any) []string {
	fsSet := r.fileSet()
	var out []string
	var walk func(any)
	walk = func(v any) {
		switch t := v.(type) {
		case string:
			cand := filepath.Clean(filepath.Join(basePath, filepath.FromSlash(t)))
			rel, err := filepath.Rel(r.root, cand)
			if err != nil {
				return
			}
			rel = filepath.ToSlash(rel)
			if fsSet[rel] {
				out = append(out, filepath.Join(r.root, rel))
			}
			if kust, ok := r.kustomizationAt[rel]; ok {
				out = append(out, filepath.Join(r.root, kust))
			}
		case []any:
			for _, x := range t {
				walk(x)
			}
		case map[string]any:
			for k, val := range t {
				walk(k)
				walk(val)
			}
		}
	}
	walk(item)
	return dedupeAbs(out)
}

func dedupeAbs(paths []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, p := range paths {
		ap, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		if seen[ap] {
			continue
		}
		seen[ap] = true
		out = append(out, ap)
	}
	return out
}

func (r *Resolver) expandRecursive(paths []string, visited map[string]bool) []string {
	var all []string
	for _, p := range paths {
		all = append(all, p)
		base := filepath.Base(p)
		if !isKustomizationFile(base) {
			continue
		}
		ap, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		if visited[ap] {
			continue
		}
		visited[ap] = true
		k, err := r.parseKustomization(ap)
		if err != nil {
			continue
		}
		nested := r.collectPathsInKustomization(filepath.Dir(ap), k)
		all = append(all, r.expandRecursive(nested, visited)...)
	}
	return dedupeAbs(all)
}

func (r *Resolver) toRelPaths(abs []string) []string {
	out := make([]string, 0, len(abs))
	for _, p := range abs {
		rel, err := filepath.Rel(r.root, p)
		if err != nil {
			continue
		}
		out = append(out, filepath.ToSlash(rel))
	}
	slices.Sort(out)
	return out
}

func (r *Resolver) reverseDeps(pathRel string, recursive bool) ([]string, error) {
	pathRel = filepath.ToSlash(filepath.Clean(pathRel))
	target, err := filepath.Abs(filepath.Join(r.root, filepath.FromSlash(pathRel)))
	if err != nil {
		return nil, err
	}
	reverse := make(map[string]map[string]bool)
	for _, kustRel := range r.KustomizationPaths() {
		full := filepath.Join(r.root, kustRel)
		k, err := r.parseKustomization(full)
		if err != nil {
			continue
		}
		deps := r.collectPathsInKustomization(filepath.Dir(full), k)
		for _, d := range deps {
			ap, err := filepath.Abs(d)
			if err != nil {
				continue
			}
			if reverse[ap] == nil {
				reverse[ap] = make(map[string]bool)
			}
			kap, _ := filepath.Abs(full)
			reverse[ap][kap] = true
		}
	}
	direct := reverse[target]
	if direct == nil {
		return nil, nil
	}
	if !recursive {
		var out []string
		for k := range direct {
			rel, err := filepath.Rel(r.root, k)
			if err != nil {
				continue
			}
			out = append(out, filepath.ToSlash(rel))
		}
		slices.Sort(out)
		return out, nil
	}
	all := make(map[string]bool)
	visited := make(map[string]bool)
	var queue []string
	for k := range direct {
		queue = append(queue, k)
	}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if visited[cur] {
			continue
		}
		visited[cur] = true
		rel, err := filepath.Rel(r.root, cur)
		if err != nil {
			continue
		}
		all[filepath.ToSlash(rel)] = true
		for next := range reverse[cur] {
			queue = append(queue, next)
		}
	}
	out := make([]string, 0, len(all))
	for k := range all {
		out = append(out, k)
	}
	slices.Sort(out)
	return out, nil
}
