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

// UserHandler implements scim.ResourceHandler for SCIM User resources.
// Maps SCIM Users to internal UserTable:
//   - userName → user_id
//   - externalId → metadata["sso_user_id"]
//   - active → metadata["scim_active"]
//   - emails[0].value → user_email
//   - displayName → user_alias
type UserHandler struct {
	DB SCIMStore
}

func (h *UserHandler) Create(r *http.Request, attrs libscim.ResourceAttributes) (libscim.Resource, error) {
	ctx := r.Context()

	params := fromSCIMUser(attrs)
	if params.UserID == "" {
		params.UserID = uuid.NewString()
	}

	eid := extractExternalID(attrs)

	user, err := h.DB.CreateUser(ctx, params)
	if err != nil {
		return libscim.Resource{}, scimerrors.ScimErrorUniqueness
	}

	// Store externalId and active state in metadata
	meta := setMetadataField(user.Metadata, "scim_active", true)
	if eid.Present() {
		meta = setMetadataField(meta, "sso_user_id", eid.Value())
	}
	_ = h.DB.UpdateUserMetadata(ctx, db.UpdateUserMetadataParams{
		UserID:   user.UserID,
		Metadata: meta,
	})
	user.Metadata = meta

	return toSCIMUser(user), nil
}

func (h *UserHandler) Get(r *http.Request, id string) (libscim.Resource, error) {
	user, err := h.DB.GetUser(r.Context(), id)
	if err != nil {
		return libscim.Resource{}, scimerrors.ScimErrorResourceNotFound(id)
	}
	return toSCIMUser(user), nil
}

func (h *UserHandler) GetAll(r *http.Request, params libscim.ListRequestParams) (libscim.Page, error) {
	users, err := h.DB.ListUsers(r.Context())
	if err != nil {
		return libscim.Page{}, scimerrors.ScimError{Status: 500, Detail: "failed to list users"}
	}

	// Apply filter if present
	var filtered []db.UserTable
	if params.FilterValidator != nil {
		for _, u := range users {
			res := toSCIMUser(u)
			if err := params.FilterValidator.PassesFilter(res.Attributes); err == nil {
				filtered = append(filtered, u)
			}
		}
	} else {
		filtered = users
	}

	total := len(filtered)

	// Apply pagination
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
	for _, u := range filtered[start:end] {
		resources = append(resources, toSCIMUser(u))
	}

	return libscim.Page{
		TotalResults: total,
		Resources:    resources,
	}, nil
}

func (h *UserHandler) Replace(r *http.Request, id string, attrs libscim.ResourceAttributes) (libscim.Resource, error) {
	ctx := r.Context()

	existing, err := h.DB.GetUser(ctx, id)
	if err != nil {
		return libscim.Resource{}, scimerrors.ScimErrorResourceNotFound(id)
	}

	// Extract fields from SCIM attributes
	var alias, email *string
	if dn, ok := attrs["displayName"].(string); ok {
		alias = &dn
	}
	if emails, ok := attrs["emails"].([]interface{}); ok && len(emails) > 0 {
		if em, ok := emails[0].(map[string]interface{}); ok {
			if val, ok := em["value"].(string); ok {
				email = &val
			}
		}
	}

	user, err := h.DB.UpdateUser(ctx, db.UpdateUserParams{
		UserID:    id,
		UserAlias: alias,
		UserEmail: email,
		UserRole:  existing.UserRole,
		UpdatedBy: "scim",
	})
	if err != nil {
		return libscim.Resource{}, scimerrors.ScimError{Status: 500, Detail: "failed to update user"}
	}

	// Update metadata: externalId and active state
	meta := user.Metadata
	eid := extractExternalID(attrs)
	if eid.Present() {
		meta = setMetadataField(meta, "sso_user_id", eid.Value())
	}
	if active, ok := attrs["active"].(bool); ok {
		meta = setMetadataField(meta, "scim_active", active)
		if !active {
			h.deactivateUser(ctx, id)
		}
	}
	if len(meta) > 0 {
		_ = h.DB.UpdateUserMetadata(ctx, db.UpdateUserMetadataParams{
			UserID:   id,
			Metadata: meta,
		})
		user.Metadata = meta
	}

	return toSCIMUser(user), nil
}

