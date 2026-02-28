package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAssistantsProxy_NotConfigured(t *testing.T) {
	h := newTestHandlers()

	endpoints := []struct {
		name string
		fn   func(http.ResponseWriter, *http.Request)
	}{
		{"AssistantCreate", h.AssistantCreate},
		{"AssistantGet", h.AssistantGet},
		{"AssistantList", h.AssistantList},
		{"AssistantModify", h.AssistantModify},
		{"AssistantDelete", h.AssistantDelete},
		{"ThreadCreate", h.ThreadCreate},
		{"ThreadGet", h.ThreadGet},
		{"ThreadModify", h.ThreadModify},
		{"ThreadDelete", h.ThreadDelete},
		{"MessageCreate", h.MessageCreate},
		{"MessageList", h.MessageList},
		{"MessageGet", h.MessageGet},
		{"RunCreate", h.RunCreate},
		{"RunGet", h.RunGet},
		{"RunList", h.RunList},
		{"RunCancel", h.RunCancel},
		{"RunStepsList", h.RunStepsList},
		{"RunStepGet", h.RunStepGet},
	}

	for _, ep := range endpoints {
		t.Run(ep.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("POST", "/v1/assistants", nil)
			ep.fn(w, r)
			assert.Equal(t, http.StatusNotImplemented, w.Code)
		})
	}
}
