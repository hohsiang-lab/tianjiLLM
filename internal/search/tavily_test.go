package search

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTavily_TransformRequest(t *testing.T) {
	tv := &Tavily{}

	t.Run("basic request", func(t *testing.T) {
		req := tv.TransformRequest(SearchParams{Query: "golang", MaxResults: 3})
		m, _ := req.(map[string]any)
		assert.Equal(t, "golang", m["query"])
		assert.Equal(t, 3, m["max_results"])
	})

	t.Run("domain filter", func(t *testing.T) {
		req := tv.TransformRequest(SearchParams{
			Query:              "test",
			SearchDomainFilter: []string{"github.com"},
		})
		m, _ := req.(map[string]any)
		assert.Equal(t, []string{"github.com"}, m["include_domains"])
	})

	t.Run("default max results", func(t *testing.T) {
		req := tv.TransformRequest(SearchParams{Query: "test"})
		m, _ := req.(map[string]any)
		assert.Equal(t, 5, m["max_results"])
	})
}

func TestTavily_TransformResponse(t *testing.T) {
	tv := &Tavily{}

	body := []byte(`{
		"results": [
			{"title": "Result 1", "url": "https://example.com/1", "content": "First result"},
			{"title": "Result 2", "url": "https://example.com/2", "content": "Second result"}
		]
	}`)

	resp, err := tv.TransformResponse(body)
	require.NoError(t, err)
	assert.Equal(t, "search.results", resp.Object)
	assert.Len(t, resp.Results, 2)
	assert.Equal(t, "First result", resp.Results[0].Snippet)
}

func TestTavily_ValidateEnvironment(t *testing.T) {
	tv := &Tavily{}

	_, err := tv.ValidateEnvironment("", "")
	assert.Error(t, err)

	headers, err := tv.ValidateEnvironment("key", "")
	require.NoError(t, err)
	assert.Equal(t, "application/json", headers.Get("Content-Type"))
}

func TestTavily_RequestJSON(t *testing.T) {
	tv := &Tavily{}
	req := tv.TransformRequest(SearchParams{Query: "test", MaxResults: 5})
	b, err := json.Marshal(req)
	require.NoError(t, err)
	assert.Contains(t, string(b), `"query":"test"`)
}
