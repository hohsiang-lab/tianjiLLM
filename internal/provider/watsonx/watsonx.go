package watsonx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
)

const (
	defaultBaseURL = "https://us-south.ml.cloud.ibm.com"
	iamTokenURL    = "https://iam.cloud.ibm.com/identity/token"
	defaultVersion = "2024-03-13"
)

// Provider implements the IBM WatsonX translation layer.
type Provider struct {
	baseURL    string
	projectID  string
	apiKey     string
	apiVersion string

	mu          sync.RWMutex
	accessToken string
	tokenExpiry time.Time
}

func New(baseURL, projectID, apiKey string) *Provider {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Provider{
		baseURL:    baseURL,
		projectID:  projectID,
		apiKey:     apiKey,
		apiVersion: defaultVersion,
	}
}

func (p *Provider) TransformRequest(ctx context.Context, req *model.ChatCompletionRequest, apiKey string) (*http.Request, error) {
	if apiKey == "" {
		apiKey = p.apiKey
	}

	token, err := p.getIAMToken(ctx, apiKey)
	if err != nil {
		return nil, fmt.Errorf("watsonx auth: %w", err)
	}

	body := map[string]any{
		"model_id":   req.Model,
		"messages":   req.Messages,
		"project_id": p.projectID,
	}

	params := map[string]any{}
	if req.Temperature != nil {
		params["temperature"] = *req.Temperature
	}
	if req.MaxTokens != nil {
		params["max_tokens"] = *req.MaxTokens
	}
	if req.TopP != nil {
		params["top_p"] = *req.TopP
	}
	if req.Stop != nil {
		params["stop_sequences"] = req.Stop
	}
	if len(params) > 0 {
		body["parameters"] = params
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal watsonx request: %w", err)
	}

	endpoint := "chat"
	if req.IsStreaming() {
		endpoint = "chat_stream"
	}
	reqURL := fmt.Sprintf("%s/ml/v1/text/%s?version=%s", p.baseURL, endpoint, p.apiVersion)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create watsonx request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)

	return httpReq, nil
}

func (p *Provider) TransformResponse(_ context.Context, resp *http.Response) (*model.ModelResponse, error) {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &model.TianjiError{
			StatusCode: resp.StatusCode,
			Message:    string(body),
			Type:       "api_error",
			Provider:   "watsonx",
			Err:        model.MapHTTPStatusToError(resp.StatusCode),
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read watsonx response: %w", err)
	}

	var wxResp watsonxResponse
	if err := json.Unmarshal(body, &wxResp); err != nil {
		return nil, fmt.Errorf("parse watsonx response: %w", err)
	}

	return transformToOpenAI(&wxResp), nil
}

func (p *Provider) TransformStreamChunk(_ context.Context, data []byte) (*model.StreamChunk, bool, error) {
	var wxResp watsonxStreamChunk
	if err := json.Unmarshal(data, &wxResp); err != nil {
		return nil, false, err
	}

	chunk := &model.StreamChunk{
		Object: "chat.completion.chunk",
		Model:  wxResp.ModelID,
	}

	if len(wxResp.Choices) > 0 {
		choice := wxResp.Choices[0]
		content := choice.Delta.Content
		streamChoice := model.StreamChoice{
			Index: choice.Index,
			Delta: model.Delta{
				Content: &content,
			},
		}
		if choice.FinishReason != "" {
			streamChoice.FinishReason = &choice.FinishReason
		}
		chunk.Choices = []model.StreamChoice{streamChoice}
	}

	isDone := len(wxResp.Choices) > 0 && wxResp.Choices[0].FinishReason == "stop"
	return chunk, isDone, nil
}

func (p *Provider) GetSupportedParams() []string {
	return []string{
		"model", "messages", "temperature", "max_tokens", "top_p", "stop", "stream",
	}
}

func (p *Provider) MapParams(params map[string]any) map[string]any {
	result := make(map[string]any, len(params))
	for k, v := range params {
		switch k {
		case "stop":
			result["stop_sequences"] = v
		default:
			result[k] = v
		}
	}
	return result
}

func (p *Provider) GetRequestURL(modelName string) string {
	return fmt.Sprintf("%s/ml/v1/text/chat?version=%s", p.baseURL, p.apiVersion)
}

func (p *Provider) SetupHeaders(req *http.Request, _ string) {
	req.Header.Set("Content-Type", "application/json")
}

func (p *Provider) getIAMToken(ctx context.Context, apiKey string) (string, error) {
	p.mu.RLock()
	if p.accessToken != "" && time.Now().Before(p.tokenExpiry) {
		token := p.accessToken
		p.mu.RUnlock()
		return token, nil
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.accessToken != "" && time.Now().Before(p.tokenExpiry) {
		return p.accessToken, nil
	}

	form := url.Values{
		"grant_type": {"urn:ibm:params:oauth:grant-type:apikey"},
		"apikey":     {apiKey},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, iamTokenURL,
		bytes.NewReader([]byte(form.Encode())))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("iam token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("iam token: status %d: %s", resp.StatusCode, body)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("iam token decode: %w", err)
	}

	p.accessToken = tokenResp.AccessToken
	p.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn-10) * time.Second)
	return p.accessToken, nil
}

// WatsonX response types

type watsonxResponse struct {
	ModelID string          `json:"model_id"`
	Choices []watsonxChoice `json:"choices"`
	Usage   *watsonxUsage   `json:"usage"`
}

type watsonxChoice struct {
	Index        int        `json:"index"`
	Message      watsonxMsg `json:"message"`
	FinishReason string     `json:"finish_reason"`
}

type watsonxMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type watsonxUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type watsonxStreamChunk struct {
	ModelID string                `json:"model_id"`
	Choices []watsonxStreamChoice `json:"choices"`
}

type watsonxStreamChoice struct {
	Index        int          `json:"index"`
	Delta        watsonxDelta `json:"delta"`
	FinishReason string       `json:"finish_reason"`
}

type watsonxDelta struct {
	Content string `json:"content"`
}

func transformToOpenAI(resp *watsonxResponse) *model.ModelResponse {
	var choices []model.Choice
	for _, c := range resp.Choices {
		fr := c.FinishReason
		choices = append(choices, model.Choice{
			Index: c.Index,
			Message: &model.Message{
				Role:    c.Message.Role,
				Content: c.Message.Content,
			},
			FinishReason: &fr,
		})
	}

	result := &model.ModelResponse{
		Object:  "chat.completion",
		Model:   resp.ModelID,
		Choices: choices,
	}

	if resp.Usage != nil {
		result.Usage = model.Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}
	}

	return result
}

func init() {
	baseURL := os.Getenv("WATSONX_URL")
	projectID := os.Getenv("WATSONX_PROJECT_ID")
	apiKey := os.Getenv("WATSONX_APIKEY")
	provider.Register("watsonx", New(baseURL, projectID, apiKey))
}
