package strategy

import (
	"strings"

	"github.com/praxisllmlab/tianjiLLM/internal/router"
)

// FilterByRegion returns deployments whose Region starts with the allowed prefix.
// For example, allowedRegion "eu" matches "eu-west1", "eu-central1".
// Returns all deployments if no match found or allowedRegion is empty.
func FilterByRegion(deployments []*router.Deployment, allowedRegion string) []*router.Deployment {
	if allowedRegion == "" {
		return deployments
	}

	var filtered []*router.Deployment
	for _, d := range deployments {
		if d.Region != "" && strings.HasPrefix(d.Region, allowedRegion) {
			filtered = append(filtered, d)
		}
	}

	if len(filtered) == 0 {
		return deployments
	}
	return filtered
}
