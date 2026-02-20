package search

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

func init() {
	Register("searxng", &SearXNG{})
}

// SearXNG implements SearchProvider for SearXNG instances.
type SearXNG struct{}

func (s *SearXNG) Name() string       { return "searxng" }
func (s *SearXNG) HTTPMethod() string { return http.MethodGet }
func (s *SearXNG) DefaultAPIBase() string {
	return "http://localhost:8888/search"
}

var countryToLang = map[string]string{
	"us": "en", "gb": "en", "de": "de", "fr": "fr",
	"es": "es", "it": "it", "jp": "ja", "kr": "ko",
	"cn": "zh", "br": "pt", "ru": "ru", "nl": "nl",
}

func (s *SearXNG) ValidateEnvironment(apiKey, apiBase string) (http.Header, error) {
	if apiBase == "" {
		return nil, fmt.Errorf("searxng: api_base required (SearXNG instance URL)")
	}
	h := http.Header{}
	if apiKey != "" {
		h.Set("Authorization", "Bearer "+apiKey)
	}
	h.Set("Accept", "application/json")
	return h, nil
}

func (s *SearXNG) GetCompleteURL(apiBase string, params SearchParams) string {
	if apiBase == "" {
		apiBase = s.DefaultAPIBase()
	}
	v := url.Values{}
	v.Set("q", params.Query)
	v.Set("format", "json")
	if params.Country != "" {
		if lang, ok := countryToLang[params.Country]; ok {
			v.Set("language", lang)
		}
	}
	return apiBase + "?" + v.Encode()
}

func (s *SearXNG) TransformRequest(_ SearchParams) any { return nil }

func (s *SearXNG) TransformResponse(body []byte) (*SearchResponse, error) {
	var raw struct {
		Results []struct {
			Title       string `json:"title"`
			URL         string `json:"url"`
			Content     string `json:"content"`
			PublishDate string `json:"publishedDate"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("searxng: parse response: %w", err)
	}

	results := make([]SearchResult, 0, len(raw.Results))
	for _, r := range raw.Results {
		results = append(results, SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Content,
			Date:    r.PublishDate,
		})
	}
	return &SearchResponse{Results: results, Object: "search.results"}, nil
}
