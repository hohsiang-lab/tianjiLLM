package model

// RerankRequest represents a rerank API request.
type RerankRequest struct {
	Model     string   `json:"model"`
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
	TopN      *int     `json:"top_n,omitempty"`
}

// RerankResponse represents a rerank API response.
type RerankResponse struct {
	Results []RerankResult `json:"results"`
	Model   string         `json:"model,omitempty"`
	Usage   *RerankUsage   `json:"usage,omitempty"`
	// Meta holds Cohere-style usage (meta.tokens.input_tokens).
	Meta *RerankMeta `json:"meta,omitempty"`
}

// RerankResult represents a single rerank result.
type RerankResult struct {
	Index          int     `json:"index"`
	RelevanceScore float64 `json:"relevance_score"`
}

// RerankUsage holds token usage for rerank requests.
type RerankUsage struct {
	TotalTokens int `json:"total_tokens"`
}

// RerankMeta holds Cohere-style usage metadata.
type RerankMeta struct {
	Tokens *RerankMetaTokens `json:"tokens,omitempty"`
}

// RerankMetaTokens holds token counts from Cohere-style meta field.
type RerankMetaTokens struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}
