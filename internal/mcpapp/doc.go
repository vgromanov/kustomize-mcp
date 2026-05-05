package mcpapp

// Instructions is sent to MCP clients during initialization.
const Instructions = `You are a Kustomize rendering, diffing, and troubleshooting server.

## Multi-project workspaces

The project parameter scopes all operations (checkpoints, renders, dependency
scans) to one project directory. It is REQUIRED when the client has more than one
workspace root open. Omitting it when multiple roots exist binds the call to an
arbitrary root — do not rely on this.

How to set project:

1. Multiple workspace roots (the common case in Cursor with separate repo folders):
   The client lists each folder as an absolute path in roots/list. Pass that
   exact absolute path as project on every call.
   Example: project="/Users/me/repos/clusters-universal"

2. Single workspace root with nested project directories:
   Pass a single-segment relative subdirectory name.
   Example: project="my-cluster"

3. Single workspace root, single project: omit project entirely.

Do NOT use multi-segment relative paths (e.g. "infra/clusters-universal").
They resolve against the first root and fail when the target lives elsewhere.
Always use the absolute path instead.

ALWAYS pass the identical project value on every related call:
create_checkpoint, render, inventory, trace, diff_checkpoints, diff_paths,
clear_checkpoint, dependencies. Checkpoint IDs are scoped per project — a
checkpoint created with project="/a" is invisible to calls with project="/b"
or no project.

## Checkpoints

A checkpoint is a named directory that holds rendered Kustomize output.

Lifecycle rules:
- Call create_checkpoint BEFORE render. It returns a checkpoint_id.
- A given Kustomize path can only be rendered once per checkpoint. To re-render
  the same path, create a new checkpoint.
- Checkpoints persist until cleared with clear_checkpoint.
- Each project (when project is set) has its own isolated checkpoint pool under
  <project>/.kustomize-mcp/checkpoints/.

## Core workflow

1. create_checkpoint → returns checkpoint_id.
2. render(checkpoint_id, path) → builds the Kustomize directory at path into that
   checkpoint. path is ALWAYS relative to the effective project root, never absolute.
3. After rendering, use:
   - inventory(checkpoint_id) to list all resources with provenance metadata.
   - trace(checkpoint_id, path, kind, name) to show where one resource originated
     and what transformers modified it.
   - diff_paths(checkpoint_id, path_1, path_2) to compare two rendered roots in
     the same checkpoint.
   - diff_checkpoints(checkpoint_id_1, checkpoint_id_2) to compare all rendered
     output between two checkpoints (before/after changes).
4. dependencies(path) lists files that affect a Kustomization — no checkpoint
   needed.

## Parameter reference

path (on render, trace, diff_paths, dependencies): MUST be a forward-slash
relative path from the effective root to a Kustomize directory. Never absolute.
Examples: "overlays/prod", "app", "clusters/bootstrap".

checkpoint_id: the string returned by create_checkpoint. Pass it unchanged.

filter (on inventory): narrows results by exact match on kind, api_version,
namespace, or name. Each field is optional.

## Flux Kustomization recursive rendering

Call render with recursive=true on a Kustomize root that produces Flux
Kustomization CRDs (kustomize.toolkit.fluxcd.io/v1 or v1beta2). The server
renders the root, discovers Flux Kustomization objects, and for each one with a
local spec.path, renders that target with Flux-specific transformers merged
in-memory. This repeats breadth-first for nested Flux Kustomizations.

After a recursive render:
- Use inventory without a path filter to list resources across all subtrees.
- The response includes warnings (missing paths, cycles) and conflicts (same
  resource identity produced by multiple Flux Kustomizations).

The annotation kustomize.toolkit.fluxcd.io/kustomization-path on a Flux
Kustomization CRD overrides spec.path for local workspace builds. Flux
Kustomization CRDs without metadata.namespace are normalized to "default".`
