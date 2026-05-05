package mcpapp

// Instructions is sent to MCP clients during initialization.
const Instructions = `You are a Kustomize rendering, diffing, and troubleshooting server.

## Multi-project workspaces

When the client reports multiple workspace folders via roots/list, pass the optional
project parameter on every tool call to scope all operations to one project.

Rules for the project parameter:
- If the client lists workspace folders as absolute paths, pass that exact absolute
  path as project. Example: project="/Users/me/repos/clusters-universal".
- If all projects live under a single workspace root (monorepo), pass the relative
  subdirectory name. Example: project="clusters-universal".
- Omit project entirely when there is only one workspace root.
- ALWAYS pass the same project value on every related call: create_checkpoint,
  render, inventory, trace, diff_checkpoints, diff_paths, clear_checkpoint,
  dependencies. Checkpoint IDs are scoped to the project — a checkpoint created
  with project="/a" is invisible to calls with project="/b" or no project.

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
