package mcpapp

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vgromanov/kustomize-mcp/internal/diff"
	"github.com/vgromanov/kustomize-mcp/internal/filter"
	"github.com/vgromanov/kustomize-mcp/internal/kustmcp"
	"github.com/vgromanov/kustomize-mcp/internal/manifest"
	"github.com/vgromanov/kustomize-mcp/internal/prompts"
	"github.com/vgromanov/kustomize-mcp/internal/trace"
	"github.com/vgromanov/kustomize-mcp/internal/workspace"
)

// Options configures tool behavior (mirrors environment flags).
type Options struct {
	LoadRestrictions bool
	Helm             bool
}

// createCheckpointOut and dependenciesOut must be JSON objects for MCP output schemas.
type createCheckpointOut struct {
	CheckpointID string `json:"checkpoint_id" jsonschema:"id of the new checkpoint directory"`
}

type dependenciesOut struct {
	Paths []string `json:"paths" jsonschema:"dependency paths relative to the effective root (MCP workspace, or project subdirectory when project is set)"`
}

// Register adds tools and prompts to the MCP server.
func Register(server *mcp.Server, opts Options) {
	serverFor := func(ctx context.Context, req *mcp.CallToolRequest, project *string) (*kustmcp.Server, error) {
		p := ""
		if project != nil {
			p = strings.TrimSpace(*project)
		}
		var root string
		var err error
		if p == "" {
			root, err = workspace.Dir(ctx, req.Session)
		} else if filepath.IsAbs(p) {
			root, err = workspace.ResolveAbsProject(ctx, req.Session, p)
		} else {
			for _, seg := range strings.Split(filepath.ToSlash(p), "/") {
				if seg == ".." {
					return nil, fmt.Errorf("project path must not traverse upward")
				}
			}
			root, err = workspace.ResolveProject(ctx, req.Session, p)
		}
		if err != nil {
			return nil, err
		}
		return kustmcp.NewServer(root, opts.LoadRestrictions, opts.Helm)
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_checkpoint",
		Description: "Creates an empty checkpoint for storing rendered Kustomize output.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct {
		Project *string `json:"project,omitempty" jsonschema:"optional; scopes effective root and checkpoints. Relative path (resolved across MCP roots) or absolute directory equal to or under a workspace root; in multi-root workspaces prefer the absolute path from roots/list. Same value on create_checkpoint, render, inventory, trace, diff, clear, dependencies"`
	}) (*mcp.CallToolResult, createCheckpointOut, error) {
		srv, err := serverFor(ctx, req, args.Project)
		if err != nil {
			return nil, createCheckpointOut{}, err
		}
		id, err := srv.CreateCheckpoint()
		if err != nil {
			return nil, createCheckpointOut{}, err
		}
		return nil, createCheckpointOut{CheckpointID: id}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "clear_checkpoint",
		Description: "Clears all checkpoints, or a single checkpoint when checkpoint_id is set.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct {
		CheckpointID *string `json:"checkpoint_id,omitempty" jsonschema:"checkpoint to remove; omit to clear all"`
		Project      *string `json:"project,omitempty" jsonschema:"optional; scopes effective root and checkpoints. Relative path (resolved across MCP roots) or absolute directory equal to or under a workspace root; in multi-root workspaces prefer the absolute path from roots/list. Same value on create_checkpoint, render, inventory, trace, diff, clear, dependencies"`
	}) (*mcp.CallToolResult, map[string]string, error) {
		srv, err := serverFor(ctx, req, args.Project)
		if err != nil {
			return nil, nil, err
		}
		if err := srv.ClearCheckpoint(args.CheckpointID); err != nil {
			return nil, nil, err
		}
		msg := "cleared all checkpoints"
		if args.CheckpointID != nil {
			msg = "cleared checkpoint " + *args.CheckpointID
		}
		return nil, map[string]string{"status": "ok", "message": msg}, nil
	})

	// renderResult is the structured output for the render tool (flat or recursive).
	type renderResult struct {
		Path               string   `json:"path,omitempty" jsonschema:"workspace-relative rendered output directory when recursive is false"`
		RootPath           string   `json:"root_path,omitempty" jsonschema:"Kustomize root when recursive is true"`
		RenderedPaths      []string `json:"rendered_paths,omitempty" jsonschema:"all rendered output dirs under the checkpoint when recursive is true"`
		FluxKustomizations []string `json:"flux_kustomizations,omitempty" jsonschema:"flux kustomizations reconciled as namespace/name when recursive is true"`
		Conflicts          int      `json:"conflicts,omitempty" jsonschema:"resource identity conflicts when recursive is true"`
		Warnings           []string `json:"warnings,omitempty" jsonschema:"non-fatal issues when recursive is true"`
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "render",
		Description: "Renders the Kustomize directory at path (relative to workspace) into a checkpoint. When recursive is true, also renders Flux Kustomization targets referenced by spec.path (or the kustomize.toolkit.fluxcd.io/kustomization-path annotation when set) and merges inventory across subtrees.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct {
		CheckpointID string  `json:"checkpoint_id" jsonschema:"checkpoint directory name"`
		Path         string  `json:"path" jsonschema:"relative path to Kustomize root"`
		Recursive    bool    `json:"recursive,omitempty" jsonschema:"when true, follow Flux Kustomization CRDs and render nested paths"`
		Project      *string `json:"project,omitempty" jsonschema:"optional; scopes effective root and checkpoints. Relative path (resolved across MCP roots) or absolute directory equal to or under a workspace root; in multi-root workspaces prefer the absolute path from roots/list. Same value on create_checkpoint, render, inventory, trace, diff, clear, dependencies"`
	}) (*mcp.CallToolResult, renderResult, error) {
		srv, err := serverFor(ctx, req, args.Project)
		if err != nil {
			return nil, renderResult{}, err
		}
		if args.Recursive {
			res, err := srv.RenderRecursive(args.CheckpointID, args.Path)
			if err != nil {
				return nil, renderResult{}, err
			}
			return nil, renderResult{
				RootPath:           res.RootPath,
				RenderedPaths:      res.RenderedPaths,
				FluxKustomizations: res.FluxKustomizations,
				Conflicts:          res.Conflicts,
				Warnings:           res.Warnings,
			}, nil
		}
		loc, err := srv.Render(args.CheckpointID, args.Path)
		if err != nil {
			return nil, renderResult{}, err
		}
		return nil, renderResult{Path: loc}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "diff_checkpoints",
		Description: "Compares all rendered manifests between two checkpoints.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct {
		CheckpointID1 string  `json:"checkpoint_id_1"`
		CheckpointID2 string  `json:"checkpoint_id_2"`
		Project       *string `json:"project,omitempty" jsonschema:"optional; scopes effective root and checkpoints. Relative path (resolved across MCP roots) or absolute directory equal to or under a workspace root; in multi-root workspaces prefer the absolute path from roots/list. Same value on create_checkpoint, render, inventory, trace, diff, clear, dependencies"`
	}) (*mcp.CallToolResult, *diff.Result, error) {
		srv, err := serverFor(ctx, req, args.Project)
		if err != nil {
			return nil, nil, err
		}
		res, err := srv.DiffCheckpoints(args.CheckpointID1, args.CheckpointID2)
		if err != nil {
			return nil, nil, err
		}
		return nil, res, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "diff_paths",
		Description: "Compares two Kustomize roots rendered under the same checkpoint.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct {
		CheckpointID string  `json:"checkpoint_id"`
		Path1        string  `json:"path_1" jsonschema:"first rendered relative path"`
		Path2        string  `json:"path_2" jsonschema:"second rendered relative path"`
		Project      *string `json:"project,omitempty" jsonschema:"optional; scopes effective root and checkpoints. Relative path (resolved across MCP roots) or absolute directory equal to or under a workspace root; in multi-root workspaces prefer the absolute path from roots/list. Same value on create_checkpoint, render, inventory, trace, diff, clear, dependencies"`
	}) (*mcp.CallToolResult, *diff.Result, error) {
		srv, err := serverFor(ctx, req, args.Project)
		if err != nil {
			return nil, nil, err
		}
		res, err := srv.DiffPaths(args.CheckpointID, args.Path1, args.Path2)
		if err != nil {
			return nil, nil, err
		}
		return nil, res, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "dependencies",
		Description: "Lists file and Kustomization dependencies for a Kustomization file path.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct {
		Path      string  `json:"path" jsonschema:"relative path to kustomization.yaml (or file for reverse mode)"`
		Recursive bool    `json:"recursive,omitempty"`
		Reverse   bool    `json:"reverse,omitempty"`
		Project   *string `json:"project,omitempty" jsonschema:"optional; scopes effective root and checkpoints. Relative path (resolved across MCP roots) or absolute directory equal to or under a workspace root; in multi-root workspaces prefer the absolute path from roots/list. Same value on create_checkpoint, render, inventory, trace, diff, clear, dependencies"`
	}) (*mcp.CallToolResult, dependenciesOut, error) {
		srv, err := serverFor(ctx, req, args.Project)
		if err != nil {
			return nil, dependenciesOut{}, err
		}
		deps, err := srv.Dependencies(args.Path, args.Recursive, args.Reverse)
		if err != nil {
			return nil, dependenciesOut{}, err
		}
		return nil, dependenciesOut{Paths: deps}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "trace",
		Description: "Traces the origin and transformations of a specific resource in a rendered checkpoint.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct {
		CheckpointID string  `json:"checkpoint_id"`
		Path         string  `json:"path" jsonschema:"relative path to the rendered kustomize root"`
		Kind         string  `json:"kind" jsonschema:"Kubernetes resource kind"`
		Name         string  `json:"name" jsonschema:"Kubernetes resource name"`
		Namespace    *string `json:"namespace,omitempty" jsonschema:"Kubernetes resource namespace"`
		Project      *string `json:"project,omitempty" jsonschema:"optional; scopes effective root and checkpoints. Relative path (resolved across MCP roots) or absolute directory equal to or under a workspace root; in multi-root workspaces prefer the absolute path from roots/list. Same value on create_checkpoint, render, inventory, trace, diff, clear, dependencies"`
	}) (*mcp.CallToolResult, *trace.TraceResult, error) {
		srv, err := serverFor(ctx, req, args.Project)
		if err != nil {
			return nil, nil, err
		}
		res, err := srv.Trace(args.CheckpointID, args.Path, args.Kind, args.Name, args.Namespace)
		if err != nil {
			return nil, nil, err
		}
		return nil, res, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "inventory",
		Description: "Lists all rendered resources in a checkpoint with origin and transformer metadata.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct {
		CheckpointID string                 `json:"checkpoint_id"`
		Path         *string                `json:"path,omitempty" jsonschema:"narrow to a single rendered kustomize root"`
		Filter       *filter.ResourceFilter `json:"filter,omitempty" jsonschema:"filter resources by kind, api_version, namespace, or name"`
		Project      *string                `json:"project,omitempty" jsonschema:"optional; scopes effective root and checkpoints. Relative path (resolved across MCP roots) or absolute directory equal to or under a workspace root; in multi-root workspaces prefer the absolute path from roots/list. Same value on create_checkpoint, render, inventory, trace, diff, clear, dependencies"`
	}) (*mcp.CallToolResult, *manifest.ResourceTree, error) {
		srv, err := serverFor(ctx, req, args.Project)
		if err != nil {
			return nil, nil, err
		}
		tree, err := srv.Inventory(args.CheckpointID, args.Path, args.Filter)
		if err != nil {
			return nil, nil, err
		}
		return nil, tree, nil
	})

	registerKustomizePrompts(server)
}

