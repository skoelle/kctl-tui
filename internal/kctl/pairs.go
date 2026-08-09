// Package kctl contains the core, non-interactive logic of kctl-tui.
// Functions here are pure (no kubectl/tmux side effects) so they can be
// unit tested without a live cluster.
package kctl

// ContextPair groups a set of related kubectl contexts that should be
// switchable via TAB while keeping the same namespace (e.g. staging/prod).
type ContextPair struct {
	Name     string   `yaml:"name"`
	Contexts []string `yaml:"contexts"`
}

// FindNextContext returns the next context in the same pair/group as
// current, cycling through the group. Returns "", false if current is not
// part of any configured pair.
func FindNextContext(current string, pairs []ContextPair) (string, bool) {
	for _, pair := range pairs {
		idx := indexOf(pair.Contexts, current)
		if idx == -1 {
			continue
		}
		next := pair.Contexts[(idx+1)%len(pair.Contexts)]
		return next, true
	}
	return "", false
}

func indexOf(items []string, target string) int {
	for i, v := range items {
		if v == target {
			return i
		}
	}
	return -1
}
