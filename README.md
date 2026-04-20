# kustomize-mcp (Go)

[![CI](https://github.com/vgromanov/kustomize-mcp/actions/workflows/ci.yml/badge.svg)](https://github.com/vgromanov/kustomize-mcp/actions/workflows/ci.yml)
[![Release](https://github.com/vgromanov/kustomize-mcp/actions/workflows/release.yml/badge.svg)](https://github.com/vgromanov/kustomize-mcp/actions/workflows/release.yml)
[![License](https://img.shields.io/github/license/vgromanov/kustomize-mcp)](LICENSE)

A [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) server that helps AI assistants work safely with [Kubernetes](https://kubernetes.io/) configuration built with [Kustomize](https://kustomize.io/): render overlays, diff checkpoints, discover manifest dependencies, and trace resource provenance.

## Acknowledgements

This project is inspired by and aims to stay compatible with the tool surface of **[mbrt/kustomize-mcp](https://github.com/mbrt/kustomize-mcp)** — the original **Python** implementation (`uv run server.py`) that shells out to `kustomize`, **git**, and **helm**, and ships a convenient Docker image. Thank you to the upstream authors for the design and documentation.

This repository is a **Go reimplementation**: rendering uses the Kustomize Go APIs ([`sigs.k8s.io/kustomize/api`](https://pkg.go.dev/sigs.k8s.io/kustomize/api)) (krusty-style embedding) instead of invoking the `kustomize` binary; unified directory diffs use pure Go ([`go-difflib`](https://github.com/pmezard/go-difflib)); dependency analysis walks the workspace using YAML and the filesystem. **Git** and the **helm** CLI are not required for core workflows. Optional Helm rendering can be enabled via environment variables and may still invoke a `helm` binary when Kustomize's Helm builtin needs it—see below.

## Features

- **Tools** (aligned with upstream naming and intent): `create_checkpoint`, `clear_checkpoint`, `render`, `diff_checkpoints`, `diff_paths`, `dependencies`, `inventory`, `trace`
- **Prompts**: `explain`, `refactor`, `diff_dirs`, `troubleshoot` (workflow text adapted from upstream usage strings)
- **Automatic origin and transformer tracking**: every `render` transparently captures Kustomize's internal provenance metadata and writes it as a structured sidecar alongside the rendered manifests

## Requirements

- **Go 1.25+** (see `go.mod`) to build from source

## Install

**Released binaries and container images** are published from GitHub (see
[Releases](https://github.com/vgromanov/kustomize-mcp/releases) and
`ghcr.io/vgromanov/kustomize-mcp` after you push a version tag).

From source with a recent Go toolchain:

```bash
go install github.com/vgromanov/kustomize-mcp/cmd/kustomize-mcp@latest
```

Print the embedded version:

```bash
kustomize-mcp -version
```

**Docker** (multi-stage build in this repo):

```bash
docker build -t kustomize-mcp .
docker run --rm -i kustomize-mcp
```

Full tool/prompt field reference: [docs/tools.md](docs/tools.md).

## Build and test

```bash
make build    # writes ./kustomize-mcp (CGO_ENABLED=0, static binary)
make test     # go test ./... -count=1
make cover    # coverage profile for internal packages → coverage.out
make lint     # golangci-lint (install: https://golangci-lint.run)
```

Cross-compile all supported platforms into `dist/`:

```bash
make dist     # builds linux/amd64, linux/arm64, darwin/amd64, darwin/arm64
```

Or build a single target manually:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o kustomize-mcp-linux-amd64 ./cmd/kustomize-mcp
```

Install from a clone:

```bash
make install
# or: go install ./cmd/kustomize-mcp
```

## Running the server

The server speaks **stdio** MCP (same transport model as the upstream project).

```bash
./kustomize-mcp
```

Clients should expose the workspace folder via MCP **roots** (e.g. Cursor / VS Code workspace). If the client does not report roots, set an explicit filesystem root:

| Variable | Meaning |
|----------|---------|
| `KUSTOMIZE_MCP_ROOT` | Absolute path used as the Kustomize workspace (overrides roots / working directory). |
| `KUSTOMIZE_LOAD_RESTRICTIONS` | `true` / `false` / `0` / `1` — passed through to Kustomize load restrictions (default: `true`). |
| `KUSTOMIZE_ENABLE_HELM` | Enable Kustomize's Helm builtin (default: `false`). When enabled, Helm may still require a `helm` binary at runtime. |

Path arguments to tools are **relative to that workspace root**.

## MCP client configuration

Point the client at the built binary and ensure the workspace is the current project (or set `KUSTOMIZE_MCP_ROOT`).

**Cursor** (`~/.cursor/mcp.json` or project config), minimal example:

```json
{
  "mcpServers": {
    "kustomize": {
      "command": "/absolute/path/to/kustomize-mcp",
      "cwd": "${workspaceFolder}"
    }
  }
}
```

Adjust the key name and path to match your setup. For VS Code MCP, use the same idea: `command` + working directory / roots so paths resolve under your repo.

The upstream Python server documents Docker-based setups; this Go repo ships its
own `Dockerfile` for a minimal static binary image.

## Resource provenance tracking

Every call to `render` transparently enriches the Kustomize build with origin and transformer metadata — without requiring users to add `buildMetadata` to their kustomization files. The mechanism works as follows:

1. **Annotating filesystem wrapper** — the render pipeline wraps the on-disk filesystem with a thin interceptor (`annotatingFS`) that injects `buildMetadata: [originAnnotations, transformerAnnotations]` into every kustomization file as krusty reads it. The injection is purely in-memory; user files on disk are never modified. If the file already declares `buildMetadata`, the values are merged without duplicates.

2. **Metadata extraction** — after krusty finishes the build, the pipeline iterates through the resulting resource map and calls `GetOrigin()` and `GetTransformations()` on each resource. Paths from these annotations are resolved to workspace-relative form so they are meaningful regardless of where the build ran.

3. **Sidecar persistence** — the extracted metadata is written as a `_tree.json` file alongside the rendered YAML manifests inside the checkpoint directory. This structured sidecar contains every resource's origin (which file introduced it, which repo/ref for remote bases) and an ordered list of transformers that modified it.

4. **Annotation stripping** — the injected `config.kubernetes.io/origin` and `alpha.config.kubernetes.io/transformations` annotations are stripped from the rendered YAML files before they are written to disk. If the user explicitly declared `buildMetadata` in their kustomization, those annotations are preserved — only the ones the server silently injected are removed. This keeps rendered output and diffs clean while the sidecar retains the full provenance data.

### Benefits

- **Zero-configuration provenance** — AI assistants (and humans) get full visibility into where every resource came from and what modified it, without touching the user's kustomization files or requiring any flags.
- **Clean diffs** — because injected annotations are stripped before writing, `diff_checkpoints` and `diff_paths` remain free of annotation noise. The provenance data lives in a separate sidecar that the diff walker skips.
- **Compositional tooling** — the `inventory` and `trace` tools are pure readers of the sidecar, not separate rendering pipelines. This means they are fast (no re-render), consistent (they read the same data the render produced), and easy to extend.
- **Respect for user intent** — if a user has explicitly opted into `originAnnotations` or `transformerAnnotations` via `buildMetadata`, those annotations appear in the rendered YAML as expected. The server only strips what it silently injected.

### Resource filtering

All resource-listing tools accept an optional `filter` parameter with exact-match fields for `kind`, `api_version`, `namespace`, and `name`. A nil filter matches everything. Filtering is applied post-render and post-diff, so the underlying data is always complete — the filter only narrows what is returned to the caller.

## Tool reference

| Tool | Description |
|------|-------------|
| `create_checkpoint` | Create an empty checkpoint directory for rendered output. |
| `clear_checkpoint` | Remove one checkpoint (`checkpoint_id`) or all checkpoints. |
| `render` | Run `kustomize build` for a relative path into a checkpoint. Automatically captures origin and transformer metadata into a `_tree.json` sidecar. |
| `diff_checkpoints` | Diff all rendered trees between two checkpoints. |
| `diff_paths` | Diff two rendered relative paths inside the same checkpoint. |
| `dependencies` | List Kustomization dependencies (`recursive`, `reverse` supported). |
| `inventory` | List all rendered resources in a checkpoint with their origin and transformer metadata. Accepts optional `path` (narrow to a single rendered root) and `filter` (select by kind/name/namespace/apiVersion). |
| `trace` | Trace the provenance of a specific resource by kind and name. Returns the source file that introduced the resource and an ordered list of every transformer that modified it. |

Structured outputs follow the MCP Go SDK rules (JSON objects for tool results).

## Prompts

| Prompt | Role |
|--------|------|
| `explain` | Guided explanation of the repo's Kustomize layout (`query`). |
| `refactor` | Guided refactor request (`query`). |
| `diff_dirs` | Compare two Kustomize directories (`path_1`, `path_2`) using checkpoints. |
| `troubleshoot` | Trace a specific resource's origin and transformations (`path`, `kind`, `name`). Guides the assistant through render, inventory, and trace. |

## Example workflows

### Inventory after render

```
1. create_checkpoint          → ckp-abc
2. render ckp-abc app/overlays/prod
3. inventory ckp-abc --path app/overlays/prod
```

Returns every resource with its source file and transformer chain. Add a `filter` with `{"kind":"Deployment"}` to narrow to deployments only.

### Tracing a resource

```
1. create_checkpoint          → ckp-abc
2. render ckp-abc app/overlays/prod
3. trace ckp-abc app/overlays/prod Deployment frontend
```

Returns the file that introduced the `frontend` Deployment and every transformer (namePrefix, commonLabels, patches, etc.) that modified it, in order.

### Comparing changes with provenance

```
1. create_checkpoint          → ckp-before
2. render ckp-before app/overlays/prod
3. (make changes to the kustomization)
4. create_checkpoint          → ckp-after
5. render ckp-after app/overlays/prod
6. diff_checkpoints ckp-before ckp-after       ← what changed
7. inventory ckp-after --path app/overlays/prod ← who owns what now
```

## Flux Kustomization support

The `render` tool accepts `recursive: true` to follow
[Flux Kustomization](https://fluxcd.io/flux/components/kustomize/kustomizations/)
CRDs (`kustomize.toolkit.fluxcd.io/v1` and `v1beta2`) in the rendered output,
resolve workspace-local `spec.path` (or the
`kustomize.toolkit.fluxcd.io/kustomization-path` annotation), merge Flux fields
into the in-memory kustomization, and render nested subtrees with `_tree.json`
provenance. See [AGENTS.md](AGENTS.md) for design details and known limits
(`sourceRef`, `postBuild`, cross-layer `dependsOn`, etc.).

Contributions toward deeper Flux parity are welcome — see
[CONTRIBUTING.md](CONTRIBUTING.md).

## Security

See [SECURITY.md](SECURITY.md) for vulnerability reporting.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[Apache License 2.0](LICENSE).
