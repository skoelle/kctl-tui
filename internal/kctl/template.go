package kctl

import "strings"

// ResolveTemplate replaces "{key}" placeholders in template with the
// corresponding value from values. Placeholders with no matching key are
// left untouched, so a misconfigured template is visible (e.g. a literal
// "{typo}" in the result) instead of silently collapsing to an empty
// string.
func ResolveTemplate(template string, values map[string]string) string {
	result := template
	for k, v := range values {
		result = strings.ReplaceAll(result, "{"+k+"}", v)
	}
	return result
}
