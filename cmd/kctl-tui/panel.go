package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os/exec"
	"strconv"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/skoelle/kctl-tui/internal/kubeexec"
)

// panelStep identifies which part of the redeploy/secrets wizard is shown.
type panelStep int

const (
	stepMenu panelStep = iota
	stepRedeployList
	stepRedeployConfirm
	stepSecretID
	stepSecretRegion
	stepSecretKeyList
	stepK8sSecretName
	stepK8sFieldName
	stepDiffResult
	stepForceSyncConfirm
	stepExternalSecretName
	stepDone
)

type panelModel struct {
	ctx, ns, team string

	step panelStep
	list list.Model
	input textinput.Model

	awsSecretID   string
	awsRegion     string
	awsValues     map[string]string
	selectedKey   string
	awsValue      string
	k8sSecretName string
	k8sFieldName  string
	k8sValue      string

	message string
	err     error
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
	case stepSecretID, stepSecretRegion, stepK8sSecretName, stepK8sFieldName, stepExternalSecretName:
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
	case stepSecretID:
		m.awsSecretID = m.input.Value()
		m.step = stepSecretRegion
		m.input.SetValue("eu-central-1")
		return m, nil
	case stepSecretRegion:
		m.awsRegion = m.input.Value()
		return m.fetchAWSSecret()
	case stepSecretKeyList:
		return m.fromSecretKeyList()
	case stepK8sSecretName:
		m.k8sSecretName = m.input.Value()
		m.step = stepK8sFieldName
		m.input.SetValue("")
		return m, nil
	case stepK8sFieldName:
		m.k8sFieldName = m.input.Value()
		return m.compareSecret()
	case stepForceSyncConfirm:
		return m.fromForceSyncConfirm()
	case stepExternalSecretName:
		return m.doForceSync()
	case stepDiffResult, stepDone:
		m.resetToMenu()
		return m, nil
	}
	return m, nil
}

func (m *panelModel) resetToMenu() {
	m.step = stepMenu
	m.list.SetItems(menuItems())
	m.list.Title = "kctl-tui panel"
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
			m.err = err
			return m, nil
		}
		items := make([]list.Item, 0, len(deployments))
		for _, d := range deployments {
			items = append(items, simpleItem{label: d, value: d})
		}
		m.list.SetItems(items)
		m.list.Title = "Select deployment to restart (esc = back)"
		m.step = stepRedeployList
	case "secrets":
		m.step = stepSecretID
		m.input.SetValue("")
		m.input.Placeholder = "AWS secret ID"
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
	m.selectedKey = item.value // reused as "deployment name" here
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
	_, err := kubeexec.RolloutRestart(m.ns, m.selectedKey)
	if err != nil {
		m.err = err
	}
	status, _ := kubeexec.RolloutStatus(m.ns, m.selectedKey)
	m.message = "Rollout status: " + status
	m.step = stepDone
	return m, nil
}

func (m *panelModel) fetchAWSSecret() (tea.Model, tea.Cmd) {
	raw, err := kubeexec.GetAWSSecretString(m.awsSecretID, m.awsRegion)
	if err != nil {
		m.err = err
		m.resetToMenu()
		return m, nil
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		m.awsValues = map[string]string{"__raw__": raw}
	} else {
		m.awsValues = map[string]string{}
		for k, v := range parsed {
			m.awsValues[k] = fmt.Sprintf("%v", v)
		}
	}
	items := make([]list.Item, 0, len(m.awsValues))
	for k := range m.awsValues {
		items = append(items, simpleItem{label: k, value: k})
	}
	m.list.SetItems(items)
	m.list.Title = "Select AWS secret key to compare"
	m.step = stepSecretKeyList
	return m, nil
}

func (m *panelModel) fromSecretKeyList() (tea.Model, tea.Cmd) {
	item, ok := m.list.SelectedItem().(simpleItem)
	if !ok {
		return m, nil
	}
	m.selectedKey = item.value
	m.awsValue = m.awsValues[item.value]
	m.step = stepK8sSecretName
	m.input.SetValue("")
	m.input.Placeholder = "Kubernetes secret name"
	return m, nil
}

func (m *panelModel) compareSecret() (tea.Model, tea.Cmd) {
	b64, err := kubeexec.GetSecretValueBase64(m.ns, m.k8sSecretName, m.k8sFieldName)
	if err != nil {
		m.err = err
		m.resetToMenu()
		return m, nil
	}
	decoded, err := kubeexec.DecodeBase64(b64)
	if err != nil {
		m.err = err
		m.resetToMenu()
		return m, nil
	}
	m.k8sValue = decoded

	if m.awsValue == m.k8sValue {
		m.message = "IDENTICAL\nAWS: " + m.awsValue + "\nK8s: " + m.k8sValue
		m.step = stepDiffResult
		return m, nil
	}

	m.message = "DIFFERENT\nAWS: " + m.awsValue + "\nK8s: " + m.k8sValue
	m.list.SetItems([]list.Item{
		simpleItem{label: "Yes, request force-sync", value: "yes"},
		simpleItem{label: "No", value: "no"},
	})
	m.list.Title = "Values differ - request ExternalSecret force-sync?"
	m.step = stepForceSyncConfirm
	return m, nil
}

func (m *panelModel) fromForceSyncConfirm() (tea.Model, tea.Cmd) {
	item, ok := m.list.SelectedItem().(simpleItem)
	if !ok || item.value != "yes" {
		m.step = stepDiffResult
		return m, nil
	}
	m.step = stepExternalSecretName
	m.input.SetValue("")
	m.input.Placeholder = "ExternalSecret object name"
	return m, nil
}

func (m *panelModel) doForceSync() (tea.Model, tea.Cmd) {
	name := m.input.Value()
	ts := time.Now().Unix()
	_, err := kubeexec.AnnotateForceSync(m.ns, name, ts)
	if err != nil {
		m.err = err
	}
	m.message = "Force-sync requested (timestamp " + strconv.FormatInt(ts, 10) + ")."
	m.step = stepDone
	return m, nil
}

func (m *panelModel) View() string {
	switch m.step {
	case stepMenu, stepRedeployList, stepRedeployConfirm, stepSecretKeyList, stepForceSyncConfirm:
		v := m.list.View()
		if m.err != nil {
			v += "\nerror: " + m.err.Error()
		}
		return v
	case stepDiffResult, stepDone:
		return m.message + "\n\n(press enter to return to menu, esc to close session)"
	default:
		return fmt.Sprintf("%s\n\n%s\n\n(enter = confirm, esc = back to menu/close session)",
			m.stepPrompt(), m.input.View())
	}
}

func (m *panelModel) stepPrompt() string {
	switch m.step {
	case stepSecretID:
		return "AWS Secrets Manager: enter secret ID"
	case stepSecretRegion:
		return "AWS region"
	case stepK8sSecretName:
		return "Kubernetes secret name (in namespace " + m.ns + ")"
	case stepK8sFieldName:
		return "Field name inside the Kubernetes secret for key \"" + m.selectedKey + "\""
	case stepExternalSecretName:
		return "ExternalSecret object name to annotate"
	}
	return ""
}
