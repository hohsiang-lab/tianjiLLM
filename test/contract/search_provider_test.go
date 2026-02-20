package contract

import (
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/search"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFirecrawlProvider_Metadata(t *testing.T) {
	p, err := search.Get("firecrawl")
	require.NoError(t, err)

	assert.Equal(t, "firecrawl", p.Name())
	assert.Equal(t, "POST", p.HTTPMethod())
	assert.Contains(t, p.DefaultAPIBase(), "firecrawl.dev")
}

func TestFirecrawlProvider_ValidateEnvironment(t *testing.T) {
	p, _ := search.Get("firecrawl")

	_, err := p.ValidateEnvironment("", "")
	assert.Error(t, err, "should require API key")

	headers, err := p.ValidateEnvironment("test-key", "")
	require.NoError(t, err)
	assert.Contains(t, headers.Get("Authorization"), "Bearer test-key")
}

func TestFirecrawlProvider_TransformRequest(t *testing.T) {
	p, _ := search.Get("firecrawl")

	body := p.TransformRequest(search.SearchParams{
		Query:      "golang tutorial",
		MaxResults: 3,
	})

	m, ok := body.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "golang tutorial", m["query"])
	assert.Equal(t, 3, m["limit"])
}

func TestFirecrawlProvider_TransformResponse(t *testing.T) {
	p, _ := search.Get("firecrawl")

	rawJSON := `{
		"data": [
			{"url": "https://go.dev", "title": "Go", "description": "Programming language"},
			{"url": "https://go.dev/tour", "title": "Tour", "description": "A tour of Go"}
		]
	}`

	resp, err := p.TransformResponse([]byte(rawJSON))
	require.NoError(t, err)
	assert.Len(t, resp.Results, 2)
	assert.Equal(t, "Go", resp.Results[0].Title)
	assert.Equal(t, "https://go.dev", resp.Results[0].URL)
}

func TestLinkupProvider_Metadata(t *testing.T) {
	p, err := search.Get("linkup")
	require.NoError(t, err)

	assert.Equal(t, "linkup", p.Name())
	assert.Equal(t, "POST", p.HTTPMethod())
	assert.Contains(t, p.DefaultAPIBase(), "linkup.so")
}

func TestLinkupProvider_ValidateEnvironment(t *testing.T) {
	p, _ := search.Get("linkup")

	_, err := p.ValidateEnvironment("", "")
	assert.Error(t, err, "should require API key")

	headers, err := p.ValidateEnvironment("test-key", "")
	require.NoError(t, err)
	assert.Contains(t, headers.Get("Authorization"), "Bearer test-key")
}

func TestLinkupProvider_TransformRequest(t *testing.T) {
	p, _ := search.Get("linkup")

	body := p.TransformRequest(search.SearchParams{
		Query:      "golang concurrency",
		MaxResults: 5,
	})

	m, ok := body.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "golang concurrency", m["q"])
	assert.Equal(t, "standard", m["depth"])
}

func TestLinkupProvider_TransformResponse(t *testing.T) {
	p, _ := search.Get("linkup")

	rawJSON := `{
		"results": [
			{"name": "Goroutines", "url": "https://go.dev/goroutines", "content": "Concurrent programming"}
		]
	}`

	resp, err := p.TransformResponse([]byte(rawJSON))
	require.NoError(t, err)
	assert.Len(t, resp.Results, 1)
	assert.Equal(t, "Goroutines", resp.Results[0].Title)
	assert.Equal(t, "Concurrent programming", resp.Results[0].Snippet)
}
