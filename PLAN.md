# PLAN.md — Implementation Roadmap

This document tracks how kctl-tui is being built, phase by phase, and what
is still open. For the full requirements, see [SPEC.md](SPEC.md).

## Phase 0 — Repository setup (done)

- [x] Go module (`github.com/skoelle/kctl-tui`), `.gitignore`, MIT `LICENSE`.
- [x] GitHub Actions workflow: `go vet` + `go test` on every push/PR, plus a
      cross-platform build matrix (linux/darwin/windows x amd64/arm64) that
      attaches binaries to GitHub Releases on version tags.
- [x] `install.sh` for Linux/macOS/WSL, downloading the latest release
      asset.
- [x] `README.md`, `config.example.yaml`.

## Phase 1 — Core logic + navigation (done, initial version)

- [x] `internal/kctl`: pure, unit-tested logic —
      context-pair matching (`FindNextContext`) and namespace/label
      filtering (`DistinctLabelValues`, `NamespacesForLabelValue`).
- [x] `internal/config`: YAML config loading (`context_pairs`,
      `team_label_key`), with safe defaults when no config file exists yet.
- [x] `internal/kubeexec`: thin wrappers around `kubectl`/`aws` CLI calls
      (contexts, namespaces, deployments, rollout restart/status, secret
      read, ExternalSecret annotation).
- [x] `cmd/kctl-tui` "full" mode: Bubble Tea navigation for
      context -> team -> namespace, with `Esc` correctly popping back one
      level at a time, defaults pre-selected from the currently active
      context/namespace.
- [x] On confirming a namespace, "full" mode launches the 3-pane `tmux`
      session (control pane + two `k9s` panes) via `tea.ExecProcess` and
      resumes at the namespace screen once the session ends.
- [x] `cmd/kctl-tui` "panel" mode: menu for Redeploy and the AWS/Kubernetes
      secrets diff + force-sync wizard, with `Esc` closing the whole tmux
      session (`tmux kill-session`).

## Phase 2 — Hardening (open)

- [ ] Replace the hand-rolled AWS secret JSON parsing/`fmt.Sprintf` value
      formatting with a proper typed decode, and handle secrets that are
      plain strings rather than JSON.
- [ ] Add integration-style tests against a local `kind`/`k3d` cluster in
      CI for the `kubeexec` wrappers currently excluded from automated
      testing.
- [ ] Input validation for the free-text steps in "panel" mode (empty
      secret ID/region/name, invalid characters).
- [ ] Graceful handling when `tmux`, `k9s`, or `aws` are not installed
      (currently surfaces the raw exec error).
- [ ] Structured logging / `--verbose` flag for troubleshooting failed
      `kubectl` calls.

## Phase 3 — Windows-native support (open, secondary priority)

- [ ] Detect OS at runtime; on native Windows (no WSL), fall back to
      `wt.exe split-pane` instead of `tmux` for the status panes.
- [ ] Document/implement that `Tab`-based context switching and
      `Esc`-triggered session close are **not** available in the native
      Windows fallback, per SPEC.md 3.6 — the panes must be closed
      manually there.

## Phase 4 — Nice-to-haves (open, not committed)

- [ ] Optional direct use of `client-go` instead of shelling out to
      `kubectl`, for faster context/namespace/label queries.
- [ ] Config validation command (`kctl-tui config check`) that reports
      unknown label keys or context names not present in the current
      kubeconfig.
- [ ] Homebrew tap / `scoop` manifest as additional install options
      alongside `install.sh`.

## Notes for contributors

- Keep any real organization-specific context names, namespace names,
  label keys, or secret names out of the repository. Use the generic
  placeholders already established in `SPEC.md` and
  `config.example.yaml`.
- Pure/testable logic belongs in `internal/kctl` and `internal/config`;
  anything that shells out to `kubectl`/`aws`/`tmux` belongs in
  `internal/kubeexec` or directly in `cmd/kctl-tui`, and should stay thin
  enough that it does not need its own test suite.
