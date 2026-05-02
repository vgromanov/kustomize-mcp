package mcpapp

// Instructions is sent to MCP clients during initialization.
const Instructions = `You are a Kustomize rendering, diffing, and troubleshooting server.

Configuration can be rendered for a given path and stored in checkpoints. You can
diff all rendered paths between two checkpoints, or diff two specific paths
inside the same checkpoint.

Use checkpoints to store different versions of the configuration for the same
path. This allows you to track the effects of changes over time.

When working with Flux Kustomization CRDs in the same repository, call render with
recursive=true on the top-level Kustomize root. The server renders the root,
discovers Flux Kustomization objects in the rendered output, renders each local
spec.path target with Flux-specific transformers merged in-memory, and repeats
for nested Flux Kustomizations. After a recursive render, use inventory without
a path filter to list resources across all rendered subtrees; warnings list
missing paths or cycles, and conflicts counts overlapping resources reconciled
by more than one Flux Kustomization.

Flux Kustomization manifests may include a workspace-only annotation
kustomize.toolkit.fluxcd.io/kustomization-path on the CRD. When set to a
non-empty value, recursive rendering uses it instead of spec.path as the
workspace-relative directory to build. The _tree.json sidecar then records
declared_path with the original spec.path when it differed from the path
used. Omit the annotation to keep using spec.path as today.

Flux Kustomization CRDs without metadata.namespace are normalized to
"default" at parse time, mirroring kube-apiserver admission. Source repos
that rely on the parent kustomize-controller to inject the namespace at
apply time render without the previous "segment must be non-empty" error.

To understand what files are involved in a Kustomization, query its dependencies.
Changing any of these will affect the rendered output. Kustomizations may depend
on other Kustomizations, so the effect of a change may be indirect.

After rendering, use the inventory tool to list all resources in a checkpoint with
their origin (which file introduced them) and transformer metadata (what modified
them). Use the optional filter parameter to narrow results by kind, api_version,
namespace, or name.

To trace the full provenance of a specific resource, use the trace tool on a
rendered checkpoint. It returns the file the resource originated from and an
ordered list of transformers that modified it.

All paths are relative to the MCP workspace root: the folder the client reports via
roots/list (e.g. the Cursor workspace), or KUSTOMIZE_MCP_ROOT if set, or the process
working directory as a last resort.

When working in a multi-project workspace, pass the optional project parameter on every
tool call (a directory path relative to that workspace root). This scopes the effective
root to workspace/project so Kustomize paths stay short (for example path overlays/prod
instead of project-a/overlays/prod), checkpoints live under project/.kustomize-mcp/checkpoints
instead of mixing across projects, and dependency scans stay within that project tree.
Use the same project value for create_checkpoint, render, inventory, trace, diff, clear,
and dependencies so checkpoints line up.`
