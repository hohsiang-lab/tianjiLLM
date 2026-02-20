package vertexai

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/gemini"
	"golang.org/x/oauth2/google"
)

// Provider wraps Gemini with Vertex AI regional endpoints and OAuth2 auth.
type Provider struct {
	inner     *gemini.Provider
	projectID string
	location  string

	mu          sync.RWMutex
	accessToken string
	tokenExpiry time.Time
}

func New(projectID, location string) *Provider {
	if location == "" {
		location = "us-central1"
	}
	return &Provider{
		inner:     gemini.NewVertex(projectID, location),
		projectID: projectID,
		location:  location,
	}
}

func (p *Provider) TransformRequest(ctx context.Context, req *model.ChatCompletionRequest, apiKey string) (*http.Request, error) {
	// If apiKey is empty, try to get an OAuth2 token via ADC.
	if apiKey == "" {
		token, err := p.getAccessToken(ctx)
		if err != nil {
			return nil, fmt.Errorf("vertex_ai auth: %w", err)
		}
		apiKey = token
	}
	return p.inner.TransformRequest(ctx, req, apiKey)
}

func (p *Provider) TransformResponse(ctx context.Context, resp *http.Response) (*model.ModelResponse, error) {
	return p.inner.TransformResponse(ctx, resp)
}

func (p *Provider) TransformStreamChunk(ctx context.Context, data []byte) (*model.StreamChunk, bool, error) {
	return p.inner.TransformStreamChunk(ctx, data)
}

func (p *Provider) GetSupportedParams() []string {
	return p.inner.GetSupportedParams()
}

func (p *Provider) MapParams(params map[string]any) map[string]any {
	return p.inner.MapParams(params)
}

func (p *Provider) GetRequestURL(modelName string) string {
	return p.inner.GetRequestURL(modelName)
}

func (p *Provider) SetupHeaders(req *http.Request, apiKey string) {
	p.inner.SetupHeaders(req, apiKey)
}

func (p *Provider) getAccessToken(ctx context.Context) (string, error) {
	p.mu.RLock()
	if p.accessToken != "" && time.Now().Before(p.tokenExpiry) {
		token := p.accessToken
		p.mu.RUnlock()
		return token, nil
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check after acquiring write lock.
	if p.accessToken != "" && time.Now().Before(p.tokenExpiry) {
		return p.accessToken, nil
	}

	creds, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return "", fmt.Errorf("find default credentials: %w", err)
	}

	token, err := creds.TokenSource.Token()
	if err != nil {
		return "", fmt.Errorf("get token: %w", err)
	}

	p.accessToken = token.AccessToken
	p.tokenExpiry = token.Expiry.Add(-30 * time.Second)
	return p.accessToken, nil
}

func init() {
	projectID := os.Getenv("VERTEX_PROJECT")
	location := os.Getenv("VERTEX_LOCATION")
	provider.Register("vertex_ai", New(projectID, location))
}
