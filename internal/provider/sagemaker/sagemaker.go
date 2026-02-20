package sagemaker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
)

// Provider translates OpenAI requests to SageMaker InvokeEndpoint calls.
type Provider struct {
	region string
}

func New(region string) *Provider {
	if region == "" {
		region = "us-east-1"
	}
	return &Provider{region: region}
}

func (p *Provider) TransformRequest(ctx context.Context, req *model.ChatCompletionRequest, _ string) (*http.Request, error) {
	// SageMaker HuggingFace Messages API: forward OpenAI-format body directly.
	body := map[string]any{
		"messages": req.Messages,
	}
	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}
	if req.MaxTokens != nil {
		body["max_tokens"] = *req.MaxTokens
	}
	if req.TopP != nil {
		body["top_p"] = *req.TopP
	}
	if req.Stop != nil {
		body["stop"] = req.Stop
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal sagemaker request: %w", err)
	}

	endpointName := req.Model // model name is the SageMaker endpoint name
	url := fmt.Sprintf("https://runtime.sagemaker.%s.amazonaws.com/endpoints/%s/invocations", p.region, endpointName)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create sagemaker request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Sign with AWS SigV4.
	if err := p.signRequest(ctx, httpReq, data); err != nil {
		return nil, fmt.Errorf("sign sagemaker request: %w", err)
	}

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
			Provider:   "sagemaker",
			Err:        model.MapHTTPStatusToError(resp.StatusCode),
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read sagemaker response: %w", err)
	}

	// HuggingFace Messages API returns OpenAI-compatible format directly.
	var result model.ModelResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse sagemaker response: %w", err)
	}

	return &result, nil
}

func (p *Provider) TransformStreamChunk(_ context.Context, data []byte) (*model.StreamChunk, bool, error) {
	// SageMaker streaming uses AWS EventStream; for HF Messages API, SSE format.
	var chunk model.StreamChunk
	if err := json.Unmarshal(data, &chunk); err != nil {
		return nil, false, err
	}

	isDone := false
	if len(chunk.Choices) > 0 && chunk.Choices[0].FinishReason != nil {
		isDone = *chunk.Choices[0].FinishReason == "stop"
	}
	return &chunk, isDone, nil
}

func (p *Provider) GetSupportedParams() []string {
	return []string{
		"model", "messages", "temperature", "max_tokens", "top_p", "stop", "stream",
	}
}

func (p *Provider) MapParams(params map[string]any) map[string]any {
	return params
}

func (p *Provider) GetRequestURL(modelName string) string {
	return fmt.Sprintf("https://runtime.sagemaker.%s.amazonaws.com/endpoints/%s/invocations", p.region, modelName)
}

func (p *Provider) SetupHeaders(req *http.Request, _ string) {
	req.Header.Set("Content-Type", "application/json")
}

func (p *Provider) signRequest(ctx context.Context, req *http.Request, body []byte) error {
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(p.region))
	if err != nil {
		return err
	}

	creds, err := cfg.Credentials.Retrieve(ctx)
	if err != nil {
		return err
	}

	signer := v4.NewSigner()
	h := sha256.Sum256(body)
	hash := hex.EncodeToString(h[:])
	return signer.SignHTTP(ctx, creds, req, hash, "sagemaker", p.region, time.Now())
}

func init() {
	region := os.Getenv("AWS_REGION_NAME")
	if region == "" {
		region = os.Getenv("AWS_DEFAULT_REGION")
	}
	provider.Register("sagemaker", New(region))
}
