package search

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func init() {
	Register("brave", &Brave{})
}

// Brave implements SearchProvider for the Brave Search API.
type Brave struct{}

func (b *Brave) Name() string       { return "brave" }
func (b *Brave) HTTPMethod() string { return http.MethodGet }

func (b *Brave) DefaultAPIBase() string {
	return "https://api.search.brave.com/res/v1/web/search"
}

func (b *Brave) ValidateEnvironment(apiKey, apiBase string) (http.Header, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("brave: BRAVE_API_KEY required")
	}
	h := http.Header{}
	h.Set("X-Subscription-Token", apiKey)
	h.Set("Accept", "application/json")
	return h, nil
}

func (b *Brave) GetCompleteURL(apiBase string, params SearchParams) string {
	if apiBase == "" {
		apiBase = b.DefaultAPIBase()
	}
	q := params.Query
	for _, domain := range params.SearchDomainFilter {
		q += " site:" + domain
	}
	v := url.Values{}
	v.Set("q", q)
	count := params.MaxResults
	if count <= 0 {
		count = 10
	}
	if count > 20 {
		count = 20
	}
	v.Set("count", fmt.Sprintf("%d", count))
	return apiBase + "?" + v.Encode()
}

func (b *Brave) TransformRequest(_ SearchParams) any { return nil }

func (b *Brave) TransformResponse(body []byte) (*SearchResponse, error) {
	var raw struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"results"`
		} `json:"web"`
		News struct {
			Results []struct {
				Title string `json:"title"`
				URL   string `json:"url"`
				Desc  string `json:"description"`
				Age   string `json:"age"`
			} `json:"results"`
		} `json:"news"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("brave: parse response: %w", err)
	}

	var results []SearchResult
	for _, r := range raw.Web.Results {
		results = append(results, SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: strings.TrimSpace(r.Description),
		})
	}
	for _, r := range raw.News.Results {
		results = append(results, SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: strings.TrimSpace(r.Desc),
			Date:    r.Age,
		})
	}

	return &SearchResponse{Results: results, Object: "search.results"}, nil
}
