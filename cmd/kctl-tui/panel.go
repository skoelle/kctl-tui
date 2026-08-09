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

	"github.com/skoelle/kctl-tui/internal/kctl"
	"github.com/skoelle/kctl-tui/internal/kubeexec"
)

// panelStep identifies which part of the redeploy/secrets wizard is shown.
type panelStep int

const (
	stepMenu panelStep = iota
	stepRedeployList
	stepRedeployConfirm
	stepSecretRegion
	stepSecretList
	stepK8sSecretName
	stepDiffResult
	stepForceSyncConfirm
	stepExternalSecretName
	stepDone
	stepError
)

type panelModel struct {
	ctx, ns, team string

	step  panelStep
	list  list.Model
	input textinput.Model

	awsRegion     string
	awsSecretID   string
	awsValues     map[string]string
	k8sSecretName string
	k8sValues     map[string]string
	diffEntries   []kctl.SecretDiffEntry

	message string
	err      error
}

func runPanel(args []string) error {
	fs := flag.NewFlagSet("panel", flag.ContinueOnError)
	ctx := fs.String("ctx", "", "kubectl context")
	ns := fs.String("ns", "", "namespace")
	team := fs.String("team", "", "team label value")
	if err := fs.Parse(args); err != nil {
		return err
	}

	m := newPanelModel(*ctx, *ns, *team)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func newPanelModel(ctx, ns, team string) *panelModel {
	l := list.New(menuItems(), list.NewDefaultDelegate(), 0, 0)
	l.Title = fmt.Sprintf("kctl-tui panel  [ctx=%s ns=%s team=%s]", ctx, ns, team)
	l.SetShowStatusBar(false)

	ti := textinput.New()
	ti.Focus()

	return &panelModel{ctx: ctx, ns: ns, team: team, step: stepMenu, list: l, input: ti}
}

func menuItems() []list.Item {
	return []list.Item{
		simpleItem{label: "Redeploy (rollout restart)", value: "redeploy"},
		simpleItem{label: "Secrets: AWS <-> Kubernetes diff", value: "secrets"},
		simpleItem{label: "Quit (closes this tmux session)", value: "quit"},
	}
}

func (m *panelModel) Init() tea.Cmd { return nil }

func (m *panelModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height-2)
		return m, nil

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
	switch m.step {
	case stepSecretRegion, stepK8sSecretName, stepExternalSecretName:
		return true
	}
	return false
}

// handleEsc closes the whole tmux session (all panes, including the two
// k9s status panes) before quitting this program, per SPEC.md 3.6.
func (m *panelModel) handleEsc() (tea.Model, tea.Cmd) {
	exec.Command("tmux", "kill-session", "-t", "kctl").Run()
	return m, tea.Quit
}

func (m *panelModel) handleEnter() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepMenu:
		return m.fromMenu()
	case stepRedeployList:
		return m.fromRedeployList()
	case stepRedeployConfirm:
		return m.fromRedeployConfirm()
	case stepSecretRegion:
		m.awsRegion = m.input.Value()
		return m.fetchSecretList()
	case stepSecretList:
		return m.fromSecretList()
	case stepK8sSecretName:
		m.k8sSecretName = m.input.Value()
		return m.compareAllFields()
	case stepForceSyncConfirm:
		return m.fromForceSyncConfirm()
	case stepExternalSecretName:
		return m.doForceSync()
	case stepDiffResult, stepDone, stepError:
		m.resetToMenu()
		return m, nil
	}
	return m, nil
}

func (m *panelModel) resetToMenu() {
	m.step = stepMenu
	m.list.SetItems(menuItems())
	m.list.Title = "kctl-tui panel"
	m.message = ""
	m.err = nil
}

// showError switches to a dedicated error screen so failures from
// kubectl/aws calls stay visible until the user explicitly acknowledges
// them with Enter, instead of being silently discarded.
func (m *panelModel) showError(err error) (tea.Model, tea.Cmd) {
	m.err = err
	m.step = stepError
	return m, nil
}

func (m *panelModel) fromMenu() (tea.Model, tea.Cmd) {
	item, ok := m.list.SelectedItem().(simpleItem)
	if !ok {
		return m, nil
	}
	switch item.value {
	case "redeploy":
		deployments, err := kubeexec.GetDeployments(m.ns)
		if err != nil {
			return m.showError(err)
		}
		items := make([]list.Item, 0, len(deployments))
		for _, d := range deployments {
			items = append(items, simpleItem{label: d, value: d})
		}
		m.list.SetItems(items)
		m.list.Title = "Select deployment to restart (esc = back)"
		m.step = stepRedeployList
	case "secrets":
		m.step = stepSecretRegion
		m.input.SetValue("eu-central-1")
		m.input.Placeholder = "AWS region"
	case "quit":
		return m.handleEsc()
	}
	return m, nil
}

