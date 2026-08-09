package main

import (
	"fmt"
	"os/exec"

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

// fullModel drives the interactive context -> team -> namespace navigation
// and, once a namespace is chosen, launches the 3-pane tmux session
// (control pane + two k9s panes) via tea.ExecProcess.
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
	return &fullModel{list: l, state: screenContext}
}

func (m *fullModel) Init() tea.Cmd {
	return m.loadContexts
}

func (m *fullModel) loadContexts() tea.Msg {
	contexts, err := kubeexec.GetContexts()
	if err != nil {
		return errMsg{err}
	}
	current := kubeexec.GetCurrentContext()

	cfgPath, _ := config.DefaultPath()
	cfg, _ := config.Load(cfgPath)

	items := make([]list.Item, 0, len(contexts))
	if current != "" {
		items = append(items, simpleItem{label: "(current) " + current, value: current})
	}
	for _, c := range contexts {
		if c != current {
			items = append(items, simpleItem{label: c, value: c})
		}
	}
	return contextsLoadedMsg{items: items, cfg: cfg}
}

type contextsLoadedMsg struct {
	items []list.Item
	cfg   config.Config
}

type teamsLoadedMsg struct {
	items      []list.Item
	namespaces map[string]map[string]string
}

type namespacesLoadedMsg struct {
	items []list.Item
}

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

	case contextsLoadedMsg:
		m.cfg = msg.cfg
		m.state = screenContext
		m.list.Title = "Select context (enter = confirm, esc/ctrl+c = quit)"
		m.list.SetItems(msg.items)
		return m, nil

	case teamsLoadedMsg:
		m.namespaces = msg.namespaces
		m.state = screenTeam
		m.list.Title = "Select team (esc = back to context)"
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

func (m *fullModel) handleSelect() (tea.Model, tea.Cmd) {
	item, ok := m.list.SelectedItem().(simpleItem)
	if !ok {
		return m, nil
	}

	switch m.state {
	case screenContext:
		m.selectedContext = item.value
		if err := kubeexec.UseContext(item.value); err != nil {
			return m, func() tea.Msg { return errMsg{err} }
		}
		return m, m.loadTeams

	case screenTeam:
		m.selectedTeam = item.value // may be "" meaning "no filter"
		return m, m.loadNamespacesFor(item.value)

	case screenNamespace:
		m.selectedNamespace = item.value
		if err := kubeexec.SetNamespace(item.value); err != nil {
			return m, func() tea.Msg { return errMsg{err} }
		}
		return m, m.startTmuxSession()
	}
	return m, nil
}

func (m *fullModel) loadTeams() tea.Msg {
	namespaces, err := kubeexec.GetNamespacesWithLabels()
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

// startTmuxSession builds the 3-pane tmux command (control pane running
// this binary in "panel" mode, plus two k9s status panes) and runs it via
// tea.ExecProcess so the Bubble Tea UI cleanly hands over the terminal.
func (m *fullModel) startTmuxSession() tea.Cmd {
	selfPath := "kctl-tui" // resolved via PATH; see README for install instructions
	panelCmd := fmt.Sprintf("%s panel --ctx=%s --ns=%s --team=%s",
		selfPath, m.selectedContext, m.selectedNamespace, m.selectedTeam)
	k9sCmdA := fmt.Sprintf("k9s --context %s -n %s", m.selectedContext, m.selectedNamespace)

	secondCtx := m.selectedContext
	if next, ok := findNextContext(m.selectedContext, m.cfg.ContextPairs); ok {
		secondCtx = next
	}
	k9sCmdB := fmt.Sprintf("k9s --context %s -n %s", secondCtx, m.selectedNamespace)

	c := exec.Command("tmux", "new-session", "-d", "-s", "kctl",
		panelCmd, ";",
		"split-window", "-v", k9sCmdA, ";",
		"split-window", "-v", k9sCmdB, ";",
		"select-layout", "main-horizontal", ";",
		"attach", "-t", "kctl",
	)

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
