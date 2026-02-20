package search

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

func init() {
	Register("dataforseo", &DataForSEO{})
}

// DataForSEO implements SearchProvider for the DataForSEO SERP API.
type DataForSEO struct{}

func (d *DataForSEO) Name() string       { return "dataforseo" }
func (d *DataForSEO) HTTPMethod() string { return http.MethodPost }

func (d *DataForSEO) DefaultAPIBase() string {
	return "https://api.dataforseo.com/v3/serp/google/organic/live/advanced"
}

func (d *DataForSEO) ValidateEnvironment(apiKey, apiBase string) (http.Header, error) {
	login := os.Getenv("DATAFORSEO_LOGIN")
	password := os.Getenv("DATAFORSEO_PASSWORD")
	if login == "" || password == "" {
		return nil, fmt.Errorf("dataforseo: DATAFORSEO_LOGIN and DATAFORSEO_PASSWORD required")
	}
	creds := base64.StdEncoding.EncodeToString([]byte(login + ":" + password))
	h := http.Header{}
	h.Set("Authorization", "Basic "+creds)
	h.Set("Content-Type", "application/json")
	return h, nil
}

func (d *DataForSEO) GetCompleteURL(apiBase string, _ SearchParams) string {
	if apiBase == "" {
		apiBase = d.DefaultAPIBase()
	}
	return apiBase
}

func (d *DataForSEO) TransformRequest(params SearchParams) any {
	depth := params.MaxResults
	if depth <= 0 {
		depth = 10
	}
	langCode := "en"
	if params.Country != "" {
		langCode = params.Country
	}
	return []map[string]any{
		{
			"keyword":       params.Query,
			"depth":         depth,
			"language_code": langCode,
		},
	}
}

func (d *DataForSEO) TransformResponse(body []byte) (*SearchResponse, error) {
	var raw struct {
		Tasks []struct {
			Result []struct {
				Items []struct {
					Type        string `json:"type"`
					Title       string `json:"title"`
					URL         string `json:"url"`
					Description string `json:"description"`
				} `json:"items"`
			} `json:"result"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("dataforseo: parse response: %w", err)
	}

	var results []SearchResult
	if len(raw.Tasks) > 0 && len(raw.Tasks[0].Result) > 0 {
		for _, item := range raw.Tasks[0].Result[0].Items {
			if item.Type != "organic" {
				continue
			}
			results = append(results, SearchResult{
				Title:   item.Title,
				URL:     item.URL,
				Snippet: item.Description,
			})
		}
	}
	return &SearchResponse{Results: results, Object: "search.results"}, nil
}
