package render

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/resource"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"

	"github.com/vgromanov/kustomize-mcp/internal/flux"
	"github.com/vgromanov/kustomize-mcp/internal/manifest"
)

const outputDir = ".kustomize-mcp"

// Renderer builds Kustomize output into checkpoint directories using krusty (no kustomize CLI).
type Renderer struct {
	rootDir          string
	checkpointsDir   string
	loadRestrictions bool
	helmEnabled      bool
}

// NewRenderer constructs a renderer; checkpoints live under rootDir/.kustomize-mcp/checkpoints.
func NewRenderer(rootDir string, loadRestrictions, helmEnabled bool) (*Renderer, error) {
	abs, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, err
	}
	r := &Renderer{
		rootDir:          abs,
		checkpointsDir:   filepath.Join(abs, outputDir, "checkpoints"),
		loadRestrictions: loadRestrictions,
		helmEnabled:      helmEnabled,
	}
	if err := os.MkdirAll(r.checkpointsDir, 0o700); err != nil {
		return nil, err
	}
	ignoreDir := filepath.Join(abs, outputDir)
	if err := os.MkdirAll(ignoreDir, 0o700); err != nil {
		return nil, err
	}
	_ = os.WriteFile(filepath.Join(ignoreDir, ".gitignore"), []byte("*\n"), 0o600)
	return r, nil
}

// CheckpointsDir returns the absolute checkpoints base path.
func (r *Renderer) CheckpointsDir() string { return r.checkpointsDir }

// Clear removes all checkpoints or deletes one by id.
func (r *Renderer) Clear(checkpointID *string) error {
	if checkpointID == nil {
		if err := os.RemoveAll(r.checkpointsDir); err != nil {
			return err
		}
		return os.MkdirAll(r.checkpointsDir, 0o700)
	}
	id := *checkpointID
	if err := validateCheckpointID(id); err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(r.checkpointsDir, id))
}

// CreateCheckpoint creates an empty checkpoint directory and returns its id.
func (r *Renderer) CreateCheckpoint() (string, error) {
	d, err := os.MkdirTemp(r.checkpointsDir, "ckp-")
	if err != nil {
		return "", err
	}
	return filepath.Base(d), nil
}

