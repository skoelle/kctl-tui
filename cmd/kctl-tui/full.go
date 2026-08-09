// Copyright (c) 2026 Stefan Koelle (https://stefankoelle.de)
// Licensed under the MIT License. See LICENSE file in project root for details.

package main

import (
	"fmt"
	"os"
	"os/exec"
	"sort"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/list"

	"github.com/skoelle/kctl-tui/internal/config"
	"github.com/skoelle/kctl-tui/internal/kubeexec"
)

// screenState identifies which selection step is currently shown.
type screenState int

const (
	screenContext screenState = iota
	screenTeam
	screenNamespace
)

// fullModel drives the interactive navigation. It starts directly at the
// team-selection screen using the configured default context, and only
// shows the context screen when the user explicitly goes back via Esc.
// Once a namespace is chosen, it launches the 3-pane tmux session
// (control pane + two k9s panes, one per configured env) via
// tea.ExecProcess.
type fullModel struct {
	list  list.Model
	state screenState

	cfg        config.Config
	namespaces map[string]map[string]string

	selectedContext   string
	selectedTeam      string
	selectedNamespace string

	statusMessage string
	err           error
}

func newFullModel() *fullModel {
	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	l.Title = "kctl-tui"
	l.SetShowStatusBar(false)
	return &fullModel{list: l}
}

func (m *fullModel) Init() tea.Cmd {
	return m.bootstrap
}

// bootstrap loads the config and applies the default context so the tool
// can jump straight to the team-selection screen.
func (m *fullModel) bootstrap() tea.Msg {
	cfgPath, _ := config.DefaultPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return errMsg{err}
	}
	if len(cfg.Contexts) == 0 {
		return errMsg{fmt.Errorf("no 'contexts' configured in ~/.kctl-tui/config.yaml (see config.example.yaml)")}
	}
	if len(cfg.Envs) == 0 {
		return errMsg{fmt.Errorf("no 'envs' configured in ~/.kctl-tui/config.yaml (see config.example.yaml)")}
	}
	if err := kubeexec.CheckTool("kubectl"); err != nil {
		return errMsg{err}
	}
	return bootstrapMsg{cfg: cfg, context: cfg.EffectiveDefaultContext()}
}

type bootstrapMsg struct {
	cfg     config.Config
	context string
}

type contextsLoadedMsg struct{ items []list.Item }

type teamsLoadedMsg struct {
	items      []list.Item
	namespaces map[string]map[string]string
}

type namespacesLoadedMsg struct{ items []list.Item }

type tmuxDoneMsg struct{ err error }

type errMsg struct{ err error }

func (m *fullModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height-2)
		return m, nil

	case errMsg:
		m.err = msg.err
		return m, nil

	case bootstrapMsg:
		m.cfg = msg.cfg
		m.selectedContext = msg.context
		return m, m.loadTeams

	case contextsLoadedMsg:
		m.state = screenContext
		m.list.Title = "Select context (enter = confirm, esc/ctrl+c = quit)"
		m.list.SetItems(msg.items)
		return m, nil

	case teamsLoadedMsg:
		m.namespaces = msg.namespaces
		m.state = screenTeam
		m.list.Title = fmt.Sprintf("Select team [context=%s]  (esc = back to context)", m.selectedContext)
		m.list.SetItems(msg.items)
		return m, nil

	case namespacesLoadedMsg:
		m.state = screenNamespace
		m.list.Title = "Select namespace (esc = back to team)"
		m.list.SetItems(msg.items)
		return m, nil

	case tmuxDoneMsg:
		if msg.err != nil {
			m.statusMessage = "tmux session ended with error: " + msg.err.Error()
		} else {
			m.statusMessage = "tmux session closed."
		}
		// Resume at the namespace screen so the user can pick a different
		// namespace right away, keeping context/team selection.
		return m, m.loadNamespacesFor(m.selectedTeam)

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			return m.handleBack()
		case "enter":
			return m.handleSelect()
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *fullModel) handleBack() (tea.Model, tea.Cmd) {
	switch m.state {
	case screenTeam:
		return m, m.loadContexts
	case screenNamespace:
		return m, m.loadTeamsFor(m.namespaces)
	default:
		return m, tea.Quit
	}
}

func (m *fullModel) loadContexts() tea.Msg {
	items := make([]list.Item, 0, len(m.cfg.Contexts))
	for _, c := range m.cfg.Contexts {
		label := c
		if c == m.selectedContext {
			label = "(current) " + c
		}
		items = append(items, simpleItem{label: label, value: c})
	}
	return contextsLoadedMsg{items: items}
}

func (m *fullModel) handleSelect() (tea.Model, tea.Cmd) {
	item, ok := m.list.SelectedItem().(simpleItem)
	if !ok {
		return m, nil
	}

	switch m.state {
	case screenContext:
		m.selectedContext = item.value
		return m, m.loadTeams

	case screenTeam:
		m.selectedTeam = item.value // may be "" meaning "no filter"
		return m, m.loadNamespacesFor(item.value)

	case screenNamespace:
		m.selectedNamespace = item.value
		if err := kubeexec.CheckTool("tmux"); err != nil {
			m.err = err
			return m, nil
		}
		if err := kubeexec.CheckTool("k9s"); err != nil {
			m.err = err
			return m, nil
		}
		return m, m.startTmuxSession()
	}
	return m, nil
}

