package strategy

import (
	"github.com/praxisllmlab/tianjiLLM/internal/router"
)

// TagBased filters deployments by required tags, then delegates selection
// to an inner strategy. If no deployments match the tags, falls back to
// the inner strategy with all deployments.
type TagBased struct {
	inner router.Strategy
}

// NewTagBased creates a tag-based strategy wrapping an inner selection strategy.
func NewTagBased(inner router.Strategy) *TagBased {
	return &TagBased{inner: inner}
}

func (t *TagBased) Pick(deployments []*router.Deployment) *router.Deployment {
	return t.inner.Pick(deployments)
}

// PickWithTags filters deployments to those matching required tags,
// then delegates to the inner strategy.
// When matchAny is true, a deployment matches if it has ANY of the requested tags (OR logic).
// When matchAny is false, a deployment matches only if it has ALL requested tags (AND logic).
func (t *TagBased) PickWithTags(deployments []*router.Deployment, tags []string, matchAny bool) *router.Deployment {
	if len(tags) == 0 {
		return t.inner.Pick(deployments)
	}

	filtered := filterByTags(deployments, tags, matchAny)
	if len(filtered) == 0 {
		// Fall back to all deployments if no tag matches
		return t.inner.Pick(deployments)
	}

	return t.inner.Pick(filtered)
}

func filterByTags(deployments []*router.Deployment, tags []string, matchAny bool) []*router.Deployment {
	var result []*router.Deployment
	matcher := hasAllTags
	if matchAny {
		matcher = hasAnyTag
	}
	for _, d := range deployments {
		if d.Config == nil {
			continue
		}
		if matcher(d.Config.Tags, tags) {
			result = append(result, d)
		}
	}
	return result
}

func hasAllTags(have, want []string) bool {
	if len(want) == 0 {
		return true
	}
	set := make(map[string]struct{}, len(have))
	for _, t := range have {
		set[t] = struct{}{}
	}
	for _, t := range want {
		if _, ok := set[t]; !ok {
			return false
		}
	}
	return true
}

func hasAnyTag(have, want []string) bool {
	if len(want) == 0 {
		return true
	}
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
