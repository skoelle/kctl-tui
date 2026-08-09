# SPEC: kctl-tui — Kubernetes Entry-Point TUI

## 1. Goal

A single terminal tool as the central entry point for everyday Kubernetes
work, bundling the most common workflows currently done via long
`kubectl`/`k9s`/`aws-cli` commands, operable through a text UI (arrow keys,
Esc, Tab) instead of long typed commands.

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
- **Tab** in the control pane switches the context pair according to the
  context-pair pattern (see 3.6) for both status panes simultaneously; the
  namespace stays the same.

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
  file (see 3.6), not hardcoded.
- Namespace labeling is a prerequisite (one-time setup outside the tool).

### 3.3 Layout: 3-panel view (core design change vs. earlier drafts)

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
tmux new-session -d -s kctl \
  "kctl-tui panel --ctx=$CTX_A --ns=$NS --team=$TEAM" \; \
  split-window -v "k9s --context $CTX_A -n $NS" \; \
  split-window -v "k9s --context $CTX_B -n $NS" \; \
  select-layout main-horizontal \; \
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

1. Load the secret from AWS Secrets Manager:
   `aws secretsmanager get-secret-value --secret-id <secret-id> --region <region> --query SecretString --output text`.
2. Show the contained keys for selection.
3. Load the matching Kubernetes secret field:
   `kubectl -n <ns> get secret <secret-name> -o jsonpath='{.data.<field>}'`,
   base64-decode it.
4. Compare the values (identical / different).
5. On mismatch, optionally request a force-sync:
   `kubectl -n <ns> annotate externalsecret <name> force-sync=<unix-timestamp> --overwrite`.

All names (secret ID, secret name, field name, ExternalSecret name) are
asked for interactively at runtime, never hardcoded in the tool.

### 3.6 Context-pair pattern (configurable) — drives both status panes at once

Requirement: the Tab switch in the control pane must switch **both**
status panes below it, not just an internal state.

Configuration format (e.g. `~/.kctl-tui/config.yaml`), purely illustrative
with generic placeholders:

```yaml
context_pairs:
  - name: "environment-pair-1"
    contexts: ["<context-a1>", "<context-a2>"]
  - name: "environment-pair-2"
    contexts: ["<context-b1>", "<context-b2>"]

team_label_key: "<organization>/<label-name>"
```

Behavior on Tab in the control pane:

1. Determine the current context pair from the configuration.
2. Restart both k9s panes via
   `tmux respawn-pane -k -t kctl:0.1 "k9s --context <newA> -n <ns>"` and
   `... kctl:0.2 ...` (namespace stays the same).
3. If the current context is in no configured list: show a hint in the
   control pane instead of an error.
4. No action outside this configuration — no error, only a hint.

**Platform limitation on Windows without WSL:** `respawn-pane`/
`kill-session` are tmux-specific. Windows Terminal (`wt.exe`) offers no
equivalent scripting to replace panes or end the session from inside a
pane. On plain Windows (without WSL), only a simplified flow is possible:
k9s panes are closed manually (`q`, then `Ctrl+Shift+W`); Tab switching and
automatic session termination are unavailable there. This limitation is
the main reason the primary target system is set to Linux/WSL.

## 4. Non-functional requirements

- **Primary platform Linux/WSL**, secondary native Windows with reduced
  functionality.
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
- Configuration file format (`config.yaml`) is a proposal, not finally
  agreed; concrete label keys, context names, and namespace names are
  project-specific and belong exclusively in the user's local, unversioned
  configuration, not in this document or the source code.
- Native Windows (without WSL) remains a secondary platform with manual
  pane handling instead of an automated tmux lifecycle.
