package ui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHandleKeyDelete_DBNil(t *testing.T) {
	h := &UIHandler{}
	req := httptest.NewRequest(http.MethodPost, "/ui/keys/delete",
		strings.NewReader("token=test-token"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.handleKeyDelete(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandleKeyDelete_MissingToken(t *testing.T) {
	h := &UIHandler{}
	req := httptest.NewRequest(http.MethodPost, "/ui/keys/delete",
		strings.NewReader("token="))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.handleKeyDelete(w, req)
	// DB nil check fires before token empty check
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}
