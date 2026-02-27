package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestAccessControl_IsPublic(t *testing.T) {
	tests := []struct {
		name string
		ac   *AccessControl
		want bool
	}{
		{"nil", nil, true},
		{"empty struct", &AccessControl{}, true},
		{"empty slices", &AccessControl{AllowedOrgs: []string{}, AllowedTeams: []string{}, AllowedKeys: []string{}}, true},
		{"has orgs", &AccessControl{AllowedOrgs: []string{"org_a"}}, false},
		{"has teams", &AccessControl{AllowedTeams: []string{"team_a"}}, false},
		{"has keys", &AccessControl{AllowedKeys: []string{"key_a"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.ac.IsPublic())
		})
	}
}

func TestAccessControl_IsAllowed(t *testing.T) {
	ac := &AccessControl{
		AllowedOrgs:  []string{"org_acme", "org_bigcorp"},
		AllowedTeams: []string{"team_ml"},
		AllowedKeys:  []string{"sk-hash-abc"},
	}

	tests := []struct {
		name      string
		orgID     string
		teamID    string
		tokenHash string
		want      bool
	}{
		{"matching org", "org_acme", "", "", true},
		{"matching team", "", "team_ml", "", true},
		{"matching key", "", "", "sk-hash-abc", true},
		{"no match", "org_x", "team_x", "sk-x", false},
		{"empty caller", "", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ac.IsAllowed(tt.orgID, tt.teamID, tt.tokenHash))
		})
	}

	// nil AC always allows
	assert.True(t, (*AccessControl)(nil).IsAllowed("any", "any", "any"))
}

func TestAccessControl_YAMLParsing(t *testing.T) {
	yamlContent := `
model_list:
  - model_name: gpt-4o
    tianji_params:
      model: openai/gpt-4o
      api_key: sk-test
    access_control:
      allowed_orgs:
        - org_acme
      allowed_teams:
        - team_ml
  - model_name: claude
    tianji_params:
      model: anthropic/claude-3
      api_key: sk-test2
`
	tmpDir := t.TempDir()
	path := tmpDir + "/config.yaml"
	if err := writeTestFile(path, yamlContent); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	// First model has access control
	assert.NotNil(t, cfg.ModelList[0].AccessControl)
	assert.Equal(t, []string{"org_acme"}, cfg.ModelList[0].AccessControl.AllowedOrgs)
	assert.Equal(t, []string{"team_ml"}, cfg.ModelList[0].AccessControl.AllowedTeams)
	assert.False(t, cfg.ModelList[0].AccessControl.IsPublic())

	// Second model has no access control
	assert.Nil(t, cfg.ModelList[1].AccessControl)
	assert.True(t, cfg.ModelList[1].AccessControl.IsPublic())
}

func TestAccessControl_RoundTrip(t *testing.T) {
	yamlContent := `
model_list:
  - model_name: gpt-4o
    tianji_params:
      model: openai/gpt-4o
      api_key: sk-test
    access_control:
      allowed_orgs:
        - org_acme
      allowed_teams:
        - team_ml
      allowed_keys:
        - sk-hash-abc
`
	tmpDir := t.TempDir()
	path := tmpDir + "/config.yaml"
	require.NoError(t, writeTestFile(path, yamlContent))

	cfg, err := Load(path)
	require.NoError(t, err)

	ac := cfg.ModelList[0].AccessControl
	require.NotNil(t, ac)
	assert.Equal(t, []string{"org_acme"}, ac.AllowedOrgs)
	assert.Equal(t, []string{"team_ml"}, ac.AllowedTeams)
	assert.Equal(t, []string{"sk-hash-abc"}, ac.AllowedKeys)

	// Marshal back to YAML and reload
	out, err := yaml.Marshal(cfg)
	require.NoError(t, err)

	path2 := tmpDir + "/config2.yaml"
	require.NoError(t, writeTestFile(path2, string(out)))

	cfg2, err := Load(path2)
	require.NoError(t, err)

	ac2 := cfg2.ModelList[0].AccessControl
	require.NotNil(t, ac2)
	assert.Equal(t, ac.AllowedOrgs, ac2.AllowedOrgs)
	assert.Equal(t, ac.AllowedTeams, ac2.AllowedTeams)
	assert.Equal(t, ac.AllowedKeys, ac2.AllowedKeys)
}

func TestAccessControl_MixedFields(t *testing.T) {
	ac := &AccessControl{
		AllowedOrgs:  []string{"org_acme"},
		AllowedTeams: []string{"team_ml"},
		AllowedKeys:  []string{"sk-hash-abc"},
	}

	assert.False(t, ac.IsPublic())

	// Any single matching field grants access
	assert.True(t, ac.IsAllowed("org_acme", "", ""))
	assert.True(t, ac.IsAllowed("", "team_ml", ""))
	assert.True(t, ac.IsAllowed("", "", "sk-hash-abc"))

	// Non-matching across all fields denies
	assert.False(t, ac.IsAllowed("org_x", "team_x", "sk-x"))

	// Matching one field is enough even if others don't match
	assert.True(t, ac.IsAllowed("org_acme", "team_x", "sk-x"))
	assert.True(t, ac.IsAllowed("org_x", "team_ml", "sk-x"))
	assert.True(t, ac.IsAllowed("org_x", "team_x", "sk-hash-abc"))
}

func writeTestFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}
