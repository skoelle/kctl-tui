package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/skoelle/kctl-tui/internal/config"
	"github.com/skoelle/kctl-tui/internal/kubeexec"
)

// version is set via -ldflags at build time.
var version = "dev"

func main() {
	args := os.Args[1:]

	// Extract global flags before delegating to sub-commands.
	verbose := false
	filtered := make([]string, 0, len(args))
	for _, a := range args {
		switch a {
		case "--verbose":
			verbose = true
		case "--version", "-v":
			fmt.Printf("kctl-tui %s\n", version)
			return
		default:
			filtered = append(filtered, a)
		}
	}
	if verbose {
		kubeexec.SetVerbose(true, os.Stderr)
	}

	if len(filtered) > 0 {
		switch filtered[0] {
		case "panel":
			if err := runPanel(filtered[1:]); err != nil {
				fmt.Fprintln(os.Stderr, "kctl-tui panel error:", err)
				os.Exit(1)
			}
			return
		case "config":
			if err := runConfig(filtered[1:]); err != nil {
				fmt.Fprintln(os.Stderr, "kctl-tui config error:", err)
				os.Exit(1)
			}
			return
		}
	}

	m := newFullModel()
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "kctl-tui error:", err)
		os.Exit(1)
	}
}

func runConfig(args []string) error {
	if len(args) == 0 || args[0] != "check" {
		return fmt.Errorf("usage: kctl-tui config check")
	}
	cfgPath, err := config.DefaultPath()
	if err != nil {
		return fmt.Errorf("cannot determine config path: %w", err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("failed to load %s: %w", cfgPath, err)
	}

	ok := true
	if len(cfg.Contexts) == 0 {
		fmt.Fprintln(os.Stderr, "ERROR: no 'contexts' configured")
		ok = false
	}
	if len(cfg.Envs) == 0 {
		fmt.Fprintln(os.Stderr, "ERROR: no 'envs' configured")
		ok = false
	}
	if cfg.ContextTemplate == "" {
		fmt.Fprintln(os.Stderr, "ERROR: 'context_template' is empty")
		ok = false
	}
	if cfg.SecretNameTemplate == "" {
		fmt.Fprintln(os.Stderr, "ERROR: 'secret_name_template' is empty")
		ok = false
	}
	if cfg.TeamLabelKey == "" {
		fmt.Fprintln(os.Stderr, "WARNING: 'team_label_key' is empty — team selection will have no groups")
	}
	if cfg.AWSRegion == "" {
		fmt.Fprintln(os.Stderr, "WARNING: 'aws_region' is empty — secrets workflow will fail")
	}

	// Try resolving one context to verify the template works.
	if len(cfg.Contexts) > 0 && len(cfg.Envs) > 0 && cfg.ContextTemplate != "" {
		ctx := cfg.ResolveContext(cfg.Envs[0], cfg.Contexts[0])
		fmt.Printf("Resolved context example: %s\n", ctx)
	}

	if ok {
		fmt.Println("Config OK")
	} else {
		fmt.Fprintln(os.Stderr, "Config has errors — see above")
		os.Exit(1)
	}
	return nil
}
