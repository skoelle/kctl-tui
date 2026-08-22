// Copyright (c) 2026 Stefan Koelle (https://stefankoelle.de)
// Licensed under the MIT License. See LICENSE file in project root for details.

package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/creativeprojects/go-selfupdate"
)

const githubSlug = "skoelle/kctl-tui"

func runUpdate(verbose bool) error {
	if verbose {
		selfupdate.SetLogger(&verboseLogger{})
	}

	current := version
	if current == "dev" {
		fmt.Fprintln(os.Stderr, "WARNING: running dev build — cannot compare versions")
		fmt.Println("Skipping version check. Build from a tagged release to enable self-update.")
		return nil
	}

	fmt.Printf("Current version: %s\n", current)
	fmt.Println("Checking for updates...")

	rel, err := detectLatest()
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	if rel == nil {
		fmt.Println("Already up-to-date.")
		return nil
	}

	fmt.Printf("Found version %s. Updating...\n", rel.Version())

	if err := applyUpdate(rel); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	fmt.Printf("Updated from %s to %s\n", current, rel.Version())
	return nil
}

// checkForUpdateInteractive checks for a new version and prompts the user to update.
// Returns true if an update was applied.
func checkForUpdateInteractive(verbose bool) bool {
	if version == "dev" {
		return false
	}

	if verbose {
		selfupdate.SetLogger(&verboseLogger{})
	}

	rel, err := detectLatest()
	if err != nil {
		// Silently ignore network errors — don't block startup
		if verbose {
			fmt.Fprintf(os.Stderr, "Update check failed: %v\n", err)
		}
		return false
	}

	if rel == nil {
		return false
	}

	current, _ := semver.NewVersion(version)
	newVersion := rel.Version()

	if current != nil && !rel.GreaterThan(current.String()) {
		return false
	}

	fmt.Printf("New version %s available (current: %s). Update now? [y/N] ", newVersion, version)

	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))

	if answer != "y" && answer != "yes" {
		return false
	}

	fmt.Println("Updating...")
	if err := applyUpdate(rel); err != nil {
		fmt.Fprintf(os.Stderr, "Update failed: %v\n", err)
		return false
	}

	fmt.Printf("Updated to %s. Starting kctl-tui...\n", newVersion)
	return true
}

func detectLatest() (*selfupdate.Release, error) {
	source, err := selfupdate.NewGitHubSource(selfupdate.GitHubConfig{})
	if err != nil {
		return nil, fmt.Errorf("failed to init GitHub source: %w", err)
	}

	updater, err := selfupdate.NewUpdater(selfupdate.Config{
		Source: source,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create updater: %w", err)
	}

	repo := selfupdate.ParseSlug(githubSlug)
	rel, found, err := updater.DetectLatest(context.Background(), repo)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}

	return rel, nil
}

func applyUpdate(rel *selfupdate.Release) error {
	source, err := selfupdate.NewGitHubSource(selfupdate.GitHubConfig{})
	if err != nil {
		return fmt.Errorf("failed to init GitHub source: %w", err)
	}

	updater, err := selfupdate.NewUpdater(selfupdate.Config{
		Source: source,
	})
	if err != nil {
		return fmt.Errorf("failed to create updater: %w", err)
	}

	return updater.UpdateTo(context.Background(), rel, "")
}

type verboseLogger struct{}

func (l *verboseLogger) Print(v ...any) {
	fmt.Fprint(os.Stderr, v...)
}

func (l *verboseLogger) Printf(format string, v ...any) {
	fmt.Fprintf(os.Stderr, format, v...)
}

func init() {
	log.SetOutput(os.Stderr)
}
