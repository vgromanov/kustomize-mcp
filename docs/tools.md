# MCP tools and prompts

This document mirrors the server surface registered in
`[internal/mcpapp/register.go](../internal/mcpapp/register.go)`. Structured tool
outputs follow the MCP Go SDK rule: **JSON objects** (no bare primitives).

All tools accept an optional `project` parameter (see below).

## `project` parameter (all tools)

| Value form | When to use | Example |
|------------|-------------|---------|
| Absolute path | Multi-root workspace; folders listed as absolute paths | `"/Users/me/repos/clusters-universal"` |
| Relative path | Monorepo; project is a subdirectory of the single root | `"clusters-universal"` |
| Omitted | Single-project workspace | — |

**Must** be the same value on every related call. Checkpoint IDs are scoped to the
project — a checkpoint created with `project="/a"` does not exist for calls using
`project="/b"` or no project.

---

## Tools

### `create_checkpoint`

Creates an empty checkpoint directory for rendered Kustomize output.

**Input:**

| Field     | Type   | Required | Description |
|-----------|--------|----------|-------------|
| `project` | string | no       | See above.  |

**Output:** `{ "checkpoint_id": "<id>" }` — directory name under
`.kustomize-mcp/checkpoints/`.

---

### `clear_checkpoint`

Removes one checkpoint or all checkpoints.

**Input:**

| Field           | Type   | Required | Description                              |
|-----------------|--------|----------|------------------------------------------|
| `checkpoint_id` | string | no       | Checkpoint to remove; omit to clear all. |
| `project`       | string | no       | See above.                               |

**Output:** `{ "status": "ok", "message": "..." }`.

---

### `render`

Renders a Kustomize directory into a checkpoint. With `recursive: true`, follows
Flux `Kustomization` CRDs in the rendered output and renders nested workspace
paths.

**Input:**

| Field           | Type   | Required | Description                                            |
|-----------------|--------|----------|--------------------------------------------------------|
| `checkpoint_id` | string | yes      | Checkpoint directory name from `create_checkpoint`.    |
| `path`          | string | yes      | Relative path to Kustomize root (from effective root). |
| `recursive`     | bool   | no       | When true, recurse into Flux Kustomizations.           |
| `project`       | string | no       | See above.                                             |

**Output (non-recursive):** `{ "path": "<rendered dir relative to effective root>" }`.

**Output (recursive):** includes `root_path`, `rendered_paths`,
`flux_kustomizations`, `conflicts`, `warnings`.

---

### `diff_checkpoints`

Unified diff of all rendered manifests between two checkpoints.

**Input:**

| Field             | Type   | Required | Description |
|-------------------|--------|----------|-------------|
| `checkpoint_id_1` | string | yes      |             |
| `checkpoint_id_2` | string | yes      |             |
| `project`         | string | no       | See above.  |

**Output:** `[diff.Result](../internal/diff/differ.go)` — added, deleted,
modified, replaced sets plus optional `.diff` text.

---

### `diff_paths`

Diff two rendered roots under the **same** checkpoint.

**Input:**

| Field           | Type   | Required | Description |
|-----------------|--------|----------|-------------|
| `checkpoint_id` | string | yes      |             |
| `path_1`        | string | yes      |             |
| `path_2`        | string | yes      |             |
| `project`       | string | no       | See above.  |

**Output:** `diff.Result`.

---

### `dependencies`

Lists file and Kustomization dependencies for a path. No checkpoint needed.

**Input:**

| Field       | Type   | Required | Description                                                  |
|-------------|--------|----------|--------------------------------------------------------------|
| `path`      | string | yes      | Relative path to `kustomization.yaml` (or file for reverse). |
| `recursive` | bool   | no       | Recursive dependency walk.                                   |
| `reverse`   | bool   | no       | Reverse dependency mode.                                     |
| `project`   | string | no       | See above.                                                   |

**Output:** `{ "paths": ["..."] }`.

---

### `trace`

Traces origin and transformations for one resource in a rendered checkpoint.

**Input:**

| Field           | Type   | Required | Description |
|-----------------|--------|----------|-------------|
| `checkpoint_id` | string | yes      |             |
| `path`          | string | yes      |             |
| `kind`          | string | yes      |             |
| `name`          | string | yes      |             |
| `namespace`     | string | no       |             |
| `project`       | string | no       | See above.  |

**Output:** `[trace.TraceResult](../internal/trace/trace.go)`.

---

### `inventory`

Lists rendered resources with provenance metadata from `_tree.json`.

**Input:**

| Field           | Type   | Required | Description                                                   |
|-----------------|--------|----------|---------------------------------------------------------------|
| `checkpoint_id` | string | yes      |                                                               |
| `path`          | string | no       | Narrow to one rendered root; omit to merge entire checkpoint. |
| `filter`        | object | no       | Exact match on `kind`, `api_version`, `namespace`, `name`.    |
| `project`       | string | no       | See above.                                                    |

**Output:** `[manifest.ResourceTree](../internal/manifest/tree.go)`.

---

## Prompts

| Name           | Arguments              | Purpose                                        |
|----------------|------------------------|-------------------------------------------------|
| `explain`      | `query`                | Explain the project's Kustomize layout.        |
| `refactor`     | `query`                | Guided refactor using the tools.               |
| `troubleshoot` | `path`, `kind`, `name` | Trace a resource (render → inventory → trace). |
| `diff_dirs`    | `path_1`, `path_2`     | Compare two Kustomize dirs via checkpoints.    |

Prompt bodies are built from `[internal/prompts/prompts.go](../internal/prompts/prompts.go)`.