func promptArg(req *mcp.GetPromptRequest, key string) string {
	if req.Params == nil || req.Params.Arguments == nil {
		return ""
	}
	return req.Params.Arguments[key]
}

func registerKustomizePrompts(server *mcp.Server) {
	user := mcp.Role("user")
	textMsg := func(s string) *mcp.PromptMessage {
		return &mcp.PromptMessage{Role: user, Content: &mcp.TextContent{Text: s}}
	}
	arg := func(name, description string) *mcp.PromptArgument {
		return &mcp.PromptArgument{Name: name, Description: description, Required: true}
	}

	server.AddPrompt(&mcp.Prompt{
		Name:        "explain",
		Description: "Ask for an explanation of the current project’s Kustomize layout or behavior.",
		Arguments:   []*mcp.PromptArgument{arg("query", "Question about the Kustomize configuration")},
	}, func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		q := promptArg(req, "query")
		return &mcp.GetPromptResult{
			Messages: []*mcp.PromptMessage{textMsg(prompts.ExplainBody(q))},
		}, nil
	})

	server.AddPrompt(&mcp.Prompt{
		Name:        "refactor",
		Description: "Ask for a Kustomize refactor guided by the available tools.",
		Arguments:   []*mcp.PromptArgument{arg("query", "What to change or achieve in the Kustomize tree")},
	}, func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		q := promptArg(req, "query")
		return &mcp.GetPromptResult{
			Messages: []*mcp.PromptMessage{textMsg(prompts.RefactorBody(q))},
		}, nil
	})

	server.AddPrompt(&mcp.Prompt{
		Name:        "troubleshoot",
		Description: "Troubleshoot a specific resource by tracing its origin and transformations.",
		Arguments: []*mcp.PromptArgument{
			arg("path", "Relative path to the Kustomize root"),
			arg("kind", "Kubernetes resource kind (e.g. Deployment)"),
			arg("name", "Kubernetes resource name"),
		},
	}, func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		p := promptArg(req, "path")
		k := promptArg(req, "kind")
		n := promptArg(req, "name")
		return &mcp.GetPromptResult{
			Messages: []*mcp.PromptMessage{textMsg(prompts.TroubleshootBody(p, k, n))},
		}, nil
	})

	server.AddPrompt(&mcp.Prompt{
		Name:        "diff_dirs",
		Description: "Compare two Kustomize directories using checkpoints and diff_paths.",
		Arguments: []*mcp.PromptArgument{
			arg("path_1", "Relative path to the first Kustomize root"),
			arg("path_2", "Relative path to the second Kustomize root"),
		},
	}, func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		p1 := promptArg(req, "path_1")
		p2 := promptArg(req, "path_2")
		m := prompts.DiffDirsMessages(p1, p2)
		return &mcp.GetPromptResult{
			Messages: []*mcp.PromptMessage{textMsg(m[0]), textMsg(m[1])},
		}, nil
	})
}
