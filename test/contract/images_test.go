package contract

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImageVariation_PassThrough(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/images/variations", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.Header.Get("Authorization"), "Bearer sk-test-key")
		assert.Contains(t, r.Header.Get("Content-Type"), "multipart/form-data")

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"created": 1700000000,
			"data": []map[string]any{
				{"url": "https://example.com/variation.png"},
			},
		})
	}))
	defer upstream.Close()

	srv := newTestServerWithBaseURL(t, upstream.URL)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("image", "test.png")
	require.NoError(t, err)
	_, _ = part.Write([]byte("fake png data"))
	_ = writer.WriteField("model", "dall-e-2")
	_ = writer.WriteField("n", "1")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/images/variations", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data, ok := resp["data"].([]any)
	require.True(t, ok)
	assert.Len(t, data, 1)
}
