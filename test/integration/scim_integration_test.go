package integration

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/scim"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSCIMServer_Creation(t *testing.T) {
	server, err := scim.NewSCIMServer(scim.Config{})
	require.NoError(t, err)
	assert.NotNil(t, server)
}

func TestSCIMServer_ServiceProviderConfig(t *testing.T) {
	server, err := scim.NewSCIMServer(scim.Config{})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/ServiceProviderConfig", nil)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/scim+json")
	assert.Contains(t, w.Body.String(), "urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig")
}

func TestSCIMServer_Schemas(t *testing.T) {
	server, err := scim.NewSCIMServer(scim.Config{})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/Schemas", nil)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "urn:ietf:params:scim:schemas:core:2.0:User")
	assert.Contains(t, w.Body.String(), "urn:ietf:params:scim:schemas:core:2.0:Group")
}

func TestSCIMServer_ResourceTypes(t *testing.T) {
	server, err := scim.NewSCIMServer(scim.Config{})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/ResourceTypes", nil)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "User")
	assert.Contains(t, w.Body.String(), "Group")
}

func TestSCIMServer_V2PrefixStripping(t *testing.T) {
	server, err := scim.NewSCIMServer(scim.Config{})
	require.NoError(t, err)

	// SCIM server supports /v2 prefix auto-stripping
	req := httptest.NewRequest(http.MethodGet, "/v2/ServiceProviderConfig", nil)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSCIMServer_MountedOnProxy(t *testing.T) {
	srv := newIntegrationServer(t)

	// Without SCIM handler mounted, /scim/v2 should return 404/405
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/ServiceProviderConfig", nil)
	req.Header.Set("Authorization", "Bearer sk-master")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	// Since SCIMHandler is nil in default test server, should get 404
	assert.Equal(t, http.StatusNotFound, w.Code)
}
