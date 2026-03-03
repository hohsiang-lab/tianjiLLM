package handler

import (
	"context"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolvePromptTemplate_LatestVersion(t *testing.T) {
	ms := newMockStore()
	ms.getLatestPromptByNameFn = func(_ context.Context, name string) (db.PromptTemplateTable, error) {
		return db.PromptTemplateTable{Template: "Hello {{name}}, welcome to {{place}}"}, nil
	}
	req := &model.ChatCompletionRequest{
		PromptName:      "greeting",
		PromptVariables: map[string]string{"name": "Alice", "place": "Wonderland"},
	}
	err := resolvePromptTemplate(context.Background(), ms, req)
	require.NoError(t, err)
	assert.Len(t, req.Messages, 1)
	assert.Equal(t, "Hello Alice, welcome to Wonderland", req.Messages[0].Content)
	assert.Empty(t, req.PromptName)
}

func TestResolvePromptTemplate_SpecificVersion(t *testing.T) {
	ms := newMockStore()
	ms.getPromptByNameVersionFn = func(_ context.Context, arg db.GetPromptTemplateByNameVersionParams) (db.PromptTemplateTable, error) {
		return db.PromptTemplateTable{Template: "v2: {{msg}}"}, nil
	}
	v := 2
	req := &model.ChatCompletionRequest{
		PromptName:      "test",
		PromptVersion:   &v,
		PromptVariables: map[string]string{"msg": "hi"},
	}
	err := resolvePromptTemplate(context.Background(), ms, req)
	require.NoError(t, err)
	assert.Equal(t, "v2: hi", req.Messages[0].Content)
}
