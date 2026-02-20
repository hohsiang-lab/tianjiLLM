package search

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func init() {
	Register("tavily", &Tavily{})
}

// Tavily implements SearchProvider for the Tavily Search API.
type Tavily struct{}

func (t *Tavily) Name() string       { return "tavily" }
func (t *Tavily) HTTPMethod() string { return http.MethodPost }

func (t *Tavily) DefaultAPIBase() string {
	return "https://api.tavily.com/search"
}

func (t *Tavily) ValidateEnvironment(apiKey, apiBase string) (http.Header, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("tavily: TAVILY_API_KEY required")
	}
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	return h, nil
}

func (t *Tavily) GetCompleteURL(apiBase string, _ SearchParams) string {
	if apiBase == "" {
		apiBase = t.DefaultAPIBase()
	}
	return apiBase
}

func (t *Tavily) TransformRequest(params SearchParams) any {
	maxResults := params.MaxResults
	if maxResults <= 0 {
		maxResults = 5
	}
	req := map[string]any{
		"query":       params.Query,
		"max_results": maxResults,
	}
	if len(params.SearchDomainFilter) > 0 {
		req["include_domains"] = params.SearchDomainFilter
	}
	return req
}

func (t *Tavily) TransformResponse(body []byte) (*SearchResponse, error) {
	var raw struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("tavily: parse response: %w", err)
	}

	results := make([]SearchResult, 0, len(raw.Results))
	for _, r := range raw.Results {
		results = append(results, SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Content,
		})
	}
	return &SearchResponse{Results: results, Object: "search.results"}, nil
}
