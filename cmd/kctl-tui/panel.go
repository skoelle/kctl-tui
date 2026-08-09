package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/skoelle/kctl-tui/internal/config"
	"github.com/skoelle/kctl-tui/internal/kctl"
	"github.com/skoelle/kctl-tui/internal/kubeexec"
)

// panelStep identifies which part of the env/action wizard is shown.
type panelStep int

const (
	stepEnvMenu panelStep = iota
	stepActionMenu
	stepRedeployList
	stepRedeployConfirm
	stepAWSAuthPrompt
	stepDiffResult
	stepForceSyncConfirm
	stepExternalSecretName
	stepDone
	stepError
)

type panelModel struct {
	context, ns, team string
	cfg               config.Config

	currentEnv string

	step  panelStep
	list  list.Model
	input textinput.Model

	deploymentName string

	awsSecretName string // resolved via secret_name_template (namespace + env)
	k8sSecretName string // resolved via k8s_secret_name_template (namespace only)
	awsValues     map[string]string
	k8sValues     map[string]string
	diffEntries   []kctl.SecretDiffEntry

	message string
	err      error
}

func runPanel(args []string) error {
	fs := flag.NewFlagSet("panel", flag.ContinueOnError)
	context := fs.String("context", "", "context (e.g. internal/external)")
	ns := fs.String("ns", "", "namespace")
	team := fs.String("team", "", "team label value")
	if err := fs.Parse(args); err != nil {
		return err
	}

	m := newPanelModel(*context, *ns, *team)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func newPanelModel(context, ns, team string) *panelModel {
	ti := textinput.New()
	ti.Focus()

	cfgPath, _ := config.DefaultPath()
	cfg, loadErr := config.Load(cfgPath)

	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	l.SetShowStatusBar(false)

	m := &panelModel{context: context, ns: ns, team: team, cfg: cfg, step: stepEnvMenu, list: l, input: ti}
	if loadErr != nil {
		m.err = fmt.Errorf("config load failed: %w", loadErr)
		m.step = stepError
	} else {
		m.showEnvMenu()
	}
	return m
}

func (m *panelModel) showEnvMenu() {
	items := make([]list.Item, 0, len(m.cfg.Envs)+1)
	items = append(items, simpleItem{label: "Quit (closes this tmux session)", value: "quit"})
	for _, env := range m.cfg.Envs {
		items = append(items, simpleItem{label: env, value: env})
	}
	m.list.SetItems(items)
	m.list.Title = fmt.Sprintf("kctl-tui panel  [context=%s ns=%s team=%s]", m.context, m.ns, m.team)
	m.step = stepEnvMenu
	m.currentEnv = ""
	m.message = ""
	m.err = nil
}

func (m *panelModel) showActionMenu() {
	m.list.SetItems([]list.Item{
		simpleItem{label: "Secrets sync (AWS <-> Kubernetes)", value: "secrets"},
		simpleItem{label: "Redeploy (rollout restart)", value: "redeploy"},
	})
	m.list.Title = fmt.Sprintf("kctl-tui panel  [context=%s ns=%s team=%s env=%s]  (esc = back)",
		m.context, m.ns, m.team, m.currentEnv)
	m.step = stepActionMenu
	m.message = ""
	m.err = nil
}

// resolvedContext returns the actual kubectl context/ARN for the
// currently selected env, built from the configured context_template.
func (m *panelModel) resolvedContext() string {
	return m.cfg.ResolveContext(m.currentEnv, m.context)
}

func (m *panelModel) Init() tea.Cmd { return nil }

type awsLoginDoneMsg struct{ err error }

func (m *panelModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height-2)
		return m, nil

	case awsLoginDoneMsg:
		return m.afterAWSLogin(msg.err)

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			return m.handleEsc()
		case "enter":
			return m.handleEnter()
		}
	}

	if m.usesTextInput() {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *panelModel) usesTextInput() bool {
	return m.step == stepExternalSecretName
}

// handleEsc navigates one level up: action menu -> env menu, most
// sub-steps -> action menu. From the top-level env menu it closes the
// whole tmux session (all panes, including the two k9s status panes)
// before quitting this program, per SPEC.md 3.6.
func (m *panelModel) handleEsc() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepEnvMenu:
		exec.Command("tmux", "kill-session", "-t", "kctl").Run()
		return m, tea.Quit
	case stepActionMenu:
		m.showEnvMenu()
		return m, nil
	default:
		m.showActionMenu()
		return m, nil
	}
}

func (m *panelModel) handleEnter() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepEnvMenu:
		return m.fromEnvMenu()
	case stepActionMenu:
		return m.fromActionMenu()
	case stepRedeployList:
		return m.fromRedeployList()
	case stepRedeployConfirm:
		return m.fromRedeployConfirm()
	case stepAWSAuthPrompt:
		return m.fromAWSAuthPrompt()
	case stepForceSyncConfirm:
		return m.fromForceSyncConfirm()
	case stepExternalSecretName:
		return m.doForceSync()
	case stepDiffResult, stepDone, stepError:
		m.showActionMenu()
		return m, nil
	}
	return m, nil
}

