package scim

import (
	"testing"

	libscim "github.com/elimity-com/scim"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/stretchr/testify/assert"
)

func TestToSCIMUser_WithEmail(t *testing.T) {
	email := "alice@example.com"
	alias := "alice"
	u := db.UserTable{
		UserID:    "u1",
		UserAlias: &alias,
		UserEmail: &email,
	}
	r := toSCIMUser(u)
	assert.Equal(t, "u1", r.Attributes["userName"])
	assert.Equal(t, "alice", r.Attributes["displayName"])
}

func TestToSCIMUser_NoEmail(t *testing.T) {
	u := db.UserTable{UserID: "u2"}
	r := toSCIMUser(u)
	assert.Equal(t, "u2", r.Attributes["userName"])
	_, hasEmail := r.Attributes["emails"]
	assert.False(t, hasEmail)
}

func TestToSCIMGroup_WithMembers(t *testing.T) {
	team := db.TeamTable{
		TeamID:  "t1",
		Members: []string{"u1", "u2"},
	}
	r := toSCIMGroup(team)
	assert.Equal(t, "t1", r.ID)
	mems, _ := r.Attributes["members"].([]interface{})
	assert.Len(t, mems, 2)
}

func TestFromSCIMGroup_WithMembers(t *testing.T) {
	attrs := libscim.ResourceAttributes{
		"displayName": "Engineering",
		"members": []interface{}{
			map[string]interface{}{"value": "u1"},
			map[string]interface{}{"value": "u2"},
		},
	}
	p := fromSCIMGroup(attrs)
	assert.Equal(t, []string{"u1", "u2"}, p.Members)
}

func TestFromSCIMGroup_NoMembers(t *testing.T) {
	attrs := libscim.ResourceAttributes{"displayName": "Empty"}
	p := fromSCIMGroup(attrs)
	assert.Empty(t, p.Members)
}

func TestExtractMembers_Valid(t *testing.T) {
	attrs := libscim.ResourceAttributes{
		"members": []interface{}{
			map[string]interface{}{"value": "u1"},
			map[string]interface{}{"value": "u2"},
		},
	}
	ids := extractMembers(attrs)
	assert.Equal(t, []string{"u1", "u2"}, ids)
}

func TestExtractMembers_Missing(t *testing.T) {
	attrs := libscim.ResourceAttributes{}
	ids := extractMembers(attrs)
	assert.Nil(t, ids)
}

func TestDerefStr_Nil(t *testing.T) {
	assert.Equal(t, "", derefStr(nil))
}

func TestDerefStr_Value(t *testing.T) {
	s := "hello"
	assert.Equal(t, "hello", derefStr(&s))
}

func TestExtractExternalID_Present(t *testing.T) {
	attrs := libscim.ResourceAttributes{"externalId": "ext-1"}
	result := extractExternalID(attrs)
	assert.NotNil(t, result)
	assert.Equal(t, "ext-1", result.Value())
}

func TestExtractExternalID_Absent(t *testing.T) {
	attrs := libscim.ResourceAttributes{}
	result := extractExternalID(attrs)
	assert.False(t, result.Present())
}
