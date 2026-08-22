// Copyright (c) 2026 Stefan Koelle (https://stefankoelle.de)
// Licensed under the MIT License. See LICENSE file in project root for details.

package main

import (
	"context"
	"fmt"
	"log"
	"os"

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

	repo := selfupdate.ParseSlug(githubSlug)

	rel, err := updater.UpdateSelf(context.Background(), current, repo)
	if err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	if rel != nil && rel.Version() != current {
		fmt.Printf("Updated from %s to %s\n", current, rel.Version())
	} else {
		fmt.Println("Already up-to-date.")
	}

	return nil
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
