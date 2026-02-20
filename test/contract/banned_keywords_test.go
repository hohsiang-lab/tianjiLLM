package contract

import (
	"context"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/hook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createBannedKeywordsHook(t *testing.T, keywords []string) hook.Hook {
	t.Helper()
	kw := make([]any, len(keywords))
	for i, k := range keywords {
		kw[i] = k
	}
	h, err := hook.Create("banned_keywords", map[string]any{"keywords": kw})
	require.NoError(t, err)
	return h
}

func createBlockedUserHook(t *testing.T, users []string) hook.Hook {
	t.Helper()
	list := make([]any, len(users))
	for i, u := range users {
		list[i] = u
	}
	h, err := hook.Create("blocked_user_list", map[string]any{"users": list})
	require.NoError(t, err)
	return h
}

func TestBannedKeywordsHook_BlocksMessage(t *testing.T) {
	h := createBannedKeywordsHook(t, []string{"secret", "password"})

	req := &model.ChatCompletionRequest{
		Model: "gpt-4",
		Messages: []model.Message{
			{Role: "user", Content: "Tell me the secret code"},
		},
	}

	err := h.PreCall(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "banned keyword")
}

func TestBannedKeywordsHook_AllowsCleanMessage(t *testing.T) {
	h := createBannedKeywordsHook(t, []string{"secret", "password"})

	req := &model.ChatCompletionRequest{
		Model: "gpt-4",
		Messages: []model.Message{
			{Role: "user", Content: "Hello, how are you?"},
		},
	}

	err := h.PreCall(context.Background(), req)
	assert.NoError(t, err)
}

func TestBannedKeywordsHook_CaseInsensitive(t *testing.T) {
	h := createBannedKeywordsHook(t, []string{"SECRET"})

	req := &model.ChatCompletionRequest{
		Model: "gpt-4",
		Messages: []model.Message{
			{Role: "user", Content: "this is a secret message"},
		},
	}

	err := h.PreCall(context.Background(), req)
	require.Error(t, err)
}

func TestBlockedUserHook_BlocksUser(t *testing.T) {
	h := createBlockedUserHook(t, []string{"bad-user", "evil-bot"})

	s := "bad-user"
	req := &model.ChatCompletionRequest{
		Model: "gpt-4",
		User:  &s,
	}

	err := h.PreCall(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "blocked")
}

func TestBlockedUserHook_AllowsGoodUser(t *testing.T) {
	h := createBlockedUserHook(t, []string{"bad-user"})

	s := "good-user"
	req := &model.ChatCompletionRequest{
		Model: "gpt-4",
		User:  &s,
	}

	err := h.PreCall(context.Background(), req)
	assert.NoError(t, err)
}

func TestBlockedUserHook_NoUserField(t *testing.T) {
	h := createBlockedUserHook(t, []string{"bad-user"})

	req := &model.ChatCompletionRequest{
		Model: "gpt-4",
	}

	err := h.PreCall(context.Background(), req)
	assert.NoError(t, err)
}
