package search

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
)

func init() {
	Register("google_pse", &GooglePSE{})
}

// GooglePSE implements SearchProvider for Google Programmable Search Engine.
type GooglePSE struct{}

func (g *GooglePSE) Name() string       { return "google_pse" }
func (g *GooglePSE) HTTPMethod() string { return http.MethodGet }

func (g *GooglePSE) DefaultAPIBase() string {
	return "https://www.googleapis.com/customsearch/v1"
}

func (g *GooglePSE) ValidateEnvironment(apiKey, apiBase string) (http.Header, error) {
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_PSE_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("google_pse: GOOGLE_PSE_API_KEY required")
	}
	cx := os.Getenv("GOOGLE_PSE_ENGINE_ID")
	if cx == "" {
		return nil, fmt.Errorf("google_pse: GOOGLE_PSE_ENGINE_ID required")
	}
	h := http.Header{}
	h.Set("Accept", "application/json")
	return h, nil
}

func (g *GooglePSE) GetCompleteURL(apiBase string, params SearchParams) string {
	if apiBase == "" {
		apiBase = g.DefaultAPIBase()
	}
	apiKey := os.Getenv("GOOGLE_PSE_API_KEY")
	cx := os.Getenv("GOOGLE_PSE_ENGINE_ID")

	v := url.Values{}
	v.Set("key", apiKey)
	v.Set("cx", cx)
	v.Set("q", params.Query)

	num := params.MaxResults
	if num <= 0 {
		num = 10
	}
	if num > 10 {
		num = 10
	}
	v.Set("num", fmt.Sprintf("%d", num))

	if len(params.SearchDomainFilter) > 0 {
		v.Set("siteSearch", params.SearchDomainFilter[0])
	}
	return apiBase + "?" + v.Encode()
}

func (g *GooglePSE) TransformRequest(_ SearchParams) any { return nil }

func (g *GooglePSE) TransformResponse(body []byte) (*SearchResponse, error) {
	var raw struct {
		Items []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("google_pse: parse response: %w", err)
	}

	results := make([]SearchResult, 0, len(raw.Items))
	for _, item := range raw.Items {
		results = append(results, SearchResult{
			Title:   item.Title,
			URL:     item.Link,
			Snippet: item.Snippet,
		})
	}
	return &SearchResponse{Results: results, Object: "search.results"}, nil
}
