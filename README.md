# kctl-tui

A small terminal entry point for everyday Kubernetes work: pick a context,
a team, and a namespace once, then drive status (via k9s), rollout
restarts, and an AWS Secrets Manager <-> Kubernetes Secret diff/force-sync
workflow from one place instead of retyping long `kubectl` commands.

## Why

Working with several clusters, many namespaces per team, and paired
environments (e.g. staging/production) quickly turns into a lot of repeated
typing with plain `kubectl`/`k9s`. kctl-tui adds:

- A guided **context -> team -> namespace** selection with sensible
  defaults (the currently active context/namespace is pre-selected).
- Namespace grouping by an arbitrary, configurable **label** instead of
  scrolling through every namespace in the cluster.
- A **3-pane view** (via `tmux`): one control pane for actions, two status
  panes running `k9s` for the current namespace across two related
  contexts.
- A guided **rollout restart** that lists deployments instead of requiring
  you to know/type the exact deployment name.
- A guided **AWS Secrets Manager vs. Kubernetes Secret** comparison,
  including an optional ExternalSecret force-sync annotation.

See [SPEC.md](SPEC.md) for the full requirements and design rationale, and
[PLAN.md](PLAN.md) for the implementation roadmap and current status.

## How it works

```
+--------------------------------------------------+
|  Control pane: kctl-tui panel                    |
|  -> Redeploy, Secrets diff/force-sync             |
+--------------------------------------------------+
|  k9s --context <context-a> -n <namespace>         |
+--------------------------------------------------+
|  k9s --context <context-b> -n <namespace>         |
+--------------------------------------------------+
```

1. Run `kctl-tui`. It walks you through context, team, and namespace
   selection.
2. Once a namespace is confirmed, it opens a `tmux` session with the layout
   above and attaches to it.
3. Inside the control pane you can trigger a rollout restart or compare/
   force-sync a secret. The two status panes keep showing live pod state
   via `k9s`, so there is no separate "status" menu entry.
4. Pressing `Esc` in the control pane closes the whole `tmux` session
   (including both `k9s` panes) and returns you to the namespace
   selection.
5. Pressing `Tab` in the control pane switches both status panes to the
   paired context configured in `context_pairs` (see Configuration),
   keeping the same namespace.

## Requirements

- `kubectl`, configured with access to your cluster(s).
- `k9s` (used for the two status panes).
- `tmux` (used for the 3-pane layout). On Windows, this means running
  kctl-tui inside **WSL** — `tmux` has no native Windows port. Native
  Windows Terminal has its own split-pane feature, but it cannot be
  scripted from inside a pane the way `tmux` can, so the automated 3-pane
  layout and the `Tab`/`Esc` session handling described above are only
  fully supported under Linux/WSL. See SPEC.md section 3.6 for details.
- `aws` CLI, configured with credentials, only needed for the secrets
  workflow.

## Installation

### Quick install (Linux/macOS/WSL)

```bash
curl -fsSL https://raw.githubusercontent.com/skoelle/kctl-tui/main/install.sh | bash
```

This downloads the latest release binary for your OS/architecture from
GitHub Releases and installs it to `/usr/local/bin/kctl-tui`.

### From source

```bash
git clone https://github.com/skoelle/kctl-tui.git
cd kctl-tui
go build -o kctl-tui ./cmd/kctl-tui
sudo mv kctl-tui /usr/local/bin/
```

Requires Go 1.22+.

### Prebuilt binaries

Every tagged release (`vX.Y.Z`) is built for `linux`, `darwin`, and
`windows`, each for `amd64` and `arm64`, via the GitHub Actions workflow in
[.github/workflows/build.yml](.github/workflows/build.yml). Download the
matching asset from the [Releases page](https://github.com/skoelle/kctl-tui/releases).

## Configuration

Copy [config.example.yaml](config.example.yaml) to `~/.kctl-tui/config.yaml`
and adjust it to your own cluster setup:

```yaml
context_pairs:
  - name: "example-environment-pair"
    contexts:
      - "example-context-a"
      - "example-context-b"

team_label_key: "example.org/team"
```

- `context_pairs`: groups of related `kubectl` contexts. `Tab` in the
  control pane cycles through the contexts of whichever group the current
  context belongs to.
- `team_label_key`: the Kubernetes namespace label used to group
  namespaces by team/ownership in the team-selection screen. This is
  entirely up to your organization's labeling convention; kctl-tui ships
  with no default team label of its own.

`~/.kctl-tui/config.yaml` is not part of this repository and should stay
that way — it typically contains your organization's internal context and
label names.

## WSL setup notes

If `kubectx`/`kubens` or `kctl-tui` report a missing kubeconfig inside WSL,
your kubeconfig most likely only exists on the Windows side. Symlink it
into WSL:

```bash
mkdir -p ~/.kube
ln -s /mnt/c/Users/<your-windows-username>/.kube/config ~/.kube/config
```

## Development

```bash
go test ./...
go vet ./...
go build ./cmd/kctl-tui
```

Pure logic (context-pair matching, label filtering, config parsing) lives
in `internal/kctl` and `internal/config` and is covered by unit tests. Code
that shells out to `kubectl`/`aws`/`tmux` lives in `internal/kubeexec` and
in `cmd/kctl-tui` and is intentionally kept thin and untested, since it has
no meaningful behavior without a live cluster.

## License

[MIT](LICENSE)
