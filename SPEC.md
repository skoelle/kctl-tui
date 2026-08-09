# SPEC: kctl-tui — Kubernetes Entry-Point TUI

## 1. Goal

A single terminal tool as the central entry point for everyday Kubernetes
work, bundling the most common workflows currently done via long
`kubectl`/`k9s`/`aws-cli` commands, operable through a text UI (arrow keys,
Esc) instead of long typed commands.

Target platform: **Linux / WSL** (primary usage scenario, since split
panes require a real terminal multiplexer). Native Windows (without WSL)
is possible but with reduced split-view functionality (see 3.6).

**Technology decision: Go + Bubble Tea** (see section 5).

## 2. Terminology

| Term | Meaning |
|---|---|
| Context | Kubernetes cluster connection (`kubectl config get-contexts`) |
| Namespace | Logical subdivision within a cluster/context |
| Deployment | Controls the pods of a service; its name does not have to match the namespace name |
| Pod | Running instance, automatically generated name, not needed manually for the restart workflow |
| Rollout Restart | Rolling restart of all pods of a deployment according to the RollingUpdate strategy — **not** a simultaneous hard-kill of all pods, except with 1 replica or the `Recreate` strategy |
| Control pane | The top tmux pane running the Go tool itself (menu for redeploy/secrets) |
| Status panes | The two lower tmux panes, each running a `k9s` instance |
| Team label | Any freely configurable namespace label used to group namespaces by team/ownership (key and values are project-specific, kept generic here) |

## 3. Functional Requirements

### 3.1 Start navigation (hierarchical, with defaults)

Order at program start (outside tmux, "full" mode):

1. **Context selection** — list of all available contexts, currently
   active context pre-selected at the top as "(current)".
2. **Team selection** — list of all distinct values of a configurable
   namespace label (key freely configurable, e.g.
   `<organization>/<label-name>`), current team value pre-selected. A
   "no filter" option is available.
3. **Namespace selection** — filtered by the chosen team label value,
   active namespace pre-selected.
4. **Main view** — directly starts the tmux session with the three panes
   (see 3.3).

Navigation:

- **Esc** in the control pane first ends the whole tmux session
  (`tmux kill-session`, closing both k9s panes as well) and then moves the
  Go tool's screen stack one level up: namespace selection -> team
  selection -> context selection.

### 3.2 Namespace grouping via labels

- Display all currently assigned labels per namespace:
  `kubectl get ns --show-labels`.
- Query all distinct values of an arbitrary label key (including keys
  with a dot/slash) via bracket notation in JSONPath. Example with a
  generic placeholder key `<team-label-key>`:
  ```
  kubectl get ns -o jsonpath='{range .items[*]}{.metadata.labels["<team-label-key>"]}{"\n"}{end}' | sort -u
  ```
- The actual label key is project-specific and set via the configuration
  file (see `team_label_key` in config), not hardcoded.
- Namespace labeling is a prerequisite (one-time setup outside the tool).

### 3.3 Layout: 3-panel view

Once start navigation is complete, the tool opens a tmux session with
**three panes**, started with a single command:

```
+--------------------------------------------------+
|  Pane 0 (top): Control pane                      |
|  -> runs the kctl-tui binary in "panel" mode      |
|  -> menu: Redeploy, secrets diff                  |
+--------------------------------------------------+
|  Pane 1 (middle): k9s --context <context-a> -n <ns>|
+--------------------------------------------------+
|  Pane 2 (bottom): k9s --context <context-b> -n <ns>|
+--------------------------------------------------+
```

Important: a separate "status" menu item in the control pane is not
needed — status is continuously visible via the two k9s panes for as long
as the session runs. The control pane only contains the actions k9s does
not cover: **redeploy** and **secrets diff**.

Example startup command (generic placeholders):

```
# Kill stale session first (separate command — tmux aborts on kill-session error).
tmux kill-session -t kctl
tmux new-session -d -s kctl \
  "kctl-tui panel --context=$CTX_A --ns=$NS --team=$TEAM" \; \
  set-option -t kctl remain-on-exit on \; \
  split-window -v -t kctl:0.0 "k9s --context $CTX_A -n $NS" \; \
  split-window -v -t kctl:0.1 "k9s --context $CTX_B -n $NS" \; \
  select-layout -t kctl even-vertical \; \
  select-pane -t kctl:0.0 \; \
  attach -t kctl
```

Switching between panes: `Ctrl-b` + arrow key, or `Ctrl-b` `o`.

### 3.4 Redeploy (rollout restart) — in the control pane

- List all deployments in the currently selected namespace for selection
  (`kubectl -n <ns> get deploy`).
- Confirmation step before execution.
- Execute `kubectl -n <ns> rollout restart deploy/<name>` followed by
  `kubectl -n <ns> rollout status deploy/<name>`.
- The result is immediately visible in the status panes below (pods get
  recreated) — no separate status feedback channel needed in the tool
  itself.

### 3.5 AWS Secrets Manager <-> Kubernetes Secret diff — in the control pane

1. Before entering the secrets workflow, verify the AWS session is valid
   (`aws sts get-caller-identity`). If expired, offer to run the
   configured SSO login command interactively.
2. Resolve the AWS Secrets Manager secret ID from `secret_name_template`
   (using namespace + env) and the Kubernetes secret name from
   `k8s_secret_name_template` (using namespace). No manual input required
   for either name.
