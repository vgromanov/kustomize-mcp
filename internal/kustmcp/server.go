package kustmcp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vgromanov/kustomize-mcp/internal/deps"
	"github.com/vgromanov/kustomize-mcp/internal/diff"
	"github.com/vgromanov/kustomize-mcp/internal/filter"
	"github.com/vgromanov/kustomize-mcp/internal/inventory"
	"github.com/vgromanov/kustomize-mcp/internal/manifest"
	"github.com/vgromanov/kustomize-mcp/internal/render"
	"github.com/vgromanov/kustomize-mcp/internal/trace"
)

// Server ties checkpoints, rendering, dependency resolution, and directory diffs.
type Server struct {
	rootDir  string
	diffsDir string
	renderer *render.Renderer
	resolver *deps.Resolver
}

// NewServer builds a server rooted at rootDir (typically the process working directory).
func NewServer(rootDir string, loadRestrictions, helmEnabled bool) (*Server, error) {
	abs, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, err
	}
	rnd, err := render.NewRenderer(abs, loadRestrictions, helmEnabled)
	if err != nil {
		return nil, err
	}
	res, err := deps.NewResolver(abs)
	if err != nil {
		return nil, err
	}
	diffs := filepath.Join(abs, ".kustomize-mcp", "diffs")
	if err := os.MkdirAll(diffs, 0o700); err != nil {
		return nil, err
	}
	return &Server{
		rootDir:  abs,
		diffsDir: diffs,
		renderer: rnd,
		resolver: res,
	}, nil
}

// CreateCheckpoint creates an empty checkpoint.
func (s *Server) CreateCheckpoint() (string, error) {
	return s.renderer.CreateCheckpoint()
}

// ClearCheckpoint clears all checkpoints or one by id.
func (s *Server) ClearCheckpoint(checkpointID *string) error {
	return s.renderer.Clear(checkpointID)
}

// Render builds Kustomize output into a checkpoint.
func (s *Server) Render(checkpointID, path string) (string, error) {
	return s.renderer.Render(checkpointID, path)
}

// DiffCheckpoints compares two checkpoint trees.
func (s *Server) DiffCheckpoints(checkpointID1, checkpointID2 string) (*diff.Result, error) {
	if err := s.ensureCheckpointRendered(checkpointID1, nil); err != nil {
		return nil, err
	}
	if err := s.ensureCheckpointRendered(checkpointID2, nil); err != nil {
		return nil, err
	}
	left := filepath.Join(s.renderer.CheckpointsDir(), checkpointID1)
	right := filepath.Join(s.renderer.CheckpointsDir(), checkpointID2)
	workDir, err := os.MkdirTemp(s.diffsDir, "diff-")
	if err != nil {
		return nil, err
	}
	return diff.Dir(left, right, workDir, s.rootDir)
}

// DiffPaths compares two rendered Kustomize roots inside the same checkpoint.
func (s *Server) DiffPaths(checkpointID, path1, path2 string) (*diff.Result, error) {
	if err := s.ensureCheckpointRendered(checkpointID, &path1); err != nil {
		return nil, err
	}
	if err := s.ensureCheckpointRendered(checkpointID, &path2); err != nil {
		return nil, err
	}
	ck := filepath.Join(s.renderer.CheckpointsDir(), checkpointID)
	left := filepath.Join(ck, filepath.FromSlash(path1))
	right := filepath.Join(ck, filepath.FromSlash(path2))
	workDir, err := os.MkdirTemp(s.diffsDir, "diff-")
	if err != nil {
		return nil, err
	}
	return diff.Dir(left, right, workDir, s.rootDir)
}

// Inventory returns the resource tree for a checkpoint, optionally filtered.
func (s *Server) Inventory(checkpointID string, path *string, f *filter.ResourceFilter) (*manifest.ResourceTree, error) {
	if err := s.ensureCheckpointRendered(checkpointID, path); err != nil {
		return nil, err
	}
	ck := filepath.Join(s.renderer.CheckpointsDir(), checkpointID)
	tree, err := inventory.Load(ck, path)
	if err != nil {
		return nil, err
	}
	return inventory.Filter(tree, f), nil
}

// Trace returns the origin and transformations for a specific resource in a checkpoint.
func (s *Server) Trace(checkpointID, path, kind, name string, namespace *string) (*trace.TraceResult, error) {
	if err := s.ensureCheckpointRendered(checkpointID, &path); err != nil {
		return nil, err
	}
	ck := filepath.Join(s.renderer.CheckpointsDir(), checkpointID)
	dir := filepath.Join(ck, filepath.FromSlash(path))
	tree, err := manifest.ReadTree(dir)
	if err != nil {
		return nil, err
	}
	return trace.Lookup(tree, kind, name, namespace)
}

// Dependencies lists forward or reverse Kustomization dependencies (relative paths).
func (s *Server) Dependencies(path string, recursive, reverse bool) ([]string, error) {
	if err := validateRelPath(path); err != nil {
		return nil, err
	}
	return s.resolver.ComputeDependencies(filepath.ToSlash(path), recursive, reverse)
}

func (s *Server) ensureCheckpointRendered(checkpointID string, path *string) error {
	ck := filepath.Join(s.renderer.CheckpointsDir(), checkpointID)
	st, err := os.Stat(ck)
	if err != nil || !st.IsDir() {
		return fmt.Errorf("checkpoint %s does not exist", checkpointID)
	}
	if path == nil {
		entries, err := os.ReadDir(ck)
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			return fmt.Errorf("no paths have been rendered in checkpoint %s", checkpointID)
		}
		return nil
	}
	sub := filepath.Join(ck, filepath.FromSlash(*path))
	if _, err := os.Stat(sub); os.IsNotExist(err) {
		_, err = s.renderer.Render(checkpointID, *path)
		return err
	} else if err != nil {
		return err
	}
	return nil
}

func validateRelPath(p string) error {
	if p == "" || filepath.IsAbs(p) {
		return fmt.Errorf("path must be non-empty and relative")
	}
	for _, seg := range strings.Split(filepath.ToSlash(p), "/") {
		if seg == ".." {
			return fmt.Errorf("path must not traverse upward")
		}
	}
	return nil
}
