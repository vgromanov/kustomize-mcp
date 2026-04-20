# Security policy

## Supported versions

The latest minor release is supported. Older versions get fixes only at the
maintainers' discretion.

## Reporting a vulnerability

Please report security issues privately via a
[GitHub Security Advisory](https://github.com/vgromanov/kustomize-mcp/security/advisories/new)
rather than a public issue. We will acknowledge within a few business days.

If GitHub advisories are unavailable to you, contact the maintainer directly
through the email listed on their GitHub profile.

## Threat model

`kustomize-mcp` is intended for **local development** with an MCP client (e.g.
Cursor, VS Code). Defaults reflect that:

- **stdio transport (default).** The MCP client launches the binary as a
  subprocess; workspace paths and env come from the client. This is the
  recommended deployment.
- **No network listener** in the default configuration — the server does not bind
  ports.

## Secrets and workspace data

- The server reads and renders files under the resolved workspace root (MCP
  roots, `KUSTOMIZE_MCP_ROOT`, or current working directory). Treat the workspace
  as sensitive if it contains credentials in plain files.
- Checkpoint output is written under `.kustomize-mcp/checkpoints/` in the
  workspace.

## Path traversal and load restrictions

- Tool path arguments are validated to stay within the workspace root; `..`
  segments are rejected.
- `KUSTOMIZE_LOAD_RESTRICTIONS` (default: enabled) is passed through to Kustomize
  load restrictions so bases cannot escape the kustomization root without
  explicit configuration.

## Helm execution

When `KUSTOMIZE_ENABLE_HELM=true`, Kustomize's Helm builtin may **shell out** to
a `helm` binary at runtime. This is **off** by default. Only enable it in
trusted environments where the `helm` binary and chart sources are under your
control.

## Tool side effects

Tools such as `render` and `clear_checkpoint` write or delete files under
`.kustomize-mcp/` in the workspace. LLMs invoking these tools should be
supervised; this server does not implement per-tool allowlisting beyond what the
MCP client provides.
