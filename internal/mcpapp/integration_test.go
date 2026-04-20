package mcpapp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vgromanov/kustomize-mcp/internal/flux"
	"github.com/vgromanov/kustomize-mcp/internal/manifest"
	"github.com/vgromanov/kustomize-mcp/internal/version"
)

func TestIntegration_toolsAndPrompts(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "app")
	if err := os.MkdirAll(app, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "kustomization.yaml"), []byte(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- cm.yaml
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "cm.yaml"), []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: cfg
data:
  k: v
`), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("KUSTOMIZE_MCP_ROOT", root)
	t.Setenv("KUSTOMIZE_LOAD_RESTRICTIONS", "true")
	t.Setenv("KUSTOMIZE_ENABLE_HELM", "false")

	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()

	srv := mcp.NewServer(&mcp.Implementation{Name: version.Name, Version: version.Version}, &mcp.ServerOptions{
		Instructions: Instructions,
	})
	Register(srv, Options{LoadRestrictions: true, Helm: false})

	ss, err := srv.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ss.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0.0.1"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	ck1 := mustToolString(t, cs, ctx, "create_checkpoint", map[string]any{}, "checkpoint_id")
	ck2 := mustToolString(t, cs, ctx, "create_checkpoint", map[string]any{}, "checkpoint_id")
	if ck1 == "" || ck2 == "" {
		t.Fatalf("checkpoint ids: %q %q", ck1, ck2)
	}

	mustToolOK(t, cs, ctx, "render", map[string]any{
		"checkpoint_id": ck1,
		"path":          "app",
	})
	mustToolOK(t, cs, ctx, "render", map[string]any{
		"checkpoint_id": ck2,
		"path":          "app",
	})

	diffCk, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "diff_checkpoints",
		Arguments: map[string]any{
			"checkpoint_id_1": ck1,
			"checkpoint_id_2": ck2,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if diffCk.IsError {
		t.Fatalf("diff_checkpoints: %+v", diffCk.Content)
	}

	diffPaths, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "diff_paths",
		Arguments: map[string]any{
			"checkpoint_id": ck1,
			"path_1":        "app",
			"path_2":        "app",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if diffPaths.IsError {
		t.Fatalf("diff_paths identical: %+v", diffPaths.Content)
	}

	depsOut := mustToolStringSlice(t, cs, ctx, "dependencies", map[string]any{
		"path":    "app/kustomization.yaml",
		"reverse": false,
	})
	if len(depsOut) == 0 {
		t.Fatalf("expected dependencies, got %v", depsOut)
	}

	// inventory tool
	invRes, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "inventory",
		Arguments: map[string]any{
			"checkpoint_id": ck1,
			"path":          "app",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if invRes.IsError {
		t.Fatalf("inventory: %+v", invRes.Content)
	}
	var invOut struct {
		Total     int `json:"total"`
		Resources []struct {
			Metadata struct {
				Kind string `json:"kind"`
				Name string `json:"name"`
			} `json:"metadata"`
			Origin *struct {
				Path string `json:"path,omitempty"`
			} `json:"origin,omitempty"`
		} `json:"resources"`
	}
	if err := structuredToMap(invRes.StructuredContent, &invOut); err != nil {
		t.Fatalf("inventory structured: %v", err)
	}
	if invOut.Total != 1 {
		t.Fatalf("inventory: expected 1 resource, got %d", invOut.Total)
	}
	if invOut.Resources[0].Origin == nil {
		t.Fatal("inventory: expected origin metadata")
	}

	// inventory with filter (no match)
	invFiltered, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "inventory",
		Arguments: map[string]any{
			"checkpoint_id": ck1,
			"path":          "app",
			"filter":        map[string]any{"kind": "Deployment"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if invFiltered.IsError {
		t.Fatalf("inventory filtered: %+v", invFiltered.Content)
	}
	var invFilteredOut struct{ Total int }
	if err := structuredToMap(invFiltered.StructuredContent, &invFilteredOut); err != nil {
		t.Fatalf("inventory filtered structured: %v", err)
	}
	if invFilteredOut.Total != 0 {
		t.Fatalf("filtered inventory: expected 0 resources, got %d", invFilteredOut.Total)
	}

	// trace tool
	traceRes, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "trace",
		Arguments: map[string]any{
			"checkpoint_id": ck1,
			"path":          "app",
			"kind":          "ConfigMap",
			"name":          "cfg",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if traceRes.IsError {
		t.Fatalf("trace: %+v", traceRes.Content)
	}
	var traceOut struct {
		Resource struct {
			Kind string `json:"kind"`
			Name string `json:"name"`
		} `json:"resource"`
		Origin *struct{} `json:"origin"`
	}
	if err := structuredToMap(traceRes.StructuredContent, &traceOut); err != nil {
		t.Fatalf("trace structured: %v", err)
	}
	if traceOut.Resource.Kind != "ConfigMap" || traceOut.Resource.Name != "cfg" {
		t.Fatalf("trace: unexpected resource %s/%s", traceOut.Resource.Kind, traceOut.Resource.Name)
	}

	clearOne := ck1
	mustToolOK(t, cs, ctx, "clear_checkpoint", map[string]any{"checkpoint_id": clearOne})
	mustToolOK(t, cs, ctx, "clear_checkpoint", map[string]any{})

	pr, err := cs.GetPrompt(ctx, &mcp.GetPromptParams{
		Name: "explain",
		Arguments: map[string]string{
			"query": "what is kustomize?",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(pr.Messages) != 1 {
		t.Fatalf("explain messages: %d", len(pr.Messages))
	}

	pr2, err := cs.GetPrompt(ctx, &mcp.GetPromptParams{
		Name: "refactor",
		Arguments: map[string]string{
			"query": "split bases",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(pr2.Messages) != 1 {
		t.Fatalf("refactor messages: %d", len(pr2.Messages))
	}

	pr3, err := cs.GetPrompt(ctx, &mcp.GetPromptParams{
		Name: "diff_dirs",
		Arguments: map[string]string{
			"path_1": "a",
			"path_2": "b",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(pr3.Messages) != 2 {
		t.Fatalf("diff_dirs messages: %d", len(pr3.Messages))
	}

	prTroubleshoot, err := cs.GetPrompt(ctx, &mcp.GetPromptParams{
		Name: "troubleshoot",
		Arguments: map[string]string{
			"path": "app",
			"kind": "ConfigMap",
			"name": "cfg",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(prTroubleshoot.Messages) != 1 {
		t.Fatalf("troubleshoot messages: %d", len(prTroubleshoot.Messages))
	}

	prNilArgs, err := cs.GetPrompt(ctx, &mcp.GetPromptParams{Name: "explain"})
	if err != nil {
		t.Fatal(err)
	}
	if len(prNilArgs.Messages) != 1 {
		t.Fatalf("explain without args: %d msgs", len(prNilArgs.Messages))
	}

	ckBad := mustToolString(t, cs, ctx, "create_checkpoint", map[string]any{}, "checkpoint_id")
	badRender, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "render",
		Arguments: map[string]any{
			"checkpoint_id": ckBad,
			"path":          "this-path-does-not-exist",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !badRender.IsError {
		t.Fatal("expected tool error for missing kustomize path")
	}
}

func TestIntegration_fluxRecursive(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "clusters", "child")
	if err := os.MkdirAll(child, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(child, "kustomization.yaml"), []byte(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- cm.yaml
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(child, "cm.yaml"), []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: shared
  namespace: default
data:
  k: v
`), 0o600); err != nil {
		t.Fatal(err)
	}

	boot := filepath.Join(root, "clusters", "bootstrap")
	if err := os.MkdirAll(boot, 0o700); err != nil {
		t.Fatal(err)
	}
	flux1 := `apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: fk1
  namespace: ns-a
spec:
  path: clusters/child
`
	flux2 := `apiVersion: kustomize.toolkit.fluxcd.io/v1beta2
kind: Kustomization
metadata:
  name: fk2
  namespace: ns-b
spec:
  path: clusters/child
`
	if err := os.WriteFile(filepath.Join(boot, "kustomization.yaml"), []byte(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- fk1.yaml
- fk2.yaml
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(boot, "fk1.yaml"), []byte(flux1), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(boot, "fk2.yaml"), []byte(flux2), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("KUSTOMIZE_MCP_ROOT", root)
	t.Setenv("KUSTOMIZE_LOAD_RESTRICTIONS", "true")
	t.Setenv("KUSTOMIZE_ENABLE_HELM", "false")

	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()
	srv := mcp.NewServer(&mcp.Implementation{Name: version.Name, Version: version.Version}, &mcp.ServerOptions{
		Instructions: Instructions,
	})
	Register(srv, Options{LoadRestrictions: true, Helm: false})
	ss, err := srv.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0.0.1"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	ck := mustToolString(t, cs, ctx, "create_checkpoint", map[string]any{}, "checkpoint_id")
	ren, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "render",
		Arguments: map[string]any{
			"checkpoint_id": ck,
			"path":          "clusters/bootstrap",
			"recursive":     true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if ren.IsError {
		t.Fatalf("render recursive: %+v", ren.Content)
	}
	var rec struct {
		RootPath           string   `json:"root_path"`
		RenderedPaths      []string `json:"rendered_paths"`
		FluxKustomizations []string `json:"flux_kustomizations"`
		Conflicts          int      `json:"conflicts"`
	}
	if err := structuredToMap(ren.StructuredContent, &rec); err != nil {
		t.Fatal(err)
	}
	if rec.RootPath != "clusters/bootstrap" {
		t.Fatalf("root_path: %q", rec.RootPath)
	}
	if len(rec.RenderedPaths) < 3 {
		t.Fatalf("rendered_paths: %v", rec.RenderedPaths)
	}
	if len(rec.FluxKustomizations) != 2 {
		t.Fatalf("flux ks: %v", rec.FluxKustomizations)
	}
	if rec.Conflicts < 1 {
		t.Fatalf("expected conflicts, got %d", rec.Conflicts)
	}

	invRes, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "inventory",
		Arguments: map[string]any{"checkpoint_id": ck},
	})
	if err != nil {
		t.Fatal(err)
	}
	if invRes.IsError {
		t.Fatalf("inventory: %+v", invRes.Content)
	}
	var invOut struct {
		Total     int `json:"total"`
		Conflicts []struct {
			Resource struct {
				Kind string `json:"kind"`
				Name string `json:"name"`
			} `json:"resource"`
			Origins []struct {
				Namespace string `json:"namespace"`
				Name      string `json:"name"`
			} `json:"origins"`
		} `json:"conflicts"`
	}
	if err := structuredToMap(invRes.StructuredContent, &invOut); err != nil {
		t.Fatal(err)
	}
	if invOut.Total < 3 {
		t.Fatalf("inventory total: %d", invOut.Total)
	}
	if len(invOut.Conflicts) < 1 {
		t.Fatalf("expected conflicts in inventory, got %+v", invOut.Conflicts)
	}

	traceRes, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "trace",
		Arguments: map[string]any{
			"checkpoint_id": ck,
			"path":          "ns-a/fk1",
			"kind":          "ConfigMap",
			"name":          "shared",
			"namespace":     "default",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if traceRes.IsError {
		t.Fatalf("trace: %+v", traceRes.Content)
	}
}

func TestIntegration_fluxRecursive_pathAnnotation(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "deploy", "child")
	if err := os.MkdirAll(child, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(child, "kustomization.yaml"), []byte(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- cm.yaml
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(child, "cm.yaml"), []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: ann-cm
  namespace: default
data:
  k: v
`), 0o600); err != nil {
		t.Fatal(err)
	}

	boot := filepath.Join(root, "deploy", "boot")
	if err := os.MkdirAll(boot, 0o700); err != nil {
		t.Fatal(err)
	}
	fluxYAML := `apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: path-ann-ks
  namespace: flux-system
  annotations:
    ` + flux.PathAnnotation + `: deploy/child
spec:
  path: ./some/non/local/path
`
	if err := os.WriteFile(filepath.Join(boot, "kustomization.yaml"), []byte(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- fk.yaml
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(boot, "fk.yaml"), []byte(fluxYAML), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("KUSTOMIZE_MCP_ROOT", root)
	t.Setenv("KUSTOMIZE_LOAD_RESTRICTIONS", "true")
	t.Setenv("KUSTOMIZE_ENABLE_HELM", "false")

	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()
	srv := mcp.NewServer(&mcp.Implementation{Name: version.Name, Version: version.Version}, &mcp.ServerOptions{
		Instructions: Instructions,
	})
	Register(srv, Options{LoadRestrictions: true, Helm: false})
	ss, err := srv.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0.0.1"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	ck := mustToolString(t, cs, ctx, "create_checkpoint", map[string]any{}, "checkpoint_id")
	ren, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "render",
		Arguments: map[string]any{
			"checkpoint_id": ck,
			"path":          "deploy/boot",
			"recursive":     true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if ren.IsError {
		t.Fatalf("render recursive: %+v", ren.Content)
	}
	var rec struct {
		Warnings []string `json:"warnings"`
	}
	if err := structuredToMap(ren.StructuredContent, &rec); err != nil {
		t.Fatal(err)
	}
	for _, w := range rec.Warnings {
		t.Errorf("unexpected warning: %s", w)
	}

	treePath := filepath.Join(root, ".kustomize-mcp", "checkpoints", ck, "flux-system", "path-ann-ks")
	tree, err := manifest.ReadTree(treePath)
	if err != nil {
		t.Fatal(err)
	}
	if tree.FluxKustomization == nil {
		t.Fatal("missing flux_kustomization in _tree.json")
	}
	fk := tree.FluxKustomization
	if fk.SpecPath != "deploy/child" {
		t.Fatalf("spec_path want deploy/child, got %q", fk.SpecPath)
	}
	if fk.DeclaredPath != "./some/non/local/path" {
		t.Fatalf("declared_path want ./some/non/local/path, got %q", fk.DeclaredPath)
	}
	if _, err := os.Stat(filepath.Join(treePath, "v1+ConfigMap+default+ann-cm.yaml")); err != nil {
		t.Fatal(err)
	}
}

func mustToolString(t *testing.T, cs *mcp.ClientSession, ctx context.Context, name string, args map[string]any, key string) string {
	t.Helper()
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if res.IsError {
		t.Fatalf("%s error: %+v", name, res.Content)
	}
	var m map[string]any
	if err := structuredToMap(res.StructuredContent, &m); err != nil {
		t.Fatalf("%s structured: %v", name, err)
	}
	v, ok := m[key].(string)
	if !ok || v == "" {
		t.Fatalf("%s: want string %q, got %#v", name, key, m[key])
	}
	return v
}

func mustToolStringSlice(t *testing.T, cs *mcp.ClientSession, ctx context.Context, name string, args map[string]any) []string {
	t.Helper()
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if res.IsError {
		t.Fatalf("%s error: %+v", name, res.Content)
	}
	var out struct {
		Paths []string `json:"paths"`
	}
	if err := structuredToMap(res.StructuredContent, &out); err != nil {
		t.Fatalf("%s structured: %v", name, err)
	}
	return out.Paths
}

func mustToolOK(t *testing.T, cs *mcp.ClientSession, ctx context.Context, name string, args map[string]any) {
	t.Helper()
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if res.IsError {
		t.Fatalf("%s error: %+v", name, res.Content)
	}
}

func structuredToMap(src any, dst any) error {
	if src == nil {
		return nil
	}
	b, err := json.Marshal(src)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}
