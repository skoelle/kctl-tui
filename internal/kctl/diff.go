package kctl

import "sort"

// SecretDiffEntry represents the comparison of one key between two secret
// sources (e.g. AWS Secrets Manager vs. a Kubernetes Secret).
type SecretDiffEntry struct {
	Key   string
	Left  string // e.g. the AWS Secrets Manager value
	Right string // e.g. the decoded Kubernetes secret value
	Match bool
}

// DiffSecretValues compares two key/value maps and returns a sorted list of
// diff entries covering the union of keys present in either map. A key that
// only exists on one side is still reported, with the missing side left as
// an empty string and Match set to false (unless both sides happen to be
// empty strings).
func DiffSecretValues(left, right map[string]string) []SecretDiffEntry {
	seen := map[string]bool{}
	for k := range left {
		seen[k] = true
	}
	for k := range right {
		seen[k] = true
	}

	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	result := make([]SecretDiffEntry, 0, len(keys))
	for _, k := range keys {
		l := left[k]
		r := right[k]
		result = append(result, SecretDiffEntry{Key: k, Left: l, Right: r, Match: l == r})
	}
	return result
}

// AnyMismatch reports whether at least one diff entry does not match.
func AnyMismatch(entries []SecretDiffEntry) bool {
	for _, e := range entries {
		if !e.Match {
			return true
		}
	}
	return false
}
