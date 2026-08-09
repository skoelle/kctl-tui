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
	showHelp := false
	filtered := make([]string, 0, len(args))
	for _, a := range args {
		switch a {
		case "--verbose":
			verbose = true
		case "--version", "-v":
			fmt.Printf("kctl-tui %s\n", version)
			return
		case "--help", "-h":
			showHelp = true
		default:
			filtered = append(filtered, a)
		}
	}
	if verbose {
		kubeexec.SetVerbose(true, os.Stderr)
	}

	if showHelp || len(filtered) == 0 {
		printUsage()
		return
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
		case "doctor":
			if err := runDoctor(); err != nil {
				fmt.Fprintln(os.Stderr, "kctl-tui doctor error:", err)
				os.Exit(1)
			}
			return
		default:
			fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", filtered[0])
			printUsage()
			os.Exit(1)
		}
	}

	m := newFullModel()
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "kctl-tui error:", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`kctl-tui — Kubernetes entry-point TUI

Usage:
  kctl-tui [flags]             start the TUI (full navigation mode)
  kctl-tui doctor              check tools, config and connections
  kctl-tui config check        validate ~/.kctl-tui/config.yaml
  kctl-tui panel [options]     control pane (called internally by tmux)

Flags:
  --verbose    log all kubectl/aws commands to stderr
  --version    print version
  --help       show this help

Examples:
  kctl-tui                          # start the TUI
  kctl-tui doctor                   # verify everything is installed
  kctl-tui --verbose 2>debug.log    # log commands to a file
  kctl-tui config check             # validate config
`)
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

func runDoctor() error {
	pass := "ok"
	fail := "FAIL"
	warn := "WARN"
	status := pass
	errs := 0

	check := func(label string, err error) {
		if err != nil {
			fmt.Printf("  [%s] %s: %v\n", fail, label, err)
			status = fail
			errs++
		} else {
			fmt.Printf("  [%s] %s\n", pass, label)
		}
	}

	warnCheck := func(label string, err error) {
		if err != nil {
			fmt.Printf("  [%s] %s: %v\n", warn, label, err)
		} else {
			fmt.Printf("  [%s] %s\n", pass, label)
		}
	}

	// --- Tools ---
	fmt.Println("\nTools:")
	check("kubectl", kubeexec.CheckTool("kubectl"))
	check("tmux/psmux", kubeexec.CheckTool("tmux"))
	check("k9s", kubeexec.CheckTool("k9s"))
	warnCheck("aws CLI (optional)", kubeexec.CheckTool("aws"))

	// --- Config ---
	fmt.Println("\nConfig:")
	cfgPath, err := config.DefaultPath()
	if err != nil {
		fmt.Printf("  [%s] config path: %v\n", fail, err)
		errs++
		status = fail
	} else {
		cfg, err := config.Load(cfgPath)
		if err != nil {
			fmt.Printf("  [%s] load config: %v\n", fail, err)
			errs++
			status = fail
		} else {
			check("config file exists", nil)
			if len(cfg.Contexts) == 0 {
				fmt.Printf("  [%s] contexts configured\n", fail)
				errs++
				status = fail
			} else {
				fmt.Printf("  [%s] contexts configured (%d)\n", pass, len(cfg.Contexts))
			}
			if len(cfg.Envs) == 0 {
				fmt.Printf("  [%s] envs configured\n", fail)
				errs++
				status = fail
			} else {
				fmt.Printf("  [%s] envs configured (%d)\n", pass, len(cfg.Envs))
			}
			if cfg.ContextTemplate != "" {
				ctx := cfg.ResolveContext(cfg.Envs[0], cfg.Contexts[0])
				fmt.Printf("  [%s] context_template resolves to: %s\n", pass, ctx)
			} else {
				fmt.Printf("  [%s] context_template is empty\n", fail)
				errs++
				status = fail
			}
			if cfg.SecretNameTemplate != "" && len(cfg.Envs) > 0 {
				secret := cfg.ResolveSecretName("example-ns", cfg.Envs[0])
				fmt.Printf("  [%s] secret_name_template resolves to: %s\n", pass, secret)
			}
		}
	}

	// --- Connections ---
	fmt.Println("\nConnections:")
	if kubeexec.CheckTool("kubectl") == nil {
		err := kubeexec.CheckAWSAuth()
		if err == nil {
			fmt.Printf("  [%s] kubectl cluster reachable\n", pass)
		} else {
			// Not fatal — cluster might be unreachable from this machine
			fmt.Printf("  [%s] kubectl cluster: %v\n", warn, err)
		}
	}
	if kubeexec.CheckTool("aws") == nil {
		err := kubeexec.CheckAWSAuth()
		if err == nil {
			fmt.Printf("  [%s] AWS credentials valid\n", pass)
		} else {
			fmt.Printf("  [%s] AWS credentials: %v\n", warn, err)
		}
	} else {
		fmt.Printf("  [%s] AWS credentials (aws CLI not installed)\n", warn)
	}

	// --- Summary ---
	fmt.Println()
	if status == pass {
		fmt.Println("All checks passed. kctl-tui is ready to use.")
	} else {
		fmt.Printf("%d error(s) found. Fix the issues above and re-run: kctl-tui doctor\n", errs)
		os.Exit(1)
	}
	return nil
}
