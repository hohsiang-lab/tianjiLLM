package contract

import (
	"context"
	"errors"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/hook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockHook struct {
	name        string
	preCallErr  error
	postCallErr error
	preCalled   bool
	postCalled  bool
}

func (m *mockHook) Name() string { return m.name }
func (m *mockHook) PreCall(_ context.Context, _ *model.ChatCompletionRequest) error {
	m.preCalled = true
	return m.preCallErr
}
func (m *mockHook) PostCall(_ context.Context, _ *model.ChatCompletionRequest, _ *model.ModelResponse) error {
	m.postCalled = true
	return m.postCallErr
}

func TestHookRegistry_PreCallChain(t *testing.T) {
	r := hook.NewRegistry()
	h1 := &mockHook{name: "hook1"}
	h2 := &mockHook{name: "hook2"}
	r.Register(h1)
	r.Register(h2)

	err := r.RunPreCall(context.Background(), &model.ChatCompletionRequest{Model: "gpt-4"})
	require.NoError(t, err)
	assert.True(t, h1.preCalled)
	assert.True(t, h2.preCalled)
}

func TestHookRegistry_PreCallRejection(t *testing.T) {
	r := hook.NewRegistry()
	rejectErr := errors.New("blocked by policy")
	h1 := &mockHook{name: "blocker", preCallErr: rejectErr}
	h2 := &mockHook{name: "should-not-run"}
	r.Register(h1)
	r.Register(h2)

	err := r.RunPreCall(context.Background(), &model.ChatCompletionRequest{Model: "gpt-4"})
	assert.ErrorIs(t, err, rejectErr)
	assert.True(t, h1.preCalled)
	assert.False(t, h2.preCalled, "second hook should not run after first rejects")
}

func TestHookRegistry_PostCallChain(t *testing.T) {
	r := hook.NewRegistry()
	h1 := &mockHook{name: "logger"}
	h2 := &mockHook{name: "auditor"}
	r.Register(h1)
	r.Register(h2)

	err := r.RunPostCall(context.Background(), &model.ChatCompletionRequest{}, &model.ModelResponse{})
	require.NoError(t, err)
	assert.True(t, h1.postCalled)
	assert.True(t, h2.postCalled)
}

func TestHookRegistry_Empty(t *testing.T) {
	r := hook.NewRegistry()
	assert.NoError(t, r.RunPreCall(context.Background(), &model.ChatCompletionRequest{}))
	assert.NoError(t, r.RunPostCall(context.Background(), &model.ChatCompletionRequest{}, &model.ModelResponse{}))
	assert.Equal(t, 0, r.Len())
}
