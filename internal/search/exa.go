package search

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func init() {
	Register("exa_ai", &Exa{})
}

// Exa implements SearchProvider for the Exa AI API.
type Exa struct{}

func (e *Exa) Name() string       { return "exa_ai" }
func (e *Exa) HTTPMethod() string { return http.MethodPost }

func (e *Exa) DefaultAPIBase() string {
	return "https://api.exa.ai/search"
}

func (e *Exa) ValidateEnvironment(apiKey, apiBase string) (http.Header, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("exa_ai: EXA_API_KEY required")
	}
	h := http.Header{}
	h.Set("x-api-key", apiKey)
	h.Set("Content-Type", "application/json")
	return h, nil
}

func (e *Exa) GetCompleteURL(apiBase string, _ SearchParams) string {
	if apiBase == "" {
		apiBase = e.DefaultAPIBase()
	}
	return apiBase
}

func (e *Exa) TransformRequest(params SearchParams) any {
	numResults := params.MaxResults
	if numResults <= 0 {
		numResults = 10
	}
	if numResults > 100 {
		numResults = 100
	}
	req := map[string]any{
		"query":      params.Query,
		"numResults": numResults,
		"contents": map[string]any{
			"text": true,
		},
	}
	if len(params.SearchDomainFilter) > 0 {
		req["includeDomains"] = params.SearchDomainFilter
	}
	return req
}

func (e *Exa) TransformResponse(body []byte) (*SearchResponse, error) {
	var raw struct {
		Results []struct {
			Title         string `json:"title"`
			URL           string `json:"url"`
			Text          string `json:"text"`
			PublishedDate string `json:"publishedDate"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("exa_ai: parse response: %w", err)
	}

	results := make([]SearchResult, 0, len(raw.Results))
	for _, r := range raw.Results {
		snippet := r.Text
		if len(snippet) > 500 {
			snippet = snippet[:500]
		}
		results = append(results, SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: snippet,
			Date:    r.PublishedDate,
		})
	}
	return &SearchResponse{Results: results, Object: "search.results"}, nil
}
