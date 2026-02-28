package config

import "testing"

func TestAccessControlIsPublic(t *testing.T) {
	var ac *AccessControl
	if !ac.IsPublic() {
		t.Fatal("nil should be public")
	}

	ac2 := &AccessControl{}
	if !ac2.IsPublic() {
		t.Fatal("empty should be public")
	}

	ac3 := &AccessControl{AllowedOrgs: []string{"org1"}}
	if ac3.IsPublic() {
		t.Fatal("should not be public with orgs")
	}
}

func TestAccessControlIsAllowed(t *testing.T) {
	ac := &AccessControl{
		AllowedOrgs:  []string{"org1"},
		AllowedTeams: []string{"team1"},
		AllowedKeys:  []string{"key1"},
	}

	if !ac.IsAllowed("org1", "", "") {
		t.Fatal("org1 should be allowed")
	}
	if !ac.IsAllowed("", "team1", "") {
		t.Fatal("team1 should be allowed")
	}
	if !ac.IsAllowed("", "", "key1") {
		t.Fatal("key1 should be allowed")
	}
	if ac.IsAllowed("org2", "team2", "key2") {
		t.Fatal("should not be allowed")
	}

	// Public allows all
	var pub *AccessControl
	if !pub.IsAllowed("any", "any", "any") {
		t.Fatal("nil (public) should allow all")
	}
}
