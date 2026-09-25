package contract

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/testutil/openaitest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func requireStandardError(t *testing.T, body []byte, param, code string) model.ErrorDetail {
	t.Helper()
	var response model.ErrorResponse
	require.NoError(t, json.Unmarshal(body, &response))
	assert.Equal(t, "invalid_request_error", response.Error.Type)
	assert.Equal(t, param, response.Error.Param)
	assert.Equal(t, code, response.Error.Code)
	return response.Error
}

func decodeSSEData(t *testing.T, body io.Reader) []string {
	t.Helper()
	var events []string
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			events = append(events, strings.TrimPrefix(line, "data: "))
		}
	}
	require.NoError(t, scanner.Err())
	return events
}

func TestSharedContractHelpers(t *testing.T) {
	direct, _ := newDirectOpenAIContractServer(t, openaitest.UpstreamServerOptions{})
	directCapability, ok := direct.Handlers.Capabilities.Lookup(model.BackendDirectOpenAIHTTP, contractModel)
	require.True(t, ok)
	assert.True(t, directCapability.SupportsNonStream)

	streamOnly, _ := newStreamOnlyContractServer(t, openaitest.UpstreamServerOptions{})
	streamCapability, ok := streamOnly.Handlers.Capabilities.Lookup(model.BackendDirectOpenAIHTTP, contractModel)
	require.True(t, ok)
	assert.False(t, streamCapability.SupportsNonStream)

	assert.Equal(t, []string{"chunk", "[DONE]"}, decodeSSEData(t, strings.NewReader("data: chunk\n\ndata: [DONE]\n\n")))
	assert.Equal(t, "bad", requireStandardError(
		t,
		[]byte(`{"error":{"message":"bad","type":"invalid_request_error","param":"temperature","code":"invalid_value"}}`),
		"temperature",
		"invalid_value",
	).Message)
}
