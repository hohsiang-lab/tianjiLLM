package search

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func init() {
	Register("firecrawl", &FirecrawlProvider{})
}

// FirecrawlProvider implements search via Firecrawl API.
type FirecrawlProvider struct{}

func (p *FirecrawlProvider) Name() string           { return "firecrawl" }
func (p *FirecrawlProvider) HTTPMethod() string     { return "POST" }
func (p *FirecrawlProvider) DefaultAPIBase() string { return "https://api.firecrawl.dev/v1/search" }

func (p *FirecrawlProvider) ValidateEnvironment(apiKey, _ string) (http.Header, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("FIRECRAWL_API_KEY required")
	}
	h := http.Header{}
	h.Set("Authorization", "Bearer "+apiKey)
	h.Set("Content-Type", "application/json")
	return h, nil
}

func (p *FirecrawlProvider) GetCompleteURL(apiBase string, _ SearchParams) string {
	if apiBase == "" {
		return p.DefaultAPIBase()
	}
	return apiBase
}

func (p *FirecrawlProvider) TransformRequest(params SearchParams) any {
	req := map[string]any{
		"query": params.Query,
		"limit": params.MaxResults,
	}
	if params.MaxResults <= 0 {
		req["limit"] = 5
	}
	return req
}

func (p *FirecrawlProvider) TransformResponse(body []byte) (*SearchResponse, error) {
	var raw struct {
		Data []struct {
			URL         string `json:"url"`
			Title       string `json:"title"`
			Description string `json:"description"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}

	results := make([]SearchResult, len(raw.Data))
	for i, r := range raw.Data {
		results[i] = SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Description,
		}
	}
	return &SearchResponse{Results: results, Object: "list"}, nil
}