// showError switches to a dedicated error screen so failures from
// kubectl/aws calls stay visible until the user explicitly acknowledges
// them with Enter, instead of being silently discarded.
func (m *panelModel) showError(err error) (tea.Model, tea.Cmd) {
	m.err = err
	m.step = stepError
	return m, nil
}

func (m *panelModel) fromEnvMenu() (tea.Model, tea.Cmd) {
	item, ok := m.list.SelectedItem().(simpleItem)
	if !ok {
		return m, nil
	}
	if item.value == "quit" {
		return m.handleEsc()
	}
	m.currentEnv = item.value
	m.showActionMenu()
	return m, nil
}

func (m *panelModel) fromActionMenu() (tea.Model, tea.Cmd) {
	item, ok := m.list.SelectedItem().(simpleItem)
	if !ok {
		return m, nil
	}
	switch item.value {
	case "redeploy":
		deployments, err := kubeexec.GetDeployments(m.resolvedContext(), m.ns)
		if err != nil {
			return m.showError(err)
		}
		items := make([]list.Item, 0, len(deployments))
		for _, d := range deployments {
			items = append(items, simpleItem{label: d, value: d})
		}
		m.list.SetItems(items)
		m.list.Title = fmt.Sprintf("Select deployment to restart [env=%s]  (esc = back)", m.currentEnv)
		m.step = stepRedeployList
	case "secrets":
		return m.checkAWSAuthAndProceed()
	}
	return m, nil
}

// checkAWSAuthAndProceed verifies the current AWS credentials/SSO session
// before entering the secrets workflow. If the check fails (e.g. an
// expired SSO session), it offers to run the configured login command
// interactively instead of letting the user hit a confusing failure
// several steps later.
func (m *panelModel) checkAWSAuthAndProceed() (tea.Model, tea.Cmd) {
	if err := kubeexec.CheckAWSAuth(); err != nil {
		m.err = err
		m.list.SetItems([]list.Item{
			simpleItem{label: "Run AWS login now (" + m.cfg.LoginCommand() + ")", value: "login"},
			simpleItem{label: "Cancel", value: "cancel"},
		})
		m.list.Title = "AWS session invalid or expired"
		m.step = stepAWSAuthPrompt
		return m, nil
	}
	return m.startSecretsFlow()
}

func (m *panelModel) fromAWSAuthPrompt() (tea.Model, tea.Cmd) {
	item, ok := m.list.SelectedItem().(simpleItem)
	if !ok || item.value != "login" {
		m.showActionMenu()
		return m, nil
	}
	cmd := kubeexec.RunAWSLogin(m.cfg.LoginCommand())
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
		return awsLoginDoneMsg{err: err}
	})
}

// afterAWSLogin re-checks AWS auth once the interactive login command has
// finished (successfully or not) and either proceeds into the secrets
// workflow or shows the remaining error.
func (m *panelModel) afterAWSLogin(execErr error) (tea.Model, tea.Cmd) {
	if execErr != nil {
		return m.showError(fmt.Errorf("login command failed to run: %w", execErr))
	}
	if err := kubeexec.CheckAWSAuth(); err != nil {
		return m.showError(fmt.Errorf("still not authenticated with AWS after running '%s': %w", m.cfg.LoginCommand(), err))
	}
	return m.startSecretsFlow()
}

// startSecretsFlow computes the AWS secret ID (namespace + env) and the
// Kubernetes secret name (namespace only) from their respective
// templates and fetches the AWS side directly - no manual input required
// for either name.
func (m *panelModel) startSecretsFlow() (tea.Model, tea.Cmd) {
	m.awsSecretName = m.cfg.ResolveSecretName(m.ns, m.currentEnv)
	m.k8sSecretName = m.cfg.ResolveK8sSecretName(m.ns)

	raw, err := kubeexec.GetAWSSecretString(m.awsSecretName, m.cfg.AWSRegion)
	if err != nil {
		return m.showError(fmt.Errorf("failed to fetch AWS secret %q: %w", m.awsSecretName, err))
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		// Not a JSON secret - treat the whole value as a single field.
		m.awsValues = map[string]string{"value": raw}
	} else {
		m.awsValues = map[string]string{}
		for k, v := range parsed {
			m.awsValues[k] = fmt.Sprintf("%v", v)
		}
	}

	return m.compareAllFields()
}

func (m *panelModel) fromRedeployList() (tea.Model, tea.Cmd) {
	item, ok := m.list.SelectedItem().(simpleItem)
	if !ok {
		return m, nil
	}
	m.deploymentName = item.value
	m.list.SetItems([]list.Item{
		simpleItem{label: "Yes, restart " + item.value, value: "yes"},
		simpleItem{label: "Cancel", value: "no"},
	})
	m.list.Title = fmt.Sprintf("Confirm rollout restart [env=%s]", m.currentEnv)
	m.step = stepRedeployConfirm
	return m, nil
}

