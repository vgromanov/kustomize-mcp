# AGENTS.md — kustomize-mcp

Persistent context for AI agents working on this codebase.

## Project identity

`kustomize-mcp` is a **Go** MCP (Model Context Protocol) server that helps AI
assistants work safely with Kubernetes configuration built with Kustomize. It is
a reimplementation of [mbrt/kustomize-mcp](https://github.com/mbrt/kustomize-mcp)
(Python, shells out to `kustomize`/`git`/`helm`) using the Kustomize Go APIs
directly.

Module path: `github.com/vgromanov/kustomize-mcp`

## Core philosophy: zero shell execs

The project was designed from day one around one hard constraint: **no subprocess
calls for core workflows**. Rendering uses `sigs.k8s.io/kustomize/api/krusty`
(the same library the `kustomize` CLI wraps), diffs use `go-difflib`, and
dependency analysis walks the filesystem with `gopkg.in/yaml.v3`. The only
exception is when `KUSTOMIZE_ENABLE_HELM=true` — Kustomize's Helm builtin may
still shell out to a `helm` binary at runtime.

## Package layout

```
cmd/kustomize-mcp/          Thin main(); delegates to internal/mcpapp.RunStdio
internal/
  mcpapp/                   MCP server wiring — tool/prompt registration, env config, integration tests
    doc.go                  Instructions constant (sent to MCP clients at init)
    register.go             Tool and prompt handlers (AddTool / AddPrompt calls)
    run.go                  Server construction, OptionsFromEnv, transport entry points
    env.go                  ParseBoolEnv helper
    integration_test.go     End-to-end MCP tests via in-memory transport
  kustmcp/                  Orchestration layer — ties checkpoints, render, deps, diff, inventory, trace
    server.go               Server struct, workspace root, checkpoint lifecycle
    recursive.go            RenderRecursive — Flux Kustomization BFS + warnings/conflicts summary
  render/                   Krusty-based rendering + provenance pipeline
    renderer.go             Checkpoint dirs, krusty build, metadata extraction, annotation stripping, RenderFlux
    annotatefs.go           annotatingFS + fluxAnnotatingFS + injectFluxFields (Flux spec merged in-memory)
    commonmeta.go           spec.commonMetadata applied post-krusty on rendered YAML
  flux/                     Flux Kustomization scan + spec types (YAML-only, no Flux SDK import)
    scan.go                 ScanRenderedDir — parse Flux Kustomization CRDs from rendered dirs
  deps/                     Dependency resolution — pure YAML + filesystem walk
    resolver.go             Lazy-indexed workspace scanner, forward/reverse dependency graph
  diff/                     Unified directory diffs — go-difflib, no git
    differ.go               Dir() produces Result{Added, Deleted, Modified, Replaced} + .diff file
  manifest/                 Kubernetes resource metadata types
    meta.go                 Metadata struct, ToFilename/FromYAMLDoc/FromRelPath encoding
    tree.go                 ResourceTree / ResourceEntry / ResourceOrigin — the _tree.json sidecar model
  filter/                   Cross-cutting resource filter (kind, api_version, namespace, name)
    filter.go               ResourceFilter.Match, FilterMetadata, FilterEntries
  inventory/                Checkpoint reader — loads _tree.json sidecars
    inventory.go            Load (single path or merged), Filter
  trace/                    Resource provenance tracer
    trace.go                Lookup (Tier-1, fast), Compare (Tier-2, with field diff)
  workspace/                Workspace root resolution
    dir.go                  Dir() — KUSTOMIZE_MCP_ROOT → MCP roots/list → os.Getwd()
  prompts/                  User-facing prompt text
    prompts.go              Usage constant, ExplainBody, RefactorBody, TroubleshootBody, DiffDirsMessages
```

## MCP surface

### Tools

| Tool | Purpose |
|------|---------|
| `create_checkpoint` | Creates an empty checkpoint directory |
| `clear_checkpoint` | Removes one or all checkpoints |
| `render` | `kustomize build` via krusty into a checkpoint; writes `_tree.json` sidecar. With `recursive=true`, follows `kustomize.toolkit.fluxcd.io` Kustomizations (local `spec.path` only) and merges inventory/conflicts |
| `diff_checkpoints` | Unified diff of all rendered manifests between two checkpoints |
| `diff_paths` | Diff two rendered paths inside the same checkpoint |
| `dependencies` | Forward/reverse Kustomization dependency graph (recursive supported) |
| `inventory` | Lists rendered resources with origin + transformer metadata; supports filter |
| `trace` | Traces one resource's provenance: origin file + ordered transformer chain |

All tools accept an optional `project` argument: a relative path resolved across MCP roots, or an absolute directory equal to or under a workspace root from `roots/list` (preferred in multi-root Cursor when folders are listed as absolute paths). When set, the effective workspace root is that directory; paths and checkpoints are scoped there (`.kustomize-mcp/` lives under that root). Omit `project` for single-folder workspaces.

### Prompts

`explain`, `refactor`, `diff_dirs`, `troubleshoot` — each composes a `Usage`
preamble with the user's query to guide the assistant through the correct tool
workflow.

## Key design patterns

### annotatingFS — transparent provenance injection

The most important internal pattern. Krusty always computes origin and
transformer annotations internally but strips them unless the kustomization
declares `buildMetadata: [originAnnotations, transformerAnnotations]`. There is
no `Options`-level override in the krusty API.

`annotatingFS` wraps `filesys.FileSystem` and intercepts `ReadFile` for
kustomization files, injecting the `buildMetadata` field into the YAML in
memory. User files on disk are never modified. If the file already declares
`buildMetadata`, values are merged without duplicates. This works through the
entire recursive kustomization chain because krusty reads all kustomization
files through the same filesystem interface.

`fluxAnnotatingFS` composes the same pattern for Flux: only the root
`kustomization.yaml` under a Flux `spec.path` directory receives merged Flux
fields (`patches`, `images`, `namespace`/`targetNamespace`, `namePrefix`,
`nameSuffix`, `components`, strategic/JSON6902 patches) before `buildMetadata`
injection, matching fluxcd/pkg `Generator.WriteFile` semantics without writing
to disk. `spec.commonMetadata` is applied after krusty as a YAML document merge
(labels/annotations; Flux values override on key collision).

After the build, the render pipeline:
1. Extracts origin/transformer metadata from each resource via `GetOrigin()` /
   `GetTransformations()` and writes it as `_tree.json`
2. Strips the injected annotations from the rendered YAML — but preserves
   annotations the user explicitly declared

This keeps rendered output and diffs clean while the sidecar retains full
provenance data.

### Checkpoints as rich artifacts

Rather than adding separate render paths, the design enriches checkpoints with
metadata sidecars. `inventory` and `trace` are pure readers of `_tree.json` —
no re-rendering, no extra krusty invocations. This makes them fast, consistent
with what the render produced, and easy to extend.

### Lazy dependency index

`deps.Resolver` defers its filesystem walk to first use (`ensureScanned`). This
was a fix for a real bug: initializing the resolver at server startup caused a
full `WalkDir` from the workspace root, which on macOS could hit
permission-denied paths like `~/.Trash` and panic. The lazy pattern also
tolerates permission errors gracefully — denied directories are skipped, not
fatal.

### Workspace root resolution

`workspace.Dir()` resolves the workspace through a fallback chain:
1. `KUSTOMIZE_MCP_ROOT` env var (explicit override)
2. MCP `roots/list` from the client session (Cursor / VS Code opened folder)
3. `os.Getwd()` as last resort

When `project` is set on a tool call, the effective root is resolved by
`workspace.ResolveAbsProject` (absolute path: must equal or lie under a root from
`AllRoots`, validated against MCP roots/list or `KUSTOMIZE_MCP_ROOT`) or
`workspace.ResolveProject` (relative path: subdirectory match across roots, then suffix
match on root paths, then fallback). Absolute paths are preferred in multi-root Cursor
workspaces when folders are listed as full paths.

Checkpoints and dependency scans then use the resolved root.

The `RootSession` interface exists specifically to make this testable without a
real MCP client.

### Manifest filename encoding

Rendered manifests are written as
`{apiVersion}+{kind}+{namespace}+{name}.yaml` where `/` in apiVersion is
replaced with `#`. This encoding is invertible — `manifest.FromRelPath` can
reconstruct full metadata from a filename alone, which the diff system uses to
classify changes without re-parsing YAML.

## Testing patterns

- **`t.TempDir()` fixtures**: Most tests build minimal kustomization trees under
  temp directories rather than maintaining fixture files.
- **In-memory MCP transport**: `integration_test.go` uses
  `mcp.NewInMemoryTransports()` + `mcp.NewServer` + `mcp.Client` to exercise the
  full tool/prompt surface end-to-end without stdio.
- **Env-driven workspace**: Integration tests set `KUSTOMIZE_MCP_ROOT` so the
  MCP layer resolves the workspace without a real client session.
- **Structured result helpers**: `structuredToMap`, `mustToolString`,
  `mustToolStringSlice`, `mustToolOK` decode `StructuredContent` from tool
  results.
- **Coverage**: `make cover` scopes to `./internal/...` to avoid toolchain
  issues with `cmd/*/main`. Target is >80% on all internal packages.

## Environment variables

| Variable | Default | Effect |
|----------|---------|--------|
| `KUSTOMIZE_MCP_ROOT` | (none) | Absolute workspace path override |
| `KUSTOMIZE_LOAD_RESTRICTIONS` | `true` | Krusty load restrictions (rootOnly vs none) |
| `KUSTOMIZE_ENABLE_HELM` | `false` | Enable Kustomize Helm builtin (may shell out to `helm`) |

## Build

```bash
make build    # → ./kustomize-mcp (CGO_ENABLED=0)
make test     # go test ./... -count=1
make cover    # coverage profile → coverage.out
make race     # -race on internal packages
```

## Design decisions from project history

### Initial implementation

The project started as a direct port of mbrt/kustomize-mcp's tool surface to Go,
replacing `subprocess.run(["kustomize", ...])` with `krusty.MakeKustomizer` and
`subprocess.run(["git", "diff", ...])` with `go-difflib`. Early bugs included:
- `os.Mkdir` failing on nested checkpoint paths (→ `os.MkdirAll`)
- `validateRelPath` not rejecting `..` segments (→ explicit segment check)
- MCP SDK requiring tool output schemas to be JSON objects (→ wrapper structs
  like `createCheckpointOut`)
- `deps.Resolver` doing a full `WalkDir` at startup (→ lazy `ensureScanned`)
- `os.Getwd()` returning wrong path when Cursor didn't set cwd (→ MCP roots
  fallback chain)

