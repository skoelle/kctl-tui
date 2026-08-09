# PLAN.md — Implementation Roadmap

This document tracks how kctl-tui is being built, phase by phase, and what
is still open. For the full requirements, see [SPEC.md](SPEC.md).

## Phase 0 — Repository setup (done)

- [x] Go module (`github.com/skoelle/kctl-tui`), `.gitignore`, MIT `LICENSE`.
- [x] GitHub Actions workflow: `go vet` + `go test` on every push/PR, plus a
      cross-platform build matrix (linux/darwin/windows x amd64/arm64) that
      attaches binaries to GitHub Releases on version tags.
- [x] `install.sh` for Linux/macOS/WSL, downloading the latest release
      asset, with clear diagnostics if no release exists yet or the GitHub
      API is unreachable.
- [x] `README.md`, `config.example.yaml`.

## Phase 1 — Core logic + navigation (done)

- [x] `internal/kctl`: pure, unit-tested logic —
      template resolution (`ResolveTemplate`), namespace/label filtering
      (`DistinctLabelValues`, `NamespacesForLabelValue`), and secret diffing
      (`DiffSecretValues`, `AnyMismatch`).
- [x] `internal/config`: YAML config loading with template-based context
      and secret name resolution (`ContextTemplate`, `SecretNameTemplate`,
      `K8sSecretNameTemplate`), with safe defaults when no config file
      exists yet.
- [x] `internal/kubeexec`: thin wrappers around `kubectl`/`aws` CLI calls
      (namespaces, deployments, rollout restart/status, fetching AWS
      secrets by template-resolved ID, reading all fields of a Kubernetes
      secret, ExternalSecret annotation, AWS auth check).
- [x] `cmd/kctl-tui` "full" mode: Bubble Tea navigation for
      context -> team -> namespace, with `Esc` correctly popping back one
      level at a time, defaults pre-selected from the currently active
      context/namespace.
- [x] On confirming a namespace, "full" mode launches the 3-pane `tmux`
      session (control pane + two `k9s` panes, `even-vertical` layout,
      `remain-on-exit` so a crashing control pane stays visible) via
      `tea.ExecProcess` and resumes at the namespace screen once the
      session ends.
- [x] `cmd/kctl-tui` "panel" mode:
      - Redeploy: pick a deployment from a list, confirm, then
        `rollout restart` + `rollout status`.
      - Secrets: AWS auth check with interactive SSO login fallback,
        then automatically resolve the AWS secret ID (from
        `secret_name_template`) and Kubernetes secret name (from
        `k8s_secret_name_template`), fetch both, diff **every field**
        in one table (key / AWS value / Kubernetes value / match status).
        If any field differs, offer a single force-sync request for the
        **whole secret** (one ExternalSecret annotation).
      - `Esc` closes the whole tmux session (`tmux kill-session`).

## Phase 2 — Hardening (done)

- [x] Handle non-JSON AWS secrets and Kubernetes secrets with binary
      (non-UTF8) values more gracefully in the diff table.
- [ ] Add integration-style tests against a local `kind`/`k3d` cluster in
      CI for the `kubeexec` wrappers currently excluded from automated
      testing. *(Deferred — superseded by client-go in v0.3.0)*
- [x] Input validation for the free-text steps in "panel" mode.
- [x] Graceful handling when `tmux`, `k9s`, or `aws` are not installed.
- [x] Structured logging / `--verbose` flag for troubleshooting failed
      `kubectl`/`aws` calls.
- [x] Paginate/scroll the secrets diff table for secrets with many fields.

## Phase 3 — Windows-native support (done)

- [x] Windows support via [psmux](https://github.com/marlocarlo/psmux).
- [x] `install.ps1` — PowerShell install script for Windows.
- [x] Updated README and SPEC with Windows + psmux setup instructions.

## Phase 4 — Nice-to-haves (done for v0.2.0)

- [x] `--version` flag — prints version, set via `-ldflags` at build time.
- [x] Config validation command (`kctl-tui config check`) — validates
      required fields and shows a resolved context example.
- [x] `kctl-tui doctor` — health check for tools, config, and connectivity.
- [x] `--help` flag with full usage documentation.
- [x] CHANGELOG.md, CONTRIBUTING.md, GitHub Issue/PR templates.

---

## Roadmap

### v0.3.0 — client-go integration

Replace kubectl shell-outs with direct API calls via `client-go`.

- [ ] Add `internal/kubeclient` package using `client-go` for:
      - Context/namespace/label queries (faster than kubectl JSON parsing)
      - Deployment list and rollout restart/status
      - Secret fetch (AWS Secrets Manager via SDK, K8s secrets via API)
      - ExternalSecret annotation update
- [ ] Keep `internal/kubeexec` as fallback for operations not yet
      covered by `client-go`
- [ ] Remove `kind`/`k3d` integration test plan (client-go has its own
      test coverage)
- [ ] Add unit tests with `fake.Clientset` for the new package

### v1.0 — Stable release

Production-ready with package manager support and documentation.

- [ ] Homebrew tap (`skoelle/homebrew-tap`) with `kctl-tui` formula
- [ ] Scoop manifest (`skoelle/scoop-bucket`) for Windows
- [ ] Full test coverage for `internal/kubeclient`
- [ ] Documentation: architecture diagram, config reference, troubleshooting
- [ ] Semantic versioning policy documented
- [ ] Deprecation policy for config schema changes

## Notes for contributors

- Keep any real organization-specific context names, namespace names,
  label keys, or secret names out of the repository. Use the generic
  placeholders already established in `SPEC.md` and
  `config.example.yaml`.
- Pure/testable logic belongs in `internal/kctl` and `internal/config`;
  anything that shells out to `kubectl`/`aws`/`tmux` belongs in
  `internal/kubeexec` or directly in `cmd/kctl-tui`, and should stay thin
  enough that it does not need its own test suite.