// Render runs Kustomize for relKustomPath (relative to workspace) into
// checkpoints/checkpointID/relKustomPath. It transparently injects
// buildMetadata to capture origin and transformer annotations, extracts
// them into a _tree.json sidecar, and strips any annotations that the
// user did not explicitly request.
func (r *Renderer) Render(checkpointID, relKustomPath string) (string, error) {
	if err := validateCheckpointID(checkpointID); err != nil {
		return "", err
	}
	if err := validateRelPath(relKustomPath); err != nil {
		return "", err
	}
	src := filepath.Join(r.rootDir, filepath.FromSlash(relKustomPath))
	if st, err := os.Stat(src); err != nil || !st.IsDir() {
		if err != nil {
			return "", err
		}
		return "", fmt.Errorf("path %s is not a directory (Kustomize root)", relKustomPath)
	}
	dest := filepath.Join(r.checkpointsDir, checkpointID, filepath.FromSlash(relKustomPath))
	if _, err := os.Stat(dest); err == nil {
		return "", fmt.Errorf("path %s has already been rendered in checkpoint %s; create a new checkpoint", relKustomPath, checkpointID)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := os.MkdirAll(dest, 0o700); err != nil {
		return "", err
	}

	diskFS := filesys.MakeFsOnDisk()
	userOrigin, userTransformer := userBuildMetadata(diskFS, src)

	opts := krusty.MakeDefaultOptions()
	if r.loadRestrictions {
		opts.LoadRestrictions = types.LoadRestrictionsRootOnly
	} else {
		opts.LoadRestrictions = types.LoadRestrictionsNone
	}
	if r.helmEnabled {
		opts.PluginConfig = types.EnabledPluginConfig(types.BploUseStaticallyLinked)
	} else {
		opts.PluginConfig = types.DisabledPluginConfig()
	}
	k := krusty.MakeKustomizer(opts)
	fSys := newAnnotatingFS(diskFS)
	m, err := k.Run(fSys, src)
	if err != nil {
		_ = os.RemoveAll(dest)
		return "", err
	}

	tree := &manifest.ResourceTree{Path: relKustomPath}

	for _, res := range m.Resources() {
		entry, err := extractEntry(res, relKustomPath)
		if err != nil {
			_ = os.RemoveAll(dest)
			return "", err
		}
		tree.Resources = append(tree.Resources, entry)

		stripInjectedAnnotations(res, userOrigin, userTransformer)

		doc, err := res.AsYAML()
		if err != nil {
			_ = os.RemoveAll(dest)
			return "", err
		}
		meta, err := manifest.FromYAMLDoc(relKustomPath, doc)
		if err != nil {
			_ = os.RemoveAll(dest)
			return "", err
		}
		outPath := filepath.Join(dest, meta.ToFilename())
		if err := os.WriteFile(outPath, doc, 0o600); err != nil {
			_ = os.RemoveAll(dest)
			return "", err
		}
	}

	if err := manifest.WriteTree(dest, tree); err != nil {
		_ = os.RemoveAll(dest)
		return "", err
	}

	relOut, err := filepath.Rel(r.rootDir, dest)
	if err != nil {
		return filepath.ToSlash(dest), nil
	}
	return filepath.ToSlash(relOut), nil
}

// RenderFlux builds the Kustomize directory at spec.Path (effective workspace-relative path,
// typically spec.path or the kustomize.toolkit.fluxcd.io/kustomization-path annotation) with Flux Kustomization
// spec fields merged into the root kustomization (in-memory), writes output to
// checkpoints/checkpointID/<namespace>/<name>/, and records flux provenance in _tree.json.
func (r *Renderer) RenderFlux(checkpointID string, spec flux.FluxKustomizationSpec) (string, error) {
	if err := validateCheckpointID(checkpointID); err != nil {
		return "", err
	}
	if err := validateFluxSegment(spec.Namespace); err != nil {
		return "", fmt.Errorf("invalid flux kustomization namespace: %w", err)
	}
	if err := validateFluxSegment(spec.Name); err != nil {
		return "", fmt.Errorf("invalid flux kustomization name: %w", err)
	}
	if err := validateRelPath(spec.Path); err != nil {
		return "", err
	}
	fluxRel := filepath.ToSlash(filepath.Join(spec.Namespace, spec.Name))
	dest := filepath.Join(r.checkpointsDir, checkpointID, filepath.FromSlash(fluxRel))
	if _, err := os.Stat(dest); err == nil {
		return "", fmt.Errorf("path %s has already been rendered in checkpoint %s; create a new checkpoint", fluxRel, checkpointID)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	src := filepath.Join(r.rootDir, filepath.FromSlash(spec.Path))
	if st, err := os.Stat(src); err != nil || !st.IsDir() {
		if err != nil {
			return "", err
		}
		return "", fmt.Errorf("flux kustomize path %s is not a directory (Kustomize root)", spec.Path)
	}
	if err := os.MkdirAll(dest, 0o700); err != nil {
		return "", err
	}

	srcAbs, err := filepath.Abs(src)
	if err != nil {
		_ = os.RemoveAll(dest)
		return "", err
	}

	diskFS := filesys.MakeFsOnDisk()
	userOrigin, userTransformer := userBuildMetadata(diskFS, src)

	opts := krusty.MakeDefaultOptions()
	if r.loadRestrictions {
		opts.LoadRestrictions = types.LoadRestrictionsRootOnly
	} else {
		opts.LoadRestrictions = types.LoadRestrictionsNone
	}
	if r.helmEnabled {
		opts.PluginConfig = types.EnabledPluginConfig(types.BploUseStaticallyLinked)
	} else {
		opts.PluginConfig = types.DisabledPluginConfig()
	}
	k := krusty.MakeKustomizer(opts)
	fSys := newFluxAnnotatingFS(diskFS, srcAbs, spec)
	m, err := k.Run(fSys, src)
	if err != nil {
		_ = os.RemoveAll(dest)
		return "", err
	}

	origin := &manifest.FluxOrigin{
		Name:      spec.Name,
		Namespace: spec.Namespace,
		SpecPath:  spec.Path,
	}
	if spec.PathAnnotation != "" && spec.SpecPath != spec.Path {
		origin.DeclaredPath = spec.SpecPath
	}
	tree := &manifest.ResourceTree{
		Path:              fluxRel,
		FluxKustomization: origin,
	}

	for _, res := range m.Resources() {
		entry, err := extractEntry(res, fluxRel)
		if err != nil {
			_ = os.RemoveAll(dest)
			return "", err
		}
		entry.FluxKustomization = origin
		tree.Resources = append(tree.Resources, entry)

		stripInjectedAnnotations(res, userOrigin, userTransformer)

		doc, err := res.AsYAML()
		if err != nil {
			_ = os.RemoveAll(dest)
			return "", err
		}
		doc, err = applyCommonMetadataYAML(doc, spec.CommonMetadata)
		if err != nil {
			_ = os.RemoveAll(dest)
			return "", err
		}
		meta, err := manifest.FromYAMLDoc(fluxRel, doc)
		if err != nil {
			_ = os.RemoveAll(dest)
			return "", err
		}
		outPath := filepath.Join(dest, meta.ToFilename())
		if err := os.WriteFile(outPath, doc, 0o600); err != nil {
			_ = os.RemoveAll(dest)
			return "", err
		}
	}

	if err := manifest.WriteTree(dest, tree); err != nil {
		_ = os.RemoveAll(dest)
		return "", err
	}

	relOut, err := filepath.Rel(r.rootDir, dest)
	if err != nil {
		return filepath.ToSlash(dest), nil
	}
	return filepath.ToSlash(relOut), nil
}

func validateFluxSegment(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return fmt.Errorf("segment must be non-empty")
	}
	if strings.Contains(s, "/") || strings.Contains(s, string(filepath.Separator)) {
		return fmt.Errorf("segment must not contain path separators")
	}
	if strings.Contains(s, "..") {
		return fmt.Errorf("segment must not contain ..")
	}
	return nil
}

const (
	originAnnotationKey      = "config.kubernetes.io/origin"
	transformerAnnotationKey = "alpha.config.kubernetes.io/transformations"
)

func extractEntry(res *resource.Resource, relKustomPath string) (manifest.ResourceEntry, error) {
	meta := manifest.Metadata{
		SourcePath: relKustomPath,
		APIVersion: res.GetApiVersion(),
		Kind:       res.GetKind(),
		Name:       res.GetName(),
	}
	if ns := res.GetNamespace(); ns != "" {
		meta.Namespace = &ns
	}

	entry := manifest.ResourceEntry{Metadata: meta}

	origin, err := res.GetOrigin()
	if err == nil && origin != nil {
		entry.Origin = convertOrigin(origin, relKustomPath)
	}

	transformations, err := res.GetTransformations()
	if err == nil && len(transformations) > 0 {
		for _, tr := range transformations {
			entry.Transformations = append(entry.Transformations, *convertOrigin(tr, relKustomPath))
		}
	}

	return entry, nil
}

func convertOrigin(o *resource.Origin, relKustomPath string) *manifest.ResourceOrigin {
	if o == nil {
		return nil
	}
	ro := &manifest.ResourceOrigin{
		Repo:         o.Repo,
		Ref:          o.Ref,
		ConfiguredIn: resolveOriginPath(o.ConfiguredIn, relKustomPath),
	}
	if o.Path != "" {
		ro.Path = resolveOriginPath(o.Path, relKustomPath)
	}
	cbID := o.ConfiguredBy
	if cbID.Kind != "" || cbID.Name != "" {
		ro.ConfiguredBy = &manifest.ResourceIdentifier{
			APIVersion: cbID.APIVersion,
			Kind:       cbID.Kind,
			Name:       cbID.Name,
			Namespace:  cbID.Namespace,
		}
	}
	return ro
}

// resolveOriginPath converts a path relative to the kustomize build root
// into a workspace-relative path.
func resolveOriginPath(p, relKustomPath string) string {
	if p == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Clean(filepath.Join(relKustomPath, p)))
}

func stripInjectedAnnotations(res *resource.Resource, userOrigin, userTransformer bool) {
	annotations := res.GetAnnotations()
	changed := false
	if !userOrigin {
		if _, ok := annotations[originAnnotationKey]; ok {
			delete(annotations, originAnnotationKey)
			changed = true
		}
	}
	if !userTransformer {
		if _, ok := annotations[transformerAnnotationKey]; ok {
			delete(annotations, transformerAnnotationKey)
			changed = true
		}
	}
	if changed {
		_ = res.SetAnnotations(annotations)
	}
}

func validateCheckpointID(id string) error {
	id = strings.TrimSpace(id)
	if id == "" || id == "." || id == ".." || strings.Contains(id, "/") || strings.Contains(id, string(filepath.Separator)) {
		return fmt.Errorf("invalid checkpoint id")
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