func (h *UserHandler) Delete(r *http.Request, id string) error {
	_, err := h.DB.GetUser(r.Context(), id)
	if err != nil {
		return scimerrors.ScimErrorResourceNotFound(id)
	}
	h.deactivateUser(r.Context(), id)
	return h.DB.DeleteUser(r.Context(), id)
}

func (h *UserHandler) Patch(r *http.Request, id string, operations []libscim.PatchOperation) (libscim.Resource, error) {
	ctx := r.Context()

	existing, err := h.DB.GetUser(ctx, id)
	if err != nil {
		return libscim.Resource{}, scimerrors.ScimErrorResourceNotFound(id)
	}

	meta := existing.Metadata
	needsUpdate := false
	var alias, email *string

	for _, op := range operations {
		pathStr := ""
		if op.Path != nil {
			pathStr = op.Path.String()
		}

		switch op.Op {
		case libscim.PatchOperationReplace, libscim.PatchOperationAdd:
			switch pathStr {
			case "active":
				if active, ok := op.Value.(bool); ok {
					meta = setMetadataField(meta, "scim_active", active)
					needsUpdate = true
					if !active {
						h.deactivateUser(ctx, id)
					}
				}
			case "displayName":
				if dn, ok := op.Value.(string); ok {
					alias = &dn
					needsUpdate = true
				}
			case "userName":
				// userName is immutable (it's the user_id)
			case "emails":
				if emails, ok := op.Value.([]interface{}); ok && len(emails) > 0 {
					if em, ok := emails[0].(map[string]interface{}); ok {
						if val, ok := em["value"].(string); ok {
							email = &val
							needsUpdate = true
						}
					}
				}
			case "externalId":
				if eid, ok := op.Value.(string); ok {
					meta = setMetadataField(meta, "sso_user_id", eid)
					needsUpdate = true
				}
			default:
				// No path — bulk replace from value map
				if pathStr == "" {
					if valueMap, ok := op.Value.(map[string]interface{}); ok {
						for k, v := range valueMap {
							switch k {
							case "active":
								if active, ok := v.(bool); ok {
									meta = setMetadataField(meta, "scim_active", active)
									needsUpdate = true
									if !active {
										h.deactivateUser(ctx, id)
									}
								}
							case "displayName":
								if dn, ok := v.(string); ok {
									alias = &dn
									needsUpdate = true
								}
							}
						}
					}
				}
			}
		case libscim.PatchOperationRemove:
			// No removable fields for user
		}
	}

	if !needsUpdate {
		return libscim.Resource{}, nil // 204 No Content
	}

	// Update DB fields
	user, err := h.DB.UpdateUser(ctx, db.UpdateUserParams{
		UserID:    id,
		UserAlias: alias,
		UserEmail: email,
		UserRole:  existing.UserRole,
		UpdatedBy: "scim",
	})
	if err != nil {
		return libscim.Resource{}, scimerrors.ScimError{Status: 500, Detail: "failed to patch user"}
	}

	// Update metadata
	_ = h.DB.UpdateUserMetadata(ctx, db.UpdateUserMetadataParams{
		UserID:   id,
		Metadata: meta,
	})
	user.Metadata = meta

	return toSCIMUser(user), nil
}

// deactivateUser revokes all API keys for the user.
func (h *UserHandler) deactivateUser(ctx context.Context, userID string) {
	uid := &userID
	tokens, err := h.DB.ListVerificationTokensByUser(ctx, uid)
	if err != nil {
		log.Printf("scim: failed to list tokens for user %s: %v", userID, err)
		return
	}
	for _, tok := range tokens {
		if err := h.DB.BlockVerificationToken(ctx, tok.Token); err != nil {
			log.Printf("scim: failed to block token %s: %v", tok.Token, err)
		}
	}
}
