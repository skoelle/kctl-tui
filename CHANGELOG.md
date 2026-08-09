# Changelog

All notable changes to kctl-tui will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/).

## [Unreleased]

## [0.2.0] - 2026-08-09

### Added
- **Windows support** via [psmux](https://github.com/marlocarlo/psmux) as tmux-compatible multiplexer
- `install.ps1` PowerShell install script for Windows
- `--help` flag with full usage documentation
- `--version` / `-v` flag (set via `-ldflags` at build time)
- `kctl-tui doctor` command to verify tools, config and cluster connectivity
- `kctl-tui config check` command to validate `~/.kctl-tui/config.yaml`
- `--verbose` flag for debug logging of all kubectl/aws commands to stderr
- Binary secret value detection (`IsBinary`) — base64 or non-UTF-8 content is flagged
- ExternalSecret name validation before force-sync
- `CheckTool()` and `CheckAWSAuth()` helpers for pre-flight checks
- Diff table scroll support (j/k, up/down arrows)
- `--command pods` flag for k9s to start directly in pod view
- `--namespace` flag for k9s (Windows compatibility)

### Fixed
- Tmux session cleanup: stale sessions are killed before creating new ones
- Panel path: uses `os.Executable()` instead of PATH lookup for correct binary
- k9s on Windows: `--namespace` instead of `-n`, direct pods view
- `--help` always shows usage and exits (no fallthrough to TUI)
- Unknown subcommands show error message + usage (exit 1)
- Panel quit: properly closes tmux session and exits
- Bubbles list filter disabled to prevent stuck filter state after tmux return

### Changed
- Config uses `k8s_secret_name_template` for Kubernetes secret names (separate from AWS `secret_name_template`)
- Contexts resolved via template (`context_template`) instead of live kubectl discovery
- Panel redesigned: env-first menu (quit/beta/prod) with secrets sync + redeploy
- Full-mode navigation starts at team selection with default context
- `tea.ExecProcess` only used for `tmux attach` — setup commands run synchronously

### Removed
- Context-pair logic (superseded by env-template based context resolution)
- Live kubectl discovery (superseded by config templates)

## [0.1.0] - 2026-07-XX

### Added
- Initial release with Bubble Tea TUI
- 3-pane tmux orchestration (control panel + 2x k9s)
- Team/namespace navigation with kubectl context switching
- Panel mode with redeploy and secrets diff/force-sync
- AWS SSO integration with interactive login prompt
- CI/CD pipeline with Go build and release
- Linux/macOS install script

[Unreleased]: https://github.com/skoelle/kctl-tui/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/skoelle/kctl-tui/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/skoelle/kctl-tui/releases/tag/v0.1.0
