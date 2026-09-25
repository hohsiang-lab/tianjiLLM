package auto

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Encoder wraps the proxy's own embedding endpoint to generate vectors.
type Encoder struct {
	model   string
	baseURL string
	apiKey  string
	client  *http.Client
}

// NewEncoder creates an encoder that calls the proxy's embedding endpoint.
func NewEncoder(model, baseURL, apiKey string) *Encoder {
	if baseURL == "" {
		baseURL = "http://localhost:4000"
	}
	return &Encoder{
		model:   model,
		baseURL: baseURL,
		apiKey:  apiKey,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

type embeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

// Encode returns embedding vectors for the given texts.
func (e *Encoder) Encode(ctx context.Context, texts []string) ([][]float32, error) {
	body, err := json.Marshal(embeddingRequest{
		Model: e.model,
		Input: texts,
	})
	if err != nil {
		return nil, fmt.Errorf("auto encoder: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("auto encoder: request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if e.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.apiKey)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("auto encoder: call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("auto encoder: status %d: %s", resp.StatusCode, string(respBody))
	}

	var embResp embeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
		return nil, fmt.Errorf("auto encoder: decode: %w", err)
	}

	vectors := make([][]float32, len(embResp.Data))
	for i, d := range embResp.Data {
		vectors[i] = d.Embedding
	}
	return vectors, nil
}
