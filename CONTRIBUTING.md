# Contributing

Thank you for helping improve this project.

## Prerequisites

- [Go](https://go.dev/dl/) **1.25** or newer (see `go.mod`).

## Workflow

1. Open an issue or discuss larger changes before investing significant time.
2. Fork or branch from `main`.
3. Make focused commits; avoid unrelated refactors in the same change.
4. Run the standard loop before submitting:

   ```bash
   make fmt vet test
   ```

   Optionally run `make lint` if you have [golangci-lint](https://golangci-lint.run/)
   installed locally (CI runs it on every PR).

5. If you touched anything user-facing, update [README.md](README.md),
   [docs/tools.md](docs/tools.md), and [CHANGELOG.md](CHANGELOG.md) under the
   `Unreleased` section.
6. Open a pull request with a short description of the problem and the fix.

## Code style

- Run **`make fmt`** and **`make vet`** before submitting.
- Follow existing naming and package layout; keep public APIs minimal unless there is a clear need.
- Prefer tests that exercise real behavior (integration-style MCP tests live under `internal/mcpapp`).

## Testing

| Target        | Purpose |
|---------------|---------|
| `make test`   | All packages (`go test ./...`). |
| `make cover`  | Statement coverage for `internal/...` and a `coverage.out` profile (merge-friendly; avoids known issues covering `package main` on some Go toolchains). |
| `make race`   | Race detector on `internal/...` (slower). |

To inspect coverage in HTML:

```bash
make cover
go tool cover -html=coverage.out
```

## Releases

Releases are cut from `main` by pushing a `vX.Y.Z` tag. The
[`release` workflow](.github/workflows/release.yml) runs
[GoReleaser](https://goreleaser.com/) to publish:

- cross-platform archives + `SHA256SUMS` to GitHub Releases,
- a multi-arch Docker image to `ghcr.io/vgromanov/kustomize-mcp`.

Update `CHANGELOG.md` in the same commit as the tag.

## Reporting security issues

See [SECURITY.md](SECURITY.md). Do **not** open public issues for vulnerabilities.

## License

By contributing, you agree that your contributions will be licensed under the same terms as the project ([Apache License 2.0](LICENSE)).
