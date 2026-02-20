package scim

import (
	"context"
	"log"
	"net/http"

	libscim "github.com/elimity-com/scim"
	scimerrors "github.com/elimity-com/scim/errors"
	"github.com/google/uuid"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

// GroupHandler implements scim.ResourceHandler for SCIM Group resources.
// Maps SCIM Groups to internal TeamTable:
//   - displayName → team_alias
//   - members[].value → team members
//   - externalId → metadata["externalId"]
type GroupHandler struct {
	DB         *db.Queries
	UpsertUser bool // auto-create missing users when true
}

func (h *GroupHandler) Create(r *http.Request, attrs libscim.ResourceAttributes) (libscim.Resource, error) {
	ctx := r.Context()

	params := fromSCIMGroup(attrs)
	params.TeamID = uuid.NewString()

	// Validate or auto-create members
	if err := h.validateMembers(r, params.Members); err != nil {
		return libscim.Resource{}, err
	}

	team, err := h.DB.CreateTeam(ctx, params)
	if err != nil {
		return libscim.Resource{}, scimerrors.ScimErrorUniqueness
	}

	// Store externalId in metadata
	eid := extractExternalID(attrs)
	if eid.Present() {
		meta := setMetadataField(team.Metadata, "externalId", eid.Value())
		_ = h.DB.UpdateTeamMetadata(ctx, db.UpdateTeamMetadataParams{
			TeamID:   team.TeamID,
			Metadata: meta,
		})
		team.Metadata = meta
	}

	return toSCIMGroup(team), nil
}

func (h *GroupHandler) Get(r *http.Request, id string) (libscim.Resource, error) {
	team, err := h.DB.GetTeam(r.Context(), id)
	if err != nil {
		return libscim.Resource{}, scimerrors.ScimErrorResourceNotFound(id)
	}
	return toSCIMGroup(team), nil
}

func (h *GroupHandler) GetAll(r *http.Request, params libscim.ListRequestParams) (libscim.Page, error) {
	teams, err := h.DB.ListTeams(r.Context())
	if err != nil {
		return libscim.Page{}, scimerrors.ScimError{Status: 500, Detail: "failed to list teams"}
	}

	var filtered []db.TeamTable
	if params.FilterValidator != nil {
		for _, t := range teams {
			res := toSCIMGroup(t)
			if err := params.FilterValidator.PassesFilter(res.Attributes); err == nil {
				filtered = append(filtered, t)
			}
		}
	} else {
		filtered = teams
	}

	total := len(filtered)

	start := params.StartIndex - 1
	if start < 0 {
		start = 0
	}
	if start >= total {
		return libscim.Page{TotalResults: total, Resources: []libscim.Resource{}}, nil
	}
	end := start + params.Count
	if end > total {
		end = total
	}

	resources := make([]libscim.Resource, 0, end-start)
	for _, t := range filtered[start:end] {
		resources = append(resources, toSCIMGroup(t))
	}

	return libscim.Page{
		TotalResults: total,
		Resources:    resources,
	}, nil
}

func (h *GroupHandler) Replace(r *http.Request, id string, attrs libscim.ResourceAttributes) (libscim.Resource, error) {
	ctx := r.Context()

	_, err := h.DB.GetTeam(ctx, id)
	if err != nil {
		return libscim.Resource{}, scimerrors.ScimErrorResourceNotFound(id)
	}

	newMembers := extractMembers(attrs)
	if err = h.validateMembers(r, newMembers); err != nil {
		return libscim.Resource{}, err
	}

	var alias *string
	if dn, ok := attrs["displayName"].(string); ok {
		alias = &dn
	}

	team, err := h.DB.UpdateTeam(ctx, db.UpdateTeamParams{
		TeamID:    id,
		TeamAlias: alias,
		UpdatedBy: "scim",
	})
	if err != nil {
		return libscim.Resource{}, scimerrors.ScimError{Status: 500, Detail: "failed to update team"}
	}

	// Sync members: remove old, add new
	h.syncMembers(ctx, id, team.Members, newMembers)

	// Update metadata with externalId
	eid := extractExternalID(attrs)
	if eid.Present() {
		meta := setMetadataField(team.Metadata, "externalId", eid.Value())
		_ = h.DB.UpdateTeamMetadata(ctx, db.UpdateTeamMetadataParams{
			TeamID:   id,
			Metadata: meta,
		})
		team.Metadata = meta
	}

	// Re-fetch to get updated members
	team, err = h.DB.GetTeam(ctx, id)
	if err != nil {
		return libscim.Resource{}, scimerrors.ScimError{Status: 500, Detail: "failed to re-fetch team"}
	}

	return toSCIMGroup(team), nil
}

func (h *GroupHandler) Delete(r *http.Request, id string) error {
	_, err := h.DB.GetTeam(r.Context(), id)
	if err != nil {
		return scimerrors.ScimErrorResourceNotFound(id)
	}
	return h.DB.DeleteTeam(r.Context(), id)
}

func (h *GroupHandler) Patch(r *http.Request, id string, operations []libscim.PatchOperation) (libscim.Resource, error) {
	ctx := r.Context()

	_, err := h.DB.GetTeam(ctx, id)
	if err != nil {
		return libscim.Resource{}, scimerrors.ScimErrorResourceNotFound(id)
	}

	changed := false
	for _, op := range operations {
		pathStr := ""
		if op.Path != nil {
			pathStr = op.Path.String()
		}

		switch op.Op {
		case libscim.PatchOperationAdd:
			switch pathStr {
			case "members":
				members := extractPatchMembers(op.Value)
				if err = h.validateMembers(r, members); err != nil {
					return libscim.Resource{}, err
				}
				for _, m := range members {
					_ = h.DB.AddTeamMember(ctx, db.AddTeamMemberParams{
						TeamID:      id,
						ArrayAppend: m,
					})
				}
				changed = true
			case "displayName":
				if dn, ok := op.Value.(string); ok {
					_, _ = h.DB.UpdateTeam(ctx, db.UpdateTeamParams{
						TeamID:    id,
						TeamAlias: &dn,
						UpdatedBy: "scim",
					})
					changed = true
				}
			}

		case libscim.PatchOperationReplace:
			switch pathStr {
			case "displayName":
				if dn, ok := op.Value.(string); ok {
					_, _ = h.DB.UpdateTeam(ctx, db.UpdateTeamParams{
						TeamID:    id,
						TeamAlias: &dn,
						UpdatedBy: "scim",
					})
					changed = true
				}
			}

		case libscim.PatchOperationRemove:
			switch pathStr {
			case "members":
				members := extractPatchMembers(op.Value)
				for _, m := range members {
					_ = h.DB.RemoveTeamMember(ctx, db.RemoveTeamMemberParams{
						TeamID:      id,
						ArrayRemove: m,
					})
				}
				changed = true
			}
		}
	}

	if !changed {
		return libscim.Resource{}, nil // 204 No Content
	}

	team, err := h.DB.GetTeam(ctx, id)
	if err != nil {
		return libscim.Resource{}, scimerrors.ScimError{Status: 500, Detail: "failed to get team after patch"}
	}

	return toSCIMGroup(team), nil
}

// validateMembers checks that all member user_ids exist, or auto-creates them if UpsertUser is enabled.
func (h *GroupHandler) validateMembers(r *http.Request, members []string) error {
	ctx := r.Context()
	for _, userID := range members {
		_, err := h.DB.GetUser(ctx, userID)
		if err != nil {
			if !h.UpsertUser {
				return scimerrors.ScimError{
					Status: 400,
					Detail: "member user not found: " + userID + " (set scim_upsert_user: true to auto-create)",
				}
			}
			// Auto-create the user
			_, err = h.DB.CreateUser(ctx, db.CreateUserParams{
				UserID:    userID,
				UserRole:  "internal_user",
				CreatedBy: "scim_upsert",
			})
			if err != nil {
				log.Printf("scim: failed to auto-create user %s: %v", userID, err)
			}
		}
	}
	return nil
}

// syncMembers removes old members not in newMembers and adds new members not in oldMembers.
func (h *GroupHandler) syncMembers(ctx context.Context, teamID string, old, new_ []string) {
	oldSet := make(map[string]bool, len(old))
	for _, m := range old {
		oldSet[m] = true
	}
	newSet := make(map[string]bool, len(new_))
	for _, m := range new_ {
		newSet[m] = true
	}

	for _, m := range old {
		if !newSet[m] {
			_ = h.DB.RemoveTeamMember(ctx, db.RemoveTeamMemberParams{
				TeamID:      teamID,
				ArrayRemove: m,
			})
		}
	}
	for _, m := range new_ {
		if !oldSet[m] {
			_ = h.DB.AddTeamMember(ctx, db.AddTeamMemberParams{
				TeamID:      teamID,
				ArrayAppend: m,
			})
		}
	}
}

// extractPatchMembers gets member IDs from a patch operation value.
func extractPatchMembers(value interface{}) []string {
	switch v := value.(type) {
	case []interface{}:
		var ids []string
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				if val, ok := m["value"].(string); ok {
					ids = append(ids, val)
				}
			}
		}
		return ids
	case map[string]interface{}:
		if val, ok := v["value"].(string); ok {
			return []string{val}
		}
	}
	return nil
}
