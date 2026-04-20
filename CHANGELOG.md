# Changelog

All notable changes to this project are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-04-20

Initial public release.

### Added

- Go MCP server (`kustomize-mcp`) using Kustomize Go APIs (krusty), no shell to
  `kustomize` or `git` for core workflows.
- Tools: `create_checkpoint`, `clear_checkpoint`, `render`, `diff_checkpoints`,
  `diff_paths`, `dependencies`, `inventory`, `trace`.
- Prompts: `explain`, `refactor`, `diff_dirs`, `troubleshoot`.
- Provenance sidecars (`_tree.json`) with origin/transformer metadata;
  optional recursive Flux `Kustomization` rendering (`render` with
  `recursive=true`).
- Environment flags: `KUSTOMIZE_MCP_ROOT`, `KUSTOMIZE_LOAD_RESTRICTIONS`,
  `KUSTOMIZE_ENABLE_HELM`.
- `make` targets, GitHub Actions CI, golangci-lint, GoReleaser + multi-arch GHCR
  image, Dockerfile.

[Unreleased]: https://github.com/vgromanov/kustomize-mcp/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/vgromanov/kustomize-mcp/releases/tag/v0.1.0
