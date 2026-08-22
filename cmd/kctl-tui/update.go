// Copyright (c) 2026 Stefan Koelle (https://stefankoelle.de)
// Licensed under the MIT License. See LICENSE file in project root for details.

package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/creativeprojects/go-selfupdate"
)

const (
	githubSlug    = "skoelle/kctl-tui"
	updateTimeout = 10 * time.Second
)

func initUpdater(verbose bool) (*selfupdate.Updater, error) {
	if verbose {
		selfupdate.SetLogger(&verboseLogger{})
	}

	source, err := selfupdate.NewGitHubSource(selfupdate.GitHubConfig{})
	if err != nil {
		return nil, fmt.Errorf("failed to init GitHub source: %w", err)
	}

	return selfupdate.NewUpdater(selfupdate.Config{
		Source: source,
	})
}

func runUpdate(verbose bool) error {
	if version == "dev" {
		fmt.Fprintln(os.Stderr, "WARNING: running dev build — cannot compare versions")
		fmt.Fprintln(os.Stderr, "Skipping version check. Build from a tagged release to enable self-update.")
		return nil
	}

	updater, err := initUpdater(verbose)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), updateTimeout)
	defer cancel()

	repo := selfupdate.ParseSlug(githubSlug)
	rel, found, err := updater.DetectLatest(ctx, repo)
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}
	if !found {
		fmt.Println("Already up-to-date.")
		return nil
	}

	current, _ := semver.NewVersion(version)
	newVersion := rel.Version()

	newVer, _ := semver.NewVersion(newVersion)

	if current != nil && !current.LessThan(newVer) {
		fmt.Println("Already up-to-date.")
		return nil
	}

	fmt.Printf("Current version: %s\n", version)
	fmt.Printf("Found version %s. Updating...\n", newVersion)

	if err := updater.UpdateTo(ctx, rel, ""); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	fmt.Printf("Updated from %s to %s\n", current, newVersion)
	return nil
}

// checkForUpdateInteractive checks for a new version and prompts the user to update.
// Returns true if an update was applied.
func checkForUpdateInteractive(verbose bool) bool {
	if version == "dev" {
		return false
	}

	updater, err := initUpdater(verbose)
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Update check failed: %v\n", err)
		}
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), updateTimeout)
	defer cancel()

	repo := selfupdate.ParseSlug(githubSlug)
	rel, found, err := updater.DetectLatest(ctx, repo)
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Update check failed: %v\n", err)
		}
		return false
	}
	if !found {
		return false
	}

	current, _ := semver.NewVersion(version)
	newVersion := rel.Version()
	newVer, _ := semver.NewVersion(newVersion)

	if current != nil && !current.LessThan(newVer) {
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
	if err := updater.UpdateTo(ctx, rel, ""); err != nil {
		fmt.Fprintf(os.Stderr, "Update failed: %v\n", err)
		return false
	}

	fmt.Printf("Updated to %s. Please restart kctl-tui.\n", newVersion)
	return true
}

type verboseLogger struct{}

func (l *verboseLogger) Print(v ...any) {
	fmt.Fprint(os.Stderr, v...)
}

func (l *verboseLogger) Printf(format string, v ...any) {
	fmt.Fprintf(os.Stderr, format, v...)
}
