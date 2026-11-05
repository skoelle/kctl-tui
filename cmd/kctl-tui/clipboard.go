// Copyright (c) 2026 Stefan Koelle (https://stefankoelle.de)
// Licensed under the MIT License. See LICENSE file in project root for details.

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/atotto/clipboard"
	osc52 "github.com/aymanbagabas/go-osc52/v2"
	tea "github.com/charmbracelet/bubbletea"
)

// clipboardDoneMsg is emitted after a clipboard copy attempt so the model
// can surface a hint (or a hard failure) without blocking the UI.
type clipboardDoneMsg struct {
	hint string
	err  error
}

// copyToClipboard copies text to the system clipboard. OSC52 is used as the
// primary mechanism because it traverses tmux and SSH sessions to the outer
// terminal; the sequence is written to stderr so it does not interleave with
// bubbletea's renderer on stdout. If writing the sequence fails, it falls back
// to the native clipboard tooling (pbcopy/xclip/wl-copy/clip).
func copyToClipboard(text string) tea.Cmd {
	return func() tea.Msg {
		seq := osc52.New(text)
		hint := ""
		switch {
		case os.Getenv("TMUX") != "":
			// tmux forwards OSC52 to the outer terminal when set-clipboard
			// is on; allow-passthrough is only needed for wrapped sequences.
			hint = "If nothing was pasted, enable it in tmux: set -g set-clipboard on"
		case strings.Contains(os.Getenv("TERM"), "screen"):
			seq = seq.Screen()
		}

		if _, err := seq.WriteTo(os.Stderr); err != nil {
			if cerr := clipboard.WriteAll(text); cerr != nil {
				return clipboardDoneMsg{err: fmt.Errorf(
					"copy to clipboard failed (OSC52: %v; native clipboard: %w) — install xclip/xsel/wl-copy or enable OSC52 in your terminal", err, cerr)}
			}
			return clipboardDoneMsg{}
		}
		return clipboardDoneMsg{hint: hint}
	}
}
