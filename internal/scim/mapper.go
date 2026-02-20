package scim

import (
	"encoding/json"
	"time"

	libscim "github.com/elimity-com/scim"
	"github.com/elimity-com/scim/optional"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

// toSCIMUser converts an internal UserTable to a SCIM Resource.
func toSCIMUser(u db.UserTable) libscim.Resource {
	attrs := libscim.ResourceAttributes{
		"userName":    u.UserID,
		"displayName": derefStr(u.UserAlias),
		"active":      isUserActive(u),
	}

	if u.UserEmail != nil {
		attrs["emails"] = []interface{}{
			map[string]interface{}{
				"value":   *u.UserEmail,
				"type":    "work",
				"primary": true,
			},
		}
	}

	var externalID optional.String
	meta := parseMetadata(u.Metadata)
	if eid, ok := meta["sso_user_id"].(string); ok {
		externalID = optional.NewString(eid)
	}
	if eid, ok := meta["externalId"].(string); ok {
		externalID = optional.NewString(eid)
	}

	var created, modified *time.Time
	if u.CreatedAt.Valid {
		t := u.CreatedAt.Time
		created = &t
	}
	if u.UpdatedAt.Valid {
		t := u.UpdatedAt.Time
		modified = &t
	}

	return libscim.Resource{
		ID:         u.UserID,
		ExternalID: externalID,
		Attributes: attrs,
		Meta: libscim.Meta{
			Created:      created,
			LastModified: modified,
		},
	}
}

// fromSCIMUser extracts CreateUserParams from SCIM resource attributes.
func fromSCIMUser(attrs libscim.ResourceAttributes) db.CreateUserParams {
	p := db.CreateUserParams{
		UserRole:  "internal_user",
		CreatedBy: "scim",
	}

	if userName, ok := attrs["userName"].(string); ok {
		p.UserID = userName
	}
	if dn, ok := attrs["displayName"].(string); ok {
		p.UserAlias = &dn
	}
	if emails, ok := attrs["emails"].([]interface{}); ok && len(emails) > 0 {
		if em, ok := emails[0].(map[string]interface{}); ok {
			if val, ok := em["value"].(string); ok {
				p.UserEmail = &val
			}
		}
	}

	return p
}

// toSCIMGroup converts an internal TeamTable to a SCIM Resource.
func toSCIMGroup(t db.TeamTable) libscim.Resource {
	attrs := libscim.ResourceAttributes{
		"displayName": derefStr(t.TeamAlias),
	}

	var members []interface{}
	for _, m := range t.Members {
		members = append(members, map[string]interface{}{
			"value":   m,
			"display": m,
		})
	}
	if members != nil {
		attrs["members"] = members
	}

	var externalID optional.String
	meta := parseMetadata(t.Metadata)
	if eid, ok := meta["externalId"].(string); ok {
		externalID = optional.NewString(eid)
	}

	var created, modified *time.Time
	if t.CreatedAt.Valid {
		ct := t.CreatedAt.Time
		created = &ct
	}
	if t.UpdatedAt.Valid {
		mt := t.UpdatedAt.Time
		modified = &mt
	}

	return libscim.Resource{
		ID:         t.TeamID,
		ExternalID: externalID,
		Attributes: attrs,
		Meta: libscim.Meta{
			Created:      created,
			LastModified: modified,
		},
	}
}

// fromSCIMGroup extracts CreateTeamParams from SCIM resource attributes.
func fromSCIMGroup(attrs libscim.ResourceAttributes) db.CreateTeamParams {
	p := db.CreateTeamParams{
		CreatedBy: "scim",
	}

	if dn, ok := attrs["displayName"].(string); ok {
		p.TeamAlias = &dn
	}

	if members, ok := attrs["members"].([]interface{}); ok {
		for _, m := range members {
			if mm, ok := m.(map[string]interface{}); ok {
				if val, ok := mm["value"].(string); ok {
					p.Members = append(p.Members, val)
				}
			}
		}
	}

	return p
}

// extractMembers gets member user IDs from SCIM group attributes.
func extractMembers(attrs libscim.ResourceAttributes) []string {
	members, ok := attrs["members"].([]interface{})
	if !ok {
		return nil
	}
	var ids []string
	for _, m := range members {
		if mm, ok := m.(map[string]interface{}); ok {
			if val, ok := mm["value"].(string); ok {
				ids = append(ids, val)
			}
		}
	}
	return ids
}

func isUserActive(u db.UserTable) bool {
	meta := parseMetadata(u.Metadata)
	if active, ok := meta["scim_active"]; ok {
		if b, ok := active.(bool); ok {
			return b
		}
	}
	return true // default active
}

func parseMetadata(data []byte) map[string]interface{} {
	if len(data) == 0 {
		return map[string]interface{}{}
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return map[string]interface{}{}
	}
	return m
}

func setMetadataField(existing []byte, key string, value interface{}) []byte {
	m := parseMetadata(existing)
	m[key] = value
	data, _ := json.Marshal(m)
	return data
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// extractExternalID gets the externalId from attributes if present.
func extractExternalID(attrs libscim.ResourceAttributes) optional.String {
	if eid, ok := attrs["externalId"].(string); ok {
		return optional.NewString(eid)
	}
	return optional.String{}
}