3. Fetch the AWS secret:
   `aws secretsmanager get-secret-value --secret-id <secret-id> --region <region> --query SecretString --output text`.
4. Fetch all fields of the Kubernetes secret and base64-decode them:
   `kubectl -n <ns> get secret <secret-name> -o json`.
5. Compare every field at once in a table (key / AWS value / Kubernetes
   value / match status).
6. On mismatch, optionally request a force-sync for the whole secret:
   `kubectl -n <ns> annotate externalsecret <name> force-sync=<unix-timestamp> --overwrite`.

The ExternalSecret object name for the force-sync annotation is the only
value asked for interactively at runtime.

### 3.6 Context resolution via templates

The actual kubectl context name/ARN for each environment is computed from
a configurable template at startup. The two k9s status panes and all
kubectl calls in the control pane use the resolved context.

Configuration format (e.g. `~/.kctl-tui/config.yaml`), purely illustrative
with generic placeholders:

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

context_template: "arn:aws:eks:{region}:{account_id}:cluster/tf-{env}-{context}-1"
secret_name_template: "tf-{namespace}-{env}-secrets"
k8s_secret_name_template: "{namespace}-common-secrets"

team_label_key: "<organization>/<label-name>"
```

The `context_template` replaces `{region}`, `{account_id}`, `{env}`, and
`{context}` placeholders with the configured values and the currently
selected environment/context. The resolved value must match an existing
context in your kubeconfig (e.g. added via `aws eks update-kubeconfig`).

**Windows support:** On native Windows, install
[psmux](https://github.com/marlocarlo/psmux) — a native, tmux-compatible
terminal multiplexer. psmux provides a `tmux` command, so kctl-tui works
without code changes (including `Esc`-triggered session termination).
Alternatively, run kctl-tui inside WSL with standard `tmux`.

## 4. Non-functional requirements

- **Primary platform Linux/WSL**, secondary native Windows (via psmux).
- **Single-binary distribution** without external runtime dependency (Go
  provides this natively).
- **External dependencies**: `kubectl` mandatory; `tmux`, `k9s`, `aws-cli`
  depending on the action used.
- **Low startup time**, noticeably faster than the current `kubens`
  experience.
- **No destructive actions without confirmation** (redeploy, force-sync).
- **No project- or company-specific names (contexts, namespaces, label
  keys, secret names) in code or example files** — everything is sourced
  via configuration or interactive input at runtime.

## 5. Technology decision: Go + Bubble Tea

Rationale:

- The Kubernetes tooling ecosystem (`kubectl`, `k9s`, `client-go`) is
  itself written in Go — enabling direct API access instead of pure shell
  calls in the future.
- Bubble Tea (Charm) is specifically built for hierarchical,
  keyboard-driven TUIs with a screen stack, same look/feel as k9s.
- Static, small binary without runtime dependency; fast cross-compilation
  (`GOOS=linux/windows`).
- Both Go and .NET are fundamentally cross-platform; since the real target
  is Linux/WSL anyway (due to the split-view requirement), .NET's main
  advantage (one runtime for both worlds in a single step) no longer
  applies, so the choice is free to fall on the technically closest
  ecosystem.

Rejected options (see discussion history):

- Python + Textual: good prototype, but requires an additional runtime, no
  single binary without extra effort (PyInstaller).
- .NET + Terminal.Gui: technically equivalent, but larger binary, no
  compelling advantage anymore with a Linux-only target.
- Rust + Ratatui: excellent TUI library, but a steeper learning curve
  without a clear extra benefit over Go for this project.

## 6. Already solved by standard tooling (no custom build needed)

- Generic fuzzy selection of context/namespace: `kubectx` + `kubens` +
  `fzf`.
- Ad-hoc actions directly from k9s with the current context/namespace/
  resource name: k9s plugins (`~/.k9s/plugin.yml`).
- Split panes themselves: tmux (Linux/WSL) or native Windows Terminal
  panes (`wt.exe split-pane`) — no custom multiplexer code needed, the
  tool only orchestrates existing tools.

## 7. Environment prerequisites (WSL-specific, from troubleshooting)

- **kubectx/kubens incompletely installed:** Some Linux distro packages
  for `kubectx` only ship the `kubectx` binary without `kubens`. Fix:
  install via the distribution's package manager properly, or manually
  symlink both scripts from the official project repository.
- **"kubeconfig file not found" in WSL:** The kubeconfig often only exists
  in the Windows user profile; WSL has its own, separate home directory.
  Fix via symlink:
  ```
  mkdir -p ~/.kube
  ln -s /mnt/c/Users/<windows-username>/.kube/config ~/.kube/config
  ```
  or via the `KUBECONFIG` environment variable pointing to the same
  Windows path.
- These fixes are a prerequisite before the tool can be meaningfully
  tested, since it builds directly on `kubectl config`.

## 8. Open items / out of scope (v1)

- No automatic label setup for namespaces (migration is a separate,
  one-time task).
- Split view v1 fixed at 2 status panes + 1 control pane (3 panes total).
- No RBAC/permission checks before executing sensitive actions — the tool
  assumes existing kubectl permissions.
- Configuration file format (`config.yaml`) is defined and implemented;
  concrete label keys, context names, and namespace names are
  project-specific and belong exclusively in the user's local, unversioned
  configuration, not in this document or the source code.
- Native Windows (without WSL) remains a secondary platform with manual
  pane handling instead of an automated tmux lifecycle.
