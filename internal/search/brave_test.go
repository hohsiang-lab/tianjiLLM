package search

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBrave_URLConstruction(t *testing.T) {
	b := &Brave{}

	t.Run("default base URL", func(t *testing.T) {
		url := b.GetCompleteURL("", SearchParams{Query: "golang"})
		assert.Contains(t, url, "api.search.brave.com")
		assert.Contains(t, url, "q=golang")
	})

	t.Run("domain filter appended to query", func(t *testing.T) {
		url := b.GetCompleteURL("", SearchParams{
			Query:              "test",
			SearchDomainFilter: []string{"example.com", "github.com"},
		})
		assert.Contains(t, url, "site%3Aexample.com")
		assert.Contains(t, url, "site%3Agithub.com")
	})

	t.Run("max results capped at 20", func(t *testing.T) {
		url := b.GetCompleteURL("", SearchParams{Query: "test", MaxResults: 50})
		assert.Contains(t, url, "count=20")
	})

	t.Run("custom base URL", func(t *testing.T) {
		url := b.GetCompleteURL("https://custom.brave.com/search", SearchParams{Query: "test"})
		assert.Contains(t, url, "custom.brave.com")
	})
}

func TestBrave_ValidateEnvironment(t *testing.T) {
	b := &Brave{}

	t.Run("missing API key", func(t *testing.T) {
		_, err := b.ValidateEnvironment("", "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "BRAVE_API_KEY")
	})

	t.Run("valid API key", func(t *testing.T) {
		headers, err := b.ValidateEnvironment("test-key", "")
		require.NoError(t, err)
		assert.Equal(t, "test-key", headers.Get("X-Subscription-Token"))
	})
}

func TestBrave_TransformResponse(t *testing.T) {
	b := &Brave{}

	body := []byte(`{
		"web": {
			"results": [
				{"title": "Go Docs", "url": "https://go.dev", "description": "Go programming language"},
				{"title": "Go Tour", "url": "https://go.dev/tour", "description": "Interactive Go tutorial"}
			]
		},
		"news": {
			"results": [
				{"title": "Go 1.22", "url": "https://blog.go.dev", "description": "Release notes", "age": "2 days"}
			]
		}
	}`)

	resp, err := b.TransformResponse(body)
	require.NoError(t, err)
	assert.Equal(t, "search.results", resp.Object)
	assert.Len(t, resp.Results, 3)
	assert.Equal(t, "Go Docs", resp.Results[0].Title)
	assert.Equal(t, "2 days", resp.Results[2].Date)
}
