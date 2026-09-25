package handler

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteRequestErrorUsesStandardShape(t *testing.T) {
	w := httptest.NewRecorder()
	writeRequestError(w, model.UnsupportedParameter("response_format", "unsupported"))

	assert.Equal(t, 400, w.Code)
	var body model.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "invalid_request_error", body.Error.Type)
	assert.Equal(t, "response_format", body.Error.Param)
	assert.Equal(t, "unsupported_parameter", body.Error.Code)
}
