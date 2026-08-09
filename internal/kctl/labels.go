package kctl

import "sort"

// DistinctLabelValues returns the sorted, unique, non-empty values of the
// given label key across a set of namespaces (name -> labels map).
func DistinctLabelValues(namespaces map[string]map[string]string, labelKey string) []string {
	seen := map[string]bool{}
	for _, labels := range namespaces {
		if v, ok := labels[labelKey]; ok && v != "" {
			seen[v] = true
		}
	}
	result := make([]string, 0, len(seen))
	for v := range seen {
		result = append(result, v)
	}
	sort.Strings(result)
	return result
}

// NamespacesForLabelValue returns the sorted namespace names whose labelKey
// matches the given value exactly.
func NamespacesForLabelValue(namespaces map[string]map[string]string, labelKey, value string) []string {
	result := make([]string, 0)
	for ns, labels := range namespaces {
		if labels[labelKey] == value {
			result = append(result, ns)
		}
	}
	sort.Strings(result)
	return result
}