// bootstrapContext resolves a kubectl context purely to discover
// namespaces/labels for the team/namespace screens. The first configured
// env is used as a stable default for this discovery step, since
// namespace names are assumed to be identical across envs.
func (m *fullModel) bootstrapContext() string {
	if len(m.cfg.Envs) == 0 {
		return ""
	}
	return m.cfg.ResolveContext(m.cfg.Envs[0], m.selectedContext)
}

func (m *fullModel) loadTeams() tea.Msg {
	namespaces, err := kubeexec.GetNamespacesWithLabels(m.bootstrapContext())
	if err != nil {
		return errMsg{err}
	}
	return *toTeamsLoadedMsg(namespaces, m.cfg.TeamLabelKey)
}

func (m *fullModel) loadTeamsFor(namespaces map[string]map[string]string) tea.Cmd {
	return func() tea.Msg {
		return *toTeamsLoadedMsg(namespaces, m.cfg.TeamLabelKey)
	}
}

func toTeamsLoadedMsg(namespaces map[string]map[string]string, labelKey string) *teamsLoadedMsg {
	values := distinctLabelValues(namespaces, labelKey)
	items := make([]list.Item, 0, len(values)+1)
	for _, v := range values {
		items = append(items, simpleItem{label: v, value: v})
	}
	items = append(items, simpleItem{label: "(no filter - all namespaces)", value: ""})
	return &teamsLoadedMsg{items: items, namespaces: namespaces}
}

func (m *fullModel) loadNamespacesFor(teamValue string) tea.Cmd {
	return func() tea.Msg {
		var names []string
		if teamValue == "" {
			for ns := range m.namespaces {
				names = append(names, ns)
			}
			sort.Strings(names)
		} else {
			names = namespacesForLabelValue(m.namespaces, m.cfg.TeamLabelKey, teamValue)
		}
		items := make([]list.Item, 0, len(names))
		for _, n := range names {
			items = append(items, simpleItem{label: n, value: n})
		}
		return namespacesLoadedMsg{items: items}
	}
}

// startTmuxSession builds the 3-pane tmux command: the control pane runs
// this binary in "panel" mode (letting the user pick an env and an
// action), and the two status panes run k9s against the first two
// configured envs, resolved via the context template, so both are
// visible side by side.
func (m *fullModel) startTmuxSession() tea.Cmd {
	selfPath, err := os.Executable()
	if err != nil {
		selfPath = "kctl-tui" // fallback to PATH lookup
	}
	panelCmd := fmt.Sprintf("%s panel --context=%s --ns=%s --team=%s",
		selfPath, m.selectedContext, m.selectedNamespace, m.selectedTeam)

	envA := m.cfg.Envs[0]
	ctxA := m.cfg.ResolveContext(envA, m.selectedContext)
	k9sCmdA := fmt.Sprintf("k9s --context %s -n %s", ctxA, m.selectedNamespace)

	// Kill stale session first (ignore error if none exists).
	killErr := exec.Command("tmux", "kill-session", "-t", "kctl").Run()
	fmt.Fprintf(os.Stderr, "[debug] selfPath=%s\n", selfPath)
	fmt.Fprintf(os.Stderr, "[debug] panelCmd=%s\n", panelCmd)
	fmt.Fprintf(os.Stderr, "[debug] kill-session err=%v\n", killErr)

	args := []string{
		"new-session", "-d", "-s", "kctl",
		panelCmd, ";",
		"set-option", "-t", "kctl", "remain-on-exit", "on", ";",
		"split-window", "-v", "-t", "kctl:0.0", k9sCmdA, ";",
	}
	if len(m.cfg.Envs) > 1 {
		envB := m.cfg.Envs[1]
		ctxB := m.cfg.ResolveContext(envB, m.selectedContext)
		k9sCmdB := fmt.Sprintf("k9s --context %s -n %s", ctxB, m.selectedNamespace)
		args = append(args,
			"split-window", "-v", "-t", "kctl:0.1", k9sCmdB, ";",
		)
	}
	args = append(args,
		"select-layout", "-t", "kctl", "even-vertical", ";",
		"select-pane", "-t", "kctl:0.0", ";",
		"attach", "-t", "kctl",
	)

	fmt.Fprintf(os.Stderr, "[debug] tmux args: %v\n", args)
	c := exec.Command("tmux", args...)

	return tea.ExecProcess(c, func(err error) tea.Msg {
		return tmuxDoneMsg{err: err}
	})
}

func (m *fullModel) View() string {
	view := m.list.View()
	if m.statusMessage != "" {
		view += "\n" + m.statusMessage
	}
	if m.err != nil {
		view += "\nerror: " + m.err.Error()
	}
	return view
}
