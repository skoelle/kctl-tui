# Contributing to kctl-tui

Thanks for your interest in contributing! This document explains how to get
started.

## Development Setup

```bash
git clone https://github.com/skoelle/kctl-tui.git
cd kctl-tui
go mod download
```

### Prerequisites

- Go 1.25+
- kubectl, k9s, tmux (or psmux on Windows)
- An active Kubernetes cluster for integration testing

### Running Locally

```bash
go run ./cmd/kctl-tui
```

### Building

```bash
go build -o kctl-tui ./cmd/kctl-tui
```

### With Version Tag

```bash
go build -ldflags "-X main.version=v0.2.0" -o kctl-tui ./cmd/kctl-tui
```

## Project Structure

```
cmd/kctl-tui/          Entry points (main, full mode, panel mode)
internal/config/       YAML config loading and template resolution
internal/kctl/         Pure logic (secret diffing, template engine)
internal/kubeexec/     kubectl/aws/tmux wrappers (side effects only)
```

### Architecture Rules

- **Pure logic** goes into `internal/kctl` or `internal/config` — no exec, no I/O.
- **Side effects** (running kubectl, aws, tmux) go into `internal/kubeexec`.
- **UI** lives in `cmd/kctl-tui/` — Bubble Tea models, views, handlers.
- Unit tests cover pure logic only. Side-effect packages are tested via
  integration/manual tests.

## Testing

```bash
go vet ./...       # static analysis
go test ./...      # unit tests
```

There are no integration tests yet. Manual testing against a real cluster is
expected for UI and kubeexec changes.

## Code Style

- Standard Go formatting (`gofmt`).
- No comments unless the logic is non-obvious.
- Error messages should be actionable — tell the user what to fix.
- Log commands with `kubeexec.VerboseLog()` when `--verbose` is active.

## Commits

- One logical change per commit.
- Imperative mood in commit messages ("Add ...", "Fix ...", "Remove ...").
- No co-authors in commits.

## Pull Requests

1. Fork the repo and create a feature branch.
2. Make your changes following the style guide above.
3. Run `go vet` and `go test`.
4. Open a PR against `main` with a clear description of what changed and why.
5. Reference any related issues.

## Issues

- Use the provided issue templates.
- Include your OS, Go version, and kctl-tui version.
- For bugs: steps to reproduce, expected vs actual behavior.

## License

By contributing, you agree that your contributions will be licensed under the
MIT License.
