package policy

import (
	"strings"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

// MatchRequest holds the multi-dimensional context to match against
// policy attachments.
type MatchRequest struct {
	TeamID string
	KeyID  string
	Model  string
	Tags   []string
}

// MatchAttachments returns all attachments that match the request.
// An attachment matches if ALL of its non-empty dimension arrays contain
// at least one match (AND across dimensions, OR within a dimension).
// Empty dimension arrays are treated as "match any".
// Global scope (scope="*") matches everything.
// Prefix wildcards use strings.HasPrefix (e.g., "openai/*" matches "openai/gpt-4").
func MatchAttachments(attachments []db.PolicyAttachmentTable, req MatchRequest) []db.PolicyAttachmentTable {
	var matched []db.PolicyAttachmentTable
	for _, a := range attachments {
		if matchAttachment(a, req) {
			matched = append(matched, a)
		}
	}
	return matched
}

func matchAttachment(a db.PolicyAttachmentTable, req MatchRequest) bool {
	// Global scope matches everything
	if a.Scope != nil && *a.Scope == "*" {
		return true
	}

	// Each non-empty dimension must have at least one match
	if len(a.Teams) > 0 && !matchDimension(a.Teams, req.TeamID) {
		return false
	}
	if len(a.Keys) > 0 && !matchDimension(a.Keys, req.KeyID) {
		return false
	}
	if len(a.Models) > 0 && !matchDimension(a.Models, req.Model) {
		return false
	}
	if len(a.Tags) > 0 && !matchAnyTag(a.Tags, req.Tags) {
		return false
	}
	return true
}

// matchDimension checks if value matches any pattern in the dimension.
// Supports prefix wildcard: "openai/*" matches "openai/gpt-4".
func matchDimension(patterns []string, value string) bool {
	for _, p := range patterns {
		if p == value {
			return true
		}
		if strings.HasSuffix(p, "*") {
			prefix := strings.TrimSuffix(p, "*")
			if strings.HasPrefix(value, prefix) {
				return true
			}
		}
	}
	return false
}

// matchAnyTag checks if any of the required tags exist in the have set.
func matchAnyTag(want, have []string) bool {
	set := make(map[string]struct{}, len(have))
	for _, t := range have {
		set[t] = struct{}{}
	}
	for _, t := range want {
		if _, ok := set[t]; ok {
			return true
		}
	}
	return false
}
