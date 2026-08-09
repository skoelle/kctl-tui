package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/skoelle/kctl-tui/internal/kubeexec"
)

func main() {
	args := os.Args[1:]

	// Extract --verbose before delegating to panel or full mode.
	verbose := false
	filtered := make([]string, 0, len(args))
	for _, a := range args {
		if a == "--verbose" {
			verbose = true
		} else {
			filtered = append(filtered, a)
		}
	}
	if verbose {
		kubeexec.SetVerbose(true, os.Stderr)
	}

	if len(filtered) > 0 && filtered[0] == "panel" {
		if err := runPanel(filtered[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "kctl-tui panel error:", err)
			os.Exit(1)
		}
		return
	}

	m := newFullModel()
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "kctl-tui error:", err)
		os.Exit(1)
	}
}