### Provenance pipeline (inventory, trace, filter)

The second major phase added resource provenance tracking. Key insight: krusty
*always* computes origin/transformer metadata internally but strips it unless
`buildMetadata` is declared. Rather than requiring users to modify their
kustomization files, the `annotatingFS` pattern was created to inject this
transparently.

The `_tree.json` sidecar model was chosen over alternatives (separate metadata
API, annotations in rendered output) because it keeps diffs clean and makes
inventory/trace into pure readers of pre-computed data — no re-rendering needed.

Token analysis showed this approach reduces agent context consumption by 75-95%
compared to having agents parse raw YAML output: inventory is ~120 tokens/resource,
trace is ~200-400 tokens total.

### Flux-aware rendering (implemented)

Recursive Flux rendering is enabled via the `render` tool parameter
`recursive=true` ([internal/kustmcp/recursive.go](internal/kustmcp/recursive.go)).
The server renders the requested Kustomize root, scans rendered YAML for
`kustomize.toolkit.fluxcd.io/v1` and `v1beta2` `Kustomization` CRDs
([internal/flux/scan.go](internal/flux/scan.go)), and for each object with a
workspace-local path runs `Renderer.RenderFlux` into
`checkpoints/<id>/<namespace>/<name>/` ([internal/render/renderer.go](internal/render/renderer.go)).
Nested Flux Kustomizations are processed breadth-first; duplicate
namespace/name pairs are skipped with a warning (cycle / duplicate). The effective path is `spec.path` unless the CRD carries a non-empty
`kustomize.toolkit.fluxcd.io/kustomization-path` annotation (workspace-relative
override for local layouts that do not match the Flux source checkout). When
the annotation overrides, `_tree.json` includes `flux_kustomization.spec_path`
(the path used) and `declared_path` (the original `spec.path`). Missing
directories for the effective path produce warnings and do not fail the whole run.

