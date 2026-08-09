package main

import "github.com/skoelle/kctl-tui/internal/kctl"

func distinctLabelValues(namespaces map[string]map[string]string, labelKey string) []string {
	return kctl.DistinctLabelValues(namespaces, labelKey)
}

func namespacesForLabelValue(namespaces map[string]map[string]string, labelKey, value string) []string {
	return kctl.NamespacesForLabelValue(namespaces, labelKey, value)
}

func findNextContext(current string, pairs []kctl.ContextPair) (string, bool) {
	return kctl.FindNextContext(current, pairs)
}

func diffSecretValues(left, right map[string]string) []kctl.SecretDiffEntry {
	return kctl.DiffSecretValues(left, right)
}

func anyMismatch(entries []kctl.SecretDiffEntry) bool {
	return kctl.AnyMismatch(entries)
}
