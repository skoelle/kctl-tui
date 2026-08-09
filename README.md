# kctl-tui

A small terminal entry point for everyday Kubernetes work: pick a context
and a namespace once, then drive rollout restarts and an AWS Secrets
Manager <-> Kubernetes Secret diff/force-sync per environment from one
place instead of retyping long `kubectl` commands.

## Why

Working with several clusters, many namespaces per team, and paired
environments (e.g. beta/prod) quickly turns into a lot of repeated typing
with plain `kubectl`/`k9s`. kctl-tui adds:

- A guided **context -> team -> namespace** selection that starts
  directly at team selection (using a configured default context), with
  the context screen just one `Esc` away.
- Namespace grouping by an arbitrary, configurable **label** instead of
  scrolling through every namespace in the cluster.
- A **3-pane view** (via `tmux`): one control pane for actions, two status
  panes running `k9s` for the current namespace across your two
  configured environments (e.g. beta/prod), shown side by side.
- A control-pane menu organized **by environment**: pick beta or prod,
  then Secrets sync or Redeploy for that environment specifically.
- AWS Secrets Manager secret IDs and Kubernetes context names/ARNs are
  **computed from configurable templates** (namespace + environment),
  instead of listing secrets or discovering contexts live from
  `kubectl`/`aws-cli`.
- A guided **AWS Secrets Manager vs. Kubernetes Secret** comparison of
  every field at once, with a force-sync request for the whole secret if
  anything differs.
- An **AWS auth check** before the secrets workflow, offering to run your
  configured SSO login command interactively if the session has expired.

See [SPEC.md](SPEC.md) for the full requirements and design rationale, and
[PLAN.md](PLAN.md) for the implementation roadmap and current status.

## How it works

```
+--------------------------------------------------+
|  Control pane: kctl-tui panel                    |
|  -> 1) Quit  2) beta  3) prod                     |
|     each with: a) Secrets sync  b) Redeploy       |
+--------------------------------------------------+
|  k9s --context <resolved beta context>  -n <ns>   |
+--------------------------------------------------+
|  k9s --context <resolved prod context>  -n <ns>   |
+--------------------------------------------------+
```

1. Run `kctl-tui`. It loads your config, applies the default context, and
   jumps straight to team selection; press `Esc` there to pick a
   different context first.
2. Pick a team (namespace label filter), then a namespace.
3. It opens a `tmux` session with the layout above: the control pane runs
   this binary in "panel" mode, the two status panes run `k9s` against
   your first two configured environments (e.g. beta and prod), resolved
   from `context_template`.
4. In the control pane, pick an environment, then Secrets sync or
   Redeploy for that environment. `Esc` goes back one level (action menu
   -> environment menu -> closes the whole tmux session, including both
   `k9s` panes, and returns you to namespace selection).

## Requirements

- `kubectl`, configured with access to your cluster(s) (the actual
  context names/ARNs are resolved from your `context_template`, see
  Configuration below - they must already exist in your kubeconfig, e.g.
  added via `aws eks update-kubeconfig`).