Flux Kustomization CRDs without `metadata.namespace` are normalized to
`default` at parse time (matching the kube-apiserver admission default). This
keeps the dest path (`<checkpoint>/<namespace>/<name>/`), `Key()`, cycle
detection, and `_tree.json` consistent for source repos that omit the field
(common when the parent kustomize-controller injects the namespace at apply
time).

`inventory` with no `path` walks the checkpoint recursively for every
`_tree.json` ([internal/inventory/inventory.go](internal/inventory/inventory.go)),
merges resources, and attaches `conflicts` when the same `(apiVersion, kind,
namespace, name)` is produced under more than one Flux subtree (or plain vs
Flux, distinguished via `FluxOrigin` metadata).

**Deferred / not implemented**: `spec.sourceRef` resolution (cluster-only),
`postBuild.substitute` / `substituteFrom`, cross-layer `dependencies` graph
extensions, and Flux `dependsOn` ordering beyond best-effort BFS.

### Flux SSA conflict behavior (research)

Research into how Flux kustomize-controller handles overlapping resources found:
- All Flux Kustomizations on a controller share one SSA field manager name
  (`kustomize-controller`), not per-Kustomization identity
- Ownership is label-based (`kustomize.toolkit.fluxcd.io/name`), not SSA-based
- Overlapping resources cause tug-of-war: last writer wins for all fields
  including ownership labels
- Per-resource SSA policies via `kustomize.toolkit.fluxcd.io/ssa` annotation
  (`Override`, `Merge`, `IfNotPresent`, `Ignore`, `reconcile: disabled`)

This research informs how the future Flux conflict detection feature should
surface warnings.

## Style and conventions

- Go 1.25+, no CGO for builds
- All internal packages under `internal/` — nothing exported outside the module
- JSON field names use `snake_case` to match the upstream Python MCP server
- Error messages are lowercase, no trailing punctuation
- Path arguments throughout the stack are workspace-relative and use forward
  slashes (converted via `filepath.ToSlash`). With the MCP `project` tool
  argument, the effective workspace root is either that absolute path (when
  allowed) or the resolved relative project; Kustomize `path` arguments stay
  relative to that root.
- Tool output schemas must be JSON objects per MCP SDK rules — wrap primitives
  in structs with `json` and `jsonschema` tags
- Tests use standard `testing` package, no third-party test framework
