package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "panel" {
		if err := runPanel(os.Args[2:]); err != nil {
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
