//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AC-5 — API: /model/new and /model/update persist access_control in model_info.

func TestAPI_ModelNew_WithAccessControl(t *testing.T) {
	setup(t)

	modelInfo := map[string]any{
		"access_control": map[string]any{
			"allowed_orgs":  []string{"org_acme", "org_bigcorp"},
			"allowed_teams": []string{"team_ml"},
		},
	}
	modelInfoJSON, _ := json.Marshal(modelInfo)
	tianjiParams := map[string]any{"model": "openai/gpt-4o", "api_key": "sk-test"}
	tianjiJSON, _ := json.Marshal(tianjiParams)

	body := map[string]any{
		"model_id":      "api-ac-test-1",
		"model_name":    "api-restricted-model",
		"tianji_params": json.RawMessage(tianjiJSON),
		"model_info":    json.RawMessage(modelInfoJSON),
		"created_by":    "e2e",
	}
	bodyJSON, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", testServer.URL+"/model/new", bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+masterKey)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	// Verify via /model/info
	infoReq, _ := http.NewRequest("GET", testServer.URL+"/model/info?model_id=api-ac-test-1", nil)
	infoReq.Header.Set("Authorization", "Bearer "+masterKey)
	infoResp, err := http.DefaultClient.Do(infoReq)
	require.NoError(t, err)
	defer infoResp.Body.Close()

	assert.Equal(t, http.StatusOK, infoResp.StatusCode)
	respBody, _ := io.ReadAll(infoResp.Body)

	var result map[string]any
	require.NoError(t, json.Unmarshal(respBody, &result))

	// model_info should contain access_control
	infoStr, ok := result["model_info"]
	require.True(t, ok, "response should contain model_info")

	var info map[string]any
	switch v := infoStr.(type) {
	case string:
		require.NoError(t, json.Unmarshal([]byte(v), &info))
	case map[string]any:
		info = v
	default:
		// It might be returned as raw JSON bytes
		raw, _ := json.Marshal(v)
		require.NoError(t, json.Unmarshal(raw, &info))
	}

	ac, ok := info["access_control"].(map[string]any)
	require.True(t, ok, "model_info should have access_control")
	assert.Equal(t, []string{"org_acme", "org_bigcorp"}, toStringSliceFromAny(ac["allowed_orgs"]))
	assert.Equal(t, []string{"team_ml"}, toStringSliceFromAny(ac["allowed_teams"]))
}

func TestAPI_ModelNew_WithoutAccessControl(t *testing.T) {
	setup(t)

	tianjiParams := map[string]any{"model": "openai/gpt-4o"}
	tianjiJSON, _ := json.Marshal(tianjiParams)

	body := map[string]any{
		"model_id":      "api-public-test-1",
		"model_name":    "api-public-model",
		"tianji_params": json.RawMessage(tianjiJSON),
		"model_info":    json.RawMessage([]byte("{}")),
		"created_by":    "e2e",
	}
	bodyJSON, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", testServer.URL+"/model/new", bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+masterKey)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	// Verify model_info has no access_control
	infoReq, _ := http.NewRequest("GET", testServer.URL+"/model/info?model_id=api-public-test-1", nil)
	infoReq.Header.Set("Authorization", "Bearer "+masterKey)
	infoResp, err := http.DefaultClient.Do(infoReq)
	require.NoError(t, err)
	defer infoResp.Body.Close()

	respBody, _ := io.ReadAll(infoResp.Body)
	var result map[string]any
	json.Unmarshal(respBody, &result)

	var info map[string]any
	switch v := result["model_info"].(type) {
	case string:
		json.Unmarshal([]byte(v), &info)
	case map[string]any:
		info = v
	}
	_, hasAC := info["access_control"]
	assert.False(t, hasAC, "public model should not have access_control in model_info")
}

func TestAPI_ModelUpdate_AccessControl(t *testing.T) {
	setup(t)

	// Create a model first
	tianjiJSON, _ := json.Marshal(map[string]any{"model": "openai/gpt-4o"})
	createBody, _ := json.Marshal(map[string]any{
		"model_id":      "api-update-ac-1",
		"model_name":    "update-ac-model",
		"tianji_params": json.RawMessage(tianjiJSON),
		"model_info":    json.RawMessage([]byte("{}")),
		"created_by":    "e2e",
	})

	createReq, _ := http.NewRequest("POST", testServer.URL+"/model/new", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+masterKey)
	createResp, _ := http.DefaultClient.Do(createReq)
	createResp.Body.Close()

	// Update with access_control
	newInfo := map[string]any{
		"access_control": map[string]any{
			"allowed_keys": []string{"sk-hash-special"},
		},
	}
	newInfoJSON, _ := json.Marshal(newInfo)
	updateBody, _ := json.Marshal(map[string]any{
		"model_id":      "api-update-ac-1",
		"model_name":    "update-ac-model",
		"tianji_params": json.RawMessage(tianjiJSON),
		"model_info":    json.RawMessage(newInfoJSON),
		"updated_by":    "e2e",
	})

	updateReq, _ := http.NewRequest("POST", testServer.URL+"/model/update", bytes.NewReader(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.Header.Set("Authorization", "Bearer "+masterKey)
	updateResp, err := http.DefaultClient.Do(updateReq)
	require.NoError(t, err)
	defer updateResp.Body.Close()

	assert.Equal(t, http.StatusOK, updateResp.StatusCode)

	// Verify via GET
	infoReq, _ := http.NewRequest("GET", testServer.URL+"/model/info?model_id=api-update-ac-1", nil)
	infoReq.Header.Set("Authorization", "Bearer "+masterKey)
	infoResp, err := http.DefaultClient.Do(infoReq)
	require.NoError(t, err)
	defer infoResp.Body.Close()

	respBody, _ := io.ReadAll(infoResp.Body)
	var result map[string]any
	json.Unmarshal(respBody, &result)

	var info map[string]any
	switch v := result["model_info"].(type) {
	case string:
		json.Unmarshal([]byte(v), &info)
	case map[string]any:
		info = v
	}
	ac := info["access_control"].(map[string]any)
	assert.Equal(t, []string{"sk-hash-special"}, toStringSliceFromAny(ac["allowed_keys"]))
}

func TestAPI_Config_ReturnsModelListAndSettings(t *testing.T) {
	// GET /config returns the in-memory config (not DB models).
	// Verifies the endpoint is accessible and returns the expected top-level keys.
	_ = setup(t) // clean slate

	req, _ := http.NewRequest("GET", testServer.URL+"/config/", nil)
	req.Header.Set("Authorization", "Bearer "+masterKey)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]any
	require.NoError(t, json.Unmarshal(respBody, &result))

	// model_list should exist (may be empty in e2e)
	_, ok := result["model_list"]
	assert.True(t, ok, "/config should return model_list")

	// general_settings should exist
	_, ok = result["general_settings"]
	assert.True(t, ok, "/config should return general_settings")
}
