// Copyright (c) 2026 Stefan Koelle (https://stefankoelle.de)
// Licensed under the MIT License. See LICENSE file in project root for details.

package main

// simpleItem is a minimal implementation of list.Item used for all
// selection screens (contexts, teams, namespaces, menu actions).
type simpleItem struct {
	label string // what is shown to the user
	value string // the underlying value (context name, team value, ...)
}

func (i simpleItem) Title() string       { return i.label }
func (i simpleItem) Description() string { return "" }
func (i simpleItem) FilterValue() string { return i.label }
