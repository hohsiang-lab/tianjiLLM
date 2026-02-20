package scim

import (
	"testing"

	libscim "github.com/elimity-com/scim"
	"github.com/elimity-com/scim/optional"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/stretchr/testify/assert"
)

func TestToSCIMUser(t *testing.T) {
	alias := "Test User"
	email := "test@example.com"

	user := db.UserTable{
		UserID:    "user-1",
		UserAlias: &alias,
		UserEmail: &email,
		UserRole:  "internal_user",
		Metadata:  []byte(`{"scim_active": true, "sso_user_id": "ext-123"}`),
		CreatedAt: pgtype.Timestamptz{Valid: false},
		UpdatedAt: pgtype.Timestamptz{Valid: false},
	}

	res := toSCIMUser(user)

	assert.Equal(t, "user-1", res.ID)
	assert.Equal(t, "Test User", res.Attributes["displayName"])
	assert.Equal(t, "user-1", res.Attributes["userName"])
	assert.Equal(t, true, res.Attributes["active"])
	assert.True(t, res.ExternalID.Present())
	assert.Equal(t, "ext-123", res.ExternalID.Value())

	emails, ok := res.Attributes["emails"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, emails, 1)
	em, _ := emails[0].(map[string]interface{})
	assert.Equal(t, "test@example.com", em["value"])
}

func TestToSCIMUserInactive(t *testing.T) {
	user := db.UserTable{
		UserID:   "user-2",
		Metadata: []byte(`{"scim_active": false}`),
	}

	res := toSCIMUser(user)
	assert.Equal(t, false, res.Attributes["active"])
}

func TestToSCIMUserNoMetadata(t *testing.T) {
	user := db.UserTable{
		UserID: "user-3",
	}

	res := toSCIMUser(user)
	assert.Equal(t, true, res.Attributes["active"]) // default active
	assert.False(t, res.ExternalID.Present())
}

func TestFromSCIMUser(t *testing.T) {
	attrs := libscim.ResourceAttributes{
		"userName":    "john.doe",
		"displayName": "John Doe",
		"emails": []interface{}{
			map[string]interface{}{
				"value":   "john@example.com",
				"type":    "work",
				"primary": true,
			},
		},
	}

	params := fromSCIMUser(attrs)

	assert.Equal(t, "john.doe", params.UserID)
	assert.Equal(t, "John Doe", *params.UserAlias)
	assert.Equal(t, "john@example.com", *params.UserEmail)
	assert.Equal(t, "internal_user", params.UserRole)
	assert.Equal(t, "scim", params.CreatedBy)
}

func TestToSCIMGroup(t *testing.T) {
	alias := "Engineering"
	team := db.TeamTable{
		TeamID:    "team-1",
		TeamAlias: &alias,
		Members:   []string{"user-1", "user-2"},
		Metadata:  []byte(`{"externalId": "ext-team-1"}`),
	}

	res := toSCIMGroup(team)

	assert.Equal(t, "team-1", res.ID)
	assert.Equal(t, "Engineering", res.Attributes["displayName"])
	assert.True(t, res.ExternalID.Present())
	assert.Equal(t, "ext-team-1", res.ExternalID.Value())

	members, ok := res.Attributes["members"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, members, 2)
}

func TestFromSCIMGroup(t *testing.T) {
	attrs := libscim.ResourceAttributes{
		"displayName": "Platform Team",
		"members": []interface{}{
			map[string]interface{}{"value": "user-a"},
			map[string]interface{}{"value": "user-b"},
		},
	}

	params := fromSCIMGroup(attrs)

	assert.Equal(t, "Platform Team", *params.TeamAlias)
	assert.Equal(t, []string{"user-a", "user-b"}, params.Members)
	assert.Equal(t, "scim", params.CreatedBy)
}

func TestExtractMembers(t *testing.T) {
	attrs := libscim.ResourceAttributes{
		"members": []interface{}{
			map[string]interface{}{"value": "u1", "display": "User 1"},
			map[string]interface{}{"value": "u2"},
		},
	}

	members := extractMembers(attrs)
	assert.Equal(t, []string{"u1", "u2"}, members)
}

func TestExtractMembersEmpty(t *testing.T) {
	attrs := libscim.ResourceAttributes{}
	members := extractMembers(attrs)
	assert.Nil(t, members)
}

func TestSetMetadataField(t *testing.T) {
	data := setMetadataField(nil, "key", "value")
	m := parseMetadata(data)
	assert.Equal(t, "value", m["key"])

	data = setMetadataField(data, "key2", true)
	m = parseMetadata(data)
	assert.Equal(t, "value", m["key"])
	assert.Equal(t, true, m["key2"])
}

func TestParseMetadataEmpty(t *testing.T) {
	m := parseMetadata(nil)
	assert.NotNil(t, m)
	assert.Len(t, m, 0)
}

func TestParseMetadataInvalid(t *testing.T) {
	m := parseMetadata([]byte("not json"))
	assert.NotNil(t, m)
	assert.Len(t, m, 0)
}

func TestExtractExternalID(t *testing.T) {
	attrs := libscim.ResourceAttributes{
		"externalId": "ext-123",
	}
	eid := extractExternalID(attrs)
	assert.True(t, eid.Present())
	assert.Equal(t, "ext-123", eid.Value())
}

func TestExtractExternalIDMissing(t *testing.T) {
	attrs := libscim.ResourceAttributes{}
	eid := extractExternalID(attrs)
	assert.False(t, eid.Present())
}

func TestExtractPatchMembers(t *testing.T) {
	// Array of members
	value := []interface{}{
		map[string]interface{}{"value": "u1"},
		map[string]interface{}{"value": "u2"},
	}
	members := extractPatchMembers(value)
	assert.Equal(t, []string{"u1", "u2"}, members)

	// Single member map
	single := map[string]interface{}{"value": "u3"}
	members = extractPatchMembers(single)
	assert.Equal(t, []string{"u3"}, members)
}

func TestResourceHandlerInterface(t *testing.T) {
	// Verify UserHandler and GroupHandler implement scim.ResourceHandler
	var _ libscim.ResourceHandler = &UserHandler{}
	var _ libscim.ResourceHandler = &GroupHandler{}
}

func TestNewSCIMServer(t *testing.T) {
	// Verify SCIM server can be created with nil DB (for interface check)
	server, err := NewSCIMServer(Config{})
	assert.NoError(t, err)
	assert.NotNil(t, server)
}

func TestDerefStr(t *testing.T) {
	s := "hello"
	assert.Equal(t, "hello", derefStr(&s))
	assert.Equal(t, "", derefStr(nil))
}

func TestIsUserActiveDefault(t *testing.T) {
	u := db.UserTable{UserID: "test"}
	assert.True(t, isUserActive(u))
}

func TestNewSCIMServerExternalID(t *testing.T) {
	eid := extractExternalID(libscim.ResourceAttributes{"externalId": "abc"})
	assert.True(t, eid.Present())
	assert.Equal(t, "abc", eid.Value())

	noEid := extractExternalID(libscim.ResourceAttributes{})
	assert.Equal(t, optional.String{}, noEid)
}
