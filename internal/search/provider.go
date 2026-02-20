package search

import "net/http"

// SearchProvider defines the interface for search provider implementations.
// Each provider translates between the unified search API and a specific
// upstream search service (Brave, Tavily, etc.).
type SearchProvider interface {
	// Name returns the provider identifier (e.g. "brave", "tavily").
	Name() string

	// HTTPMethod returns "GET" or "POST" for the upstream API.
	HTTPMethod() string

	// ValidateEnvironment checks required credentials and returns auth headers.
	ValidateEnvironment(apiKey, apiBase string) (http.Header, error)

	// GetCompleteURL builds the full upstream URL from base URL and query params.
	GetCompleteURL(apiBase string, params SearchParams) string

	// TransformRequest builds the upstream request body (for POST providers).
	// Returns nil for GET providers that use query params only.
	TransformRequest(params SearchParams) any

	// TransformResponse parses the upstream response into a SearchResponse.
	TransformResponse(body []byte) (*SearchResponse, error)

	// DefaultAPIBase returns the default upstream URL for this provider.
	DefaultAPIBase() string
}

// SearchParams holds the normalized search parameters from the incoming request.
type SearchParams struct {
	Query              string   `json:"query"`
	MaxResults         int      `json:"max_results,omitempty"`
	SearchDomainFilter []string `json:"search_domain_filter,omitempty"`
	Country            string   `json:"country,omitempty"`
}

// SearchResult represents a single search result in Perplexity-compatible format.
type SearchResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Snippet     string `json:"snippet"`
	Date        string `json:"date,omitempty"`
	LastUpdated string `json:"last_updated,omitempty"`
}

// SearchResponse is the container for search results returned to the caller.
type SearchResponse struct {
	Results []SearchResult `json:"results"`
	Object  string         `json:"object"`
}