func (m *panelModel) fromRedeployConfirm() (tea.Model, tea.Cmd) {
	item, ok := m.list.SelectedItem().(simpleItem)
	if !ok || item.value != "yes" {
		m.showActionMenu()
		return m, nil
	}
	ctx := m.resolvedContext()
	_, err := kubeexec.RolloutRestart(ctx, m.ns, m.deploymentName)
	if err != nil {
		return m.showError(err)
	}
	status, err := kubeexec.RolloutStatus(ctx, m.ns, m.deploymentName)
	if err != nil {
		return m.showError(err)
	}
	m.message = fmt.Sprintf("[env=%s] Rollout status: %s", m.currentEnv, status)
	m.step = stepDone
	return m, nil
}

// compareAllFields fetches every field of the Kubernetes secret and diffs
// it against every key of the AWS secret in one go.
func (m *panelModel) compareAllFields() (tea.Model, tea.Cmd) {
	k8sValues, err := kubeexec.GetSecretAllFields(m.resolvedContext(), m.ns, m.k8sSecretName)
	if err != nil {
		return m.showError(fmt.Errorf("failed to fetch Kubernetes secret %q: %w", m.k8sSecretName, err))
	}
	m.k8sValues = k8sValues
	m.diffEntries = diffSecretValues(m.awsValues, m.k8sValues)
	m.message = renderDiffTable(m.currentEnv, m.awsSecretName, m.k8sSecretName, m.diffEntries)

	if anyMismatch(m.diffEntries) {
		m.list.SetItems([]list.Item{
			simpleItem{label: "Yes, request force-sync for this secret", value: "yes"},
			simpleItem{label: "No", value: "no"},
		})
		m.list.Title = "Values differ - request ExternalSecret force-sync?"
		m.step = stepForceSyncConfirm
	} else {
		m.step = stepDiffResult
	}
	return m, nil
}

func renderDiffTable(env, awsSecretName, k8sSecretName string, entries []kctl.SecretDiffEntry) string {
	var b strings.Builder
	fmt.Fprintf(&b, "env: %s   AWS secret: %s   Kubernetes secret: %s\n\n", env, awsSecretName, k8sSecretName)
	fmt.Fprintf(&b, "%-25s %-20s %-20s %s\n", "KEY", "AWS", "KUBERNETES", "STATUS")
	for _, e := range entries {
		status := "OK"
		if !e.Match {
			status = "MISMATCH"
		}
		fmt.Fprintf(&b, "%-25s %-20s %-20s %s\n", e.Key, truncate(e.Left, 20), truncate(e.Right, 20), status)
	}
	return b.String()
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func (m *panelModel) fromForceSyncConfirm() (tea.Model, tea.Cmd) {
	item, ok := m.list.SelectedItem().(simpleItem)
	if !ok || item.value != "yes" {
		m.step = stepDiffResult
		return m, nil
	}
	m.step = stepExternalSecretName
	m.input.SetValue(m.k8sSecretName)
	m.input.Placeholder = "ExternalSecret object name"
	return m, nil
}

func (m *panelModel) doForceSync() (tea.Model, tea.Cmd) {
	name := m.input.Value()
	ts := time.Now().Unix()
	_, err := kubeexec.AnnotateForceSync(m.resolvedContext(), m.ns, name, ts)
	if err != nil {
		return m.showError(err)
	}
	m.message += fmt.Sprintf("\nForce-sync requested for %s (timestamp %s).", name, strconv.FormatInt(ts, 10))
	m.step = stepDone
	return m, nil
}

func (m *panelModel) View() string {
	switch m.step {
	case stepEnvMenu, stepActionMenu, stepRedeployList, stepRedeployConfirm:
		v := m.list.View()
		if m.err != nil {
			v += "\nerror: " + m.err.Error()
		}
		return v
	case stepAWSAuthPrompt:
		errText := ""
		if m.err != nil {
			errText = m.err.Error() + "\n\n"
		}
		return errText + m.list.View()
	case stepForceSyncConfirm:
		return m.message + "\n\n" + m.list.View()
	case stepDiffResult, stepDone:
		return m.message + "\n\n(press enter to return to the action menu, esc to go back)"
	case stepError:
		errText := "unknown error"
		if m.err != nil {
			errText = m.err.Error()
		}
		return "ERROR:\n\n" + errText + "\n\n(press enter to return to the action menu, esc to go back)"
	default:
		return fmt.Sprintf("%s\n\n%s\n\n(enter = confirm, esc = back)",
			m.stepPrompt(), m.input.View())
	}
}

func (m *panelModel) stepPrompt() string {
	if m.step == stepExternalSecretName {
		return "ExternalSecret object name to annotate"
	}
	return ""
}