- `k9s` (used for the two status panes).
- `tmux` (used for the 3-pane layout). On **Linux/macOS**, install
  `tmux` via your package manager. On **Windows**, install
  [psmux](https://github.com/marlocarlo/psmux) — a native,
  tmux-compatible terminal multiplexer:
  ```powershell
  scoop install psmux
  # or
  cargo install psmux
  ```
  psmux provides a `tmux` command, so kctl-tui works without changes.
- `aws` CLI, configured with credentials, only needed for the secrets
  workflow.

## Installation

### Quick install (Linux/macOS/WSL)

```bash
curl -fsSL https://raw.githubusercontent.com/skoelle/kctl-tui/main/install.sh | bash
```

### Quick install (Windows)

```powershell
irm https://raw.githubusercontent.com/skoelle/kctl-tui/main/install.ps1 | iex
```

This downloads the latest release binary for your architecture from
GitHub Releases and installs it to your PATH.

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
and adjust it to your own setup:

```yaml
contexts:
  - "internal"
  - "external"
default_context: "internal"

envs:
  - "beta"
  - "prod"

aws_region: "eu-central-1"
aws_account_id: "123456789012"

secret_name_template: "tf-{namespace}-{env}-secrets"
k8s_secret_name_template: "{namespace}-common-secrets"
context_template: "arn:aws:eks:{region}:{account_id}:cluster/tf-{env}-{context}-1"

team_label_key: "example.org/team"
aws_sso_login_command: "aws sso login"
```

- `contexts` / `default_context`: the top-level grouping the tool starts
  from (e.g. a network boundary such as internal/external-facing
  clusters). This is the outermost navigation level, one `Esc` above team
  selection.
- `envs`: the environments switchable from the control panel (e.g.
  "beta"/"prod"). The **first two** entries are also used for the two k9s
  status panes shown side by side.
- `aws_region` / `aws_account_id`: used for AWS Secrets Manager calls and
  to fill the `{account_id}` placeholder in `context_template`.
  `123456789012` is a placeholder, not a real account.
- `secret_name_template`: builds the AWS Secrets Manager secret ID from
  the chosen namespace and environment. Placeholders: `{namespace}`,
  `{env}`.
- `k8s_secret_name_template`: builds the Kubernetes secret name from the
  chosen namespace. Kept separate from `secret_name_template` because the
  two sides commonly follow different naming conventions. Placeholders:
  `{namespace}`.
- `context_template`: builds the actual kubectl context name/ARN from
  region, account ID, environment, and context. Placeholders: `{region}`,
  `{account_id}`, `{env}`, `{context}`. Adjust the literal parts (`tf-`,
  `-1`, cluster naming, ARN shape) to match how your own clusters/contexts
  are actually named — the resolved value must match an existing context
  in your kubeconfig.
- `team_label_key`: the Kubernetes namespace label used to group
  namespaces by team/ownership in the team-selection screen. This is
  entirely up to your organization's labeling convention; kctl-tui ships
  with no default team label of its own.
- `aws_sso_login_command`: run interactively if `aws sts
  get-caller-identity` fails before the secrets workflow (e.g. an expired
  SSO session). Defaults to `aws sso login`.

`~/.kctl-tui/config.yaml` is not part of this repository and should stay
that way — it typically contains your organization's internal account ID,
context naming, and label names.

## Windows notes

On native Windows (without WSL), install [psmux](https://github.com/marlocarlo/psmux)
for the 3-pane layout. psmux is a native Windows terminal multiplexer
that is tmux-compatible — kctl-tui works without code changes:

```powershell
scoop install psmux
# or
cargo install psmux
```

If you prefer WSL, symlink your kubeconfig into WSL:

```bash
mkdir -p ~/.kube
ln -s /mnt/c/Users/<your-windows-username>/.kube/config ~/.kube/config
```

## Usage

```bash
kctl-tui                    # start the TUI (full navigation mode)
kctl-tui --help             # show all commands and flags
kctl-tui --version          # print version
kctl-tui --verbose          # enable debug logging to stderr
kctl-tui doctor             # check if all tools, config and connections are OK
kctl-tui config check       # validate ~/.kctl-tui/config.yaml
kctl-tui panel --context=... --ns=... --team=...   # internal (called by tmux)
```

## Development

```bash
go test ./...
go vet ./...
go build ./cmd/kctl-tui
```

Pure logic (template resolution, label filtering, config parsing, secret
diffing) lives in `internal/kctl` and `internal/config` and is covered by
unit tests. Code that shells out to `kubectl`/`aws`/`tmux` lives in
`internal/kubeexec` and in `cmd/kctl-tui` and is intentionally kept thin
and untested, since it has no meaningful behavior without a live cluster.

## License

Licensed under the [MIT License](LICENSE) - Copyright (c) 2026 Stefan Koelle (https://stefankoelle.de)
