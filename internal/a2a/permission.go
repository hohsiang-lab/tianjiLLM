package a2a

// AgentPermissionHandler determines which agents a user/team can access.
type AgentPermissionHandler struct {
	registry *AgentRegistry
}

// NewAgentPermissionHandler creates a permission handler.
func NewAgentPermissionHandler(registry *AgentRegistry) *AgentPermissionHandler {
	return &AgentPermissionHandler{registry: registry}
}

// GetAllowedAgents returns agent IDs the caller can access based on key+team model restrictions.
// Logic mirrors Python: team restricted + key restricted = intersect, team restricted + key unrestricted = team,
// team unrestricted = key, both unrestricted = all.
func (h *AgentPermissionHandler) GetAllowedAgents(keyModels, teamModels, agentAccessGroups []string) []string {
	all := h.registry.ListAgents()

	// Pre-filter by access groups if specified
	var candidates []*AgentConfig
	if len(agentAccessGroups) > 0 {
		groupSet := toSet(agentAccessGroups)
		for _, a := range all {
			if hasOverlap(a.AccessGroups, groupSet) {
				candidates = append(candidates, a)
			}
		}
	} else {
		candidates = all
	}

	keyRestricted := len(keyModels) > 0
	teamRestricted := len(teamModels) > 0

	switch {
	case keyRestricted && teamRestricted:
		keySet := toSet(keyModels)
		teamSet := toSet(teamModels)
		return filterAgentsByIntersection(candidates, keySet, teamSet)
	case teamRestricted:
		teamSet := toSet(teamModels)
		return filterAgentsBySet(candidates, teamSet)
	case keyRestricted:
		keySet := toSet(keyModels)
		return filterAgentsBySet(candidates, keySet)
	default:
		ids := make([]string, len(candidates))
		for i, a := range candidates {
			ids[i] = a.AgentID
		}
		return ids
	}
}

// IsAgentAllowed checks if a specific agent is in the allowed list.
func IsAgentAllowed(agentID string, allowed []string) bool {
	for _, id := range allowed {
		if id == agentID {
			return true
		}
	}
	return false
}

func toSet(items []string) map[string]bool {
	s := make(map[string]bool, len(items))
	for _, v := range items {
		s[v] = true
	}
	return s
}

func hasOverlap(items []string, set map[string]bool) bool {
	for _, v := range items {
		if set[v] {
			return true
		}
	}
	return false
}

func filterAgentsBySet(agents []*AgentConfig, set map[string]bool) []string {
	var result []string
	for _, a := range agents {
		if set[a.AgentName] || set[a.AgentID] {
			result = append(result, a.AgentID)
		}
	}
	return result
}

func filterAgentsByIntersection(agents []*AgentConfig, setA, setB map[string]bool) []string {
	var result []string
	for _, a := range agents {
		inA := setA[a.AgentName] || setA[a.AgentID]
		inB := setB[a.AgentName] || setB[a.AgentID]
		if inA && inB {
			result = append(result, a.AgentID)
		}
	}
	return result
}