func (m *panelModel) fromRedeployList() (tea.Model, tea.Cmd) {
	item, ok := m.list.SelectedItem().(simpleItem)
	if !ok {
		return m, nil
	}
	m.k8sSecretName = item.value // reused as "deployment name" here
	m.list.SetItems([]list.Item{
		simpleItem{label: "Yes, restart " + item.value, value: "yes"},
		simpleItem{label: "Cancel", value: "no"},
	})
	m.list.Title = "Confirm rollout restart"
	m.step = stepRedeployConfirm
	return m, nil
}

func (m *panelModel) fromRedeployConfirm() (tea.Model, tea.Cmd) {
	item, ok := m.list.SelectedItem().(simpleItem)
	if !ok || item.value != "yes" {
		m.resetToMenu()
		return m, nil
	}
	deployment := m.k8sSecretName // set in fromRedeployList
	_, err := kubeexec.RolloutRestart(m.ns, deployment)
	if err != nil {
		return m.showError(err)
	}
	status, err := kubeexec.RolloutStatus(m.ns, deployment)
	if err != nil {
		return m.showError(err)
	}
	m.message = "Rollout status: " + status
	m.step = stepDone
	return m, nil
}

// fetchSecretList lists all AWS Secrets Manager secrets in the chosen
// region so the user can pick one instead of typing the exact secret ID.
func (m *panelModel) fetchSecretList() (tea.Model, tea.Cmd) {
	names, err := kubeexec.ListAWSSecrets(m.awsRegion)
	if err != nil {
		return m.showError(err)
	}
	if len(names) == 0 {
		return m.showError(fmt.Errorf("no AWS secrets found in region %s (or missing IAM permissions)", m.awsRegion))
	}
	items := make([]list.Item, 0, len(names))
	for _, n := range names {
		items = append(items, simpleItem{label: n, value: n})
	}
	m.list.SetItems(items)
	m.list.Title = "Select AWS secret (esc = back to menu)"
	m.step = stepSecretList
	return m, nil
}

func (m *panelModel) fromSecretList() (tea.Model, tea.Cmd) {
	item, ok := m.list.SelectedItem().(simpleItem)
	if !ok {
		return m, nil
	}
	m.awsSecretID = item.value

	raw, err := kubeexec.GetAWSSecretString(m.awsSecretID, m.awsRegion)
	if err != nil {
		return m.showError(err)
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

	m.step = stepK8sSecretName
	m.input.SetValue("")
	m.input.Placeholder = "Kubernetes secret name (in namespace " + m.ns + ")"
	return m, nil
}

// compareAllFields fetches every field of the Kubernetes secret and diffs
// it against every key of the AWS secret in one go, instead of requiring
// the user to pick a single field.
func (m *panelModel) compareAllFields() (tea.Model, tea.Cmd) {
	k8sValues, err := kubeexec.GetSecretAllFields(m.ns, m.k8sSecretName)
	if err != nil {
		return m.showError(err)
	}
	m.k8sValues = k8sValues
	m.diffEntries = diffSecretValues(m.awsValues, m.k8sValues)
	m.message = renderDiffTable(m.awsSecretID, m.k8sSecretName, m.diffEntries)

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

func renderDiffTable(awsSecretID, k8sSecretName string, entries []kctl.SecretDiffEntry) string {
	var b strings.Builder
	fmt.Fprintf(&b, "AWS secret: %s   Kubernetes secret: %s\n\n", awsSecretID, k8sSecretName)
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
	_, err := kubeexec.AnnotateForceSync(m.ns, name, ts)
	if err != nil {
		return m.showError(err)
	}
	m.message += fmt.Sprintf("\nForce-sync requested for %s (timestamp %s).", name, strconv.FormatInt(ts, 10))
	m.step = stepDone
	return m, nil
}

func (m *panelModel) View() string {
	switch m.step {
	case stepMenu, stepRedeployList, stepRedeployConfirm, stepSecretList:
		return m.list.View()
	case stepForceSyncConfirm:
		return m.message + "\n\n" + m.list.View()
	case stepDiffResult, stepDone:
		return m.message + "\n\n(press enter to return to menu, esc to close session)"
	case stepError:
		errText := "unknown error"
		if m.err != nil {
			errText = m.err.Error()
		}
		return "ERROR:\n\n" + errText + "\n\n(press enter to return to menu, esc to close session)"
	default:
		return fmt.Sprintf("%s\n\n%s\n\n(enter = confirm, esc = back to menu/close session)",
			m.stepPrompt(), m.input.View())
	}
}

func (m *panelModel) stepPrompt() string {
	switch m.step {
	case stepSecretRegion:
		return "AWS region to list secrets from"
	case stepK8sSecretName:
		return "Kubernetes secret name (in namespace " + m.ns + ")"
	case stepExternalSecretName:
		return "ExternalSecret object name to annotate"
	}
	return ""
}
