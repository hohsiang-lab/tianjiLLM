package wildcard

import (
	"regexp"
	"strings"
)

// PatternToRegex converts a wildcard pattern like "claude-*" into a compiled
// regex with capture groups: "^claude-(.*)$". Each "*" becomes one capture group.
func PatternToRegex(pattern string) *regexp.Regexp {
	var b strings.Builder
	b.WriteByte('^')
	for _, ch := range pattern {
		switch ch {
		case '*':
			b.WriteString("(.*)")
		case '.', '+', '?', '(', ')', '[', ']', '{', '}', '\\', '^', '$', '|':
			b.WriteByte('\\')
			b.WriteRune(ch)
		default:
			b.WriteRune(ch)
		}
	}
	b.WriteByte('$')
	return regexp.MustCompile(b.String())
}

// Match tests modelName against a wildcard pattern containing "*".
// Returns the captured segments (one per "*") or nil if no match.
func Match(pattern, modelName string) []string {
	if !strings.Contains(pattern, "*") {
		return nil
	}
	re := PatternToRegex(pattern)
	m := re.FindStringSubmatch(modelName)
	if m == nil {
		return nil
	}
	return m[1:] // skip full match, return capture groups only
}

// ResolveModel replaces each "*" in modelTemplate with the
// corresponding captured segment, left to right.
// "anthropic/claude-*" + ["sonnet-4-5"] → "anthropic/claude-sonnet-4-5"
func ResolveModel(modelTemplate string, captured []string) string {
	result := modelTemplate
	for _, seg := range captured {
		result = strings.Replace(result, "*", seg, 1)
	}
	return result
}

// Specificity returns a sort key: longer patterns are more specific;
// among equal-length patterns, fewer wildcards win.
// Callers sort descending by length, ascending by wildcardCount.
func Specificity(pattern string) (length int, wildcardCount int) {
	return len(pattern), strings.Count(pattern, "*")
}
