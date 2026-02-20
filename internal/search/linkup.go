package search

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func init() {
	Register("linkup", &LinkupProvider{})
}

// LinkupProvider implements search via Linkup API.
type LinkupProvider struct{}

func (p *LinkupProvider) Name() string           { return "linkup" }
func (p *LinkupProvider) HTTPMethod() string     { return "POST" }
func (p *LinkupProvider) DefaultAPIBase() string { return "https://api.linkup.so/v1/search" }

func (p *LinkupProvider) ValidateEnvironment(apiKey, _ string) (http.Header, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("LINKUP_API_KEY required")
	}
	h := http.Header{}
	h.Set("Authorization", "Bearer "+apiKey)
	h.Set("Content-Type", "application/json")
	return h, nil
}

func (p *LinkupProvider) GetCompleteURL(apiBase string, _ SearchParams) string {
	if apiBase == "" {
		return p.DefaultAPIBase()
	}
	return apiBase
}

func (p *LinkupProvider) TransformRequest(params SearchParams) any {
	return map[string]any{
		"q":          params.Query,
		"depth":      "standard",
		"outputType": "searchResults",
	}
}

func (p *LinkupProvider) TransformResponse(body []byte) (*SearchResponse, error) {
	var raw struct {
		Results []struct {
			Name    string `json:"name"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}

	results := make([]SearchResult, len(raw.Results))
	for i, r := range raw.Results {
		results[i] = SearchResult{
			Title:   r.Name,
			URL:     r.URL,
			Snippet: r.Content,
		}
	}
	return &SearchResponse{Results: results, Object: "list"}, nil
}
