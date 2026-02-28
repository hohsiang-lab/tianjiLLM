package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/pricing"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/anthropic"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/middleware"
)

// nativeProxy creates a reverse proxy to a specific provider's base URL.
func (h *Handlers) nativeProxy(w http.ResponseWriter, r *http.Request, providerName string) {
	baseURL, apiKey := h.resolveNativeUpstream(providerName)
	if baseURL == "" {
		writeJSON(w, http.StatusNotImplemented, model.ErrorResponse{
			Error: model.ErrorDetail{Message: providerName + " not configured", Type: "not_supported"},
		})
		return
	}

	target, err := url.Parse(baseURL)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid upstream URL", Type: "internal_error"},
		})
		return
	}

	// Read request body to extract model name as fallback for spend logging.
	requestModel := extractRequestModel(r)

	startTime := time.Now()
	ctx := r.Context()

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host

			// Remove client's auth headers — we inject the provider's own credentials
			req.Header.Del("Authorization")
			req.Header.Del("x-api-key")

			switch providerName {
			case "anthropic":
				if anthropic.IsOAuthToken(apiKey) {
					req.Header.Set("Authorization", "Bearer "+apiKey)
					// Append OAuth beta flag to client's existing beta headers
					if existing := req.Header.Get("anthropic-beta"); existing != "" {
						req.Header.Set("anthropic-beta", existing+","+anthropic.OAuthBetaHeader)
					} else {
						req.Header.Set("anthropic-beta", anthropic.OAuthBetaHeader)
					}
				} else {
					req.Header.Set("x-api-key", apiKey)
				}
				// Preserve client's anthropic-version; only set default if missing
				if req.Header.Get("anthropic-version") == "" {
					req.Header.Set("anthropic-version", "2023-06-01")
				}
			default:
				if apiKey != "" {
					req.Header.Set("Authorization", "Bearer "+apiKey)
				}
			}
		},
		ModifyResponse: func(resp *http.Response) error {
			if resp.StatusCode != http.StatusOK {
				if h.DB != nil {
					body, _ := io.ReadAll(resp.Body)
					resp.Body = io.NopCloser(bytes.NewReader(body))
					errMsg := fmt.Sprintf("upstream error: status %d", resp.StatusCode)
					if len(body) > 0 {
						errMsg = string(body)
					}
					apiKeyHash := ""
					if v, ok := ctx.Value(middleware.ContextKeyTokenHash).(string); ok {
						apiKeyHash = v
					}
					requestID := chiMiddleware.GetReqID(ctx)
					go func() {
						_ = h.DB.InsertErrorLog(context.Background(), db.InsertErrorLogParams{
							RequestID:    requestID,
							ApiKeyHash:   apiKeyHash,
							Model:        "",
							Provider:     providerName,
							StatusCode:   int32(resp.StatusCode),
							ErrorType:    "upstream_error",
							ErrorMessage: errMsg,
						})
					}()
				}
				return nil
			}

			if h.Callbacks == nil {
				return nil
			}

			streaming := strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream")

			if streaming {
				// Wrap body: tee all bytes while streaming to client,
				// parse usage on Close after stream ends.
				resp.Body = &sseSpendReader{
					src:          resp.Body,
					providerName: providerName,
					startTime:    startTime,
					ctx:          ctx,
					callbacks:    h.Callbacks,
					requestModel: requestModel,
				}
				return nil
			}

			// Non-streaming: read body, parse usage, restore body.
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil
			}
			resp.Body = io.NopCloser(bytes.NewReader(body))

			prompt, completion, modelName := parseUsage(providerName, body)
			if modelName == "" {
				modelName = requestModel
			}
			go h.Callbacks.LogSuccess(buildNativeLogData(
				ctx, providerName, modelName, startTime,
				prompt, completion,
			))
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("native proxy error (%s): %v", providerName, err)
			http.Error(w, `{"error":"upstream request failed"}`, http.StatusBadGateway)
		},
	}

	proxy.ServeHTTP(w, r)
}

// extractRequestModel reads the "model" field from the request body JSON
// without consuming it (the body is re-set for downstream use).
func extractRequestModel(r *http.Request) string {
	if r.Body == nil {
		return ""
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return ""
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	var partial struct {
		Model string `json:"model"`
	}
	if json.Unmarshal(body, &partial) != nil {
		return ""
	}
	return partial.Model
}

// parseUsage extracts prompt/completion tokens and model name from a non-streaming response body.
func parseUsage(providerName string, body []byte) (prompt, completion int, modelName string) {
	switch providerName {
	case "anthropic":
		var parsed struct {
			Model string `json:"model"`
			Usage struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}
		if json.Unmarshal(body, &parsed) == nil {
			return parsed.Usage.InputTokens, parsed.Usage.OutputTokens, parsed.Model
		}
	case "gemini":
		var parsed struct {
			UsageMetadata struct {
				PromptTokenCount     int `json:"promptTokenCount"`
				CandidatesTokenCount int `json:"candidatesTokenCount"`
			} `json:"usageMetadata"`
		}
		if json.Unmarshal(body, &parsed) == nil {
			return parsed.UsageMetadata.PromptTokenCount, parsed.UsageMetadata.CandidatesTokenCount, ""
		}
	}
	return 0, 0, ""
}

// buildNativeLogData constructs a LogData from native proxy usage info.
func buildNativeLogData(ctx context.Context, providerName, modelName string, startTime time.Time, prompt, completion int) callback.LogData {
	endTime := time.Now()
	data := callback.LogData{
		Model:            modelName,
		Provider:         providerName,
		StartTime:        startTime,
		EndTime:          endTime,
		Latency:          endTime.Sub(startTime),
		PromptTokens:     prompt,
		CompletionTokens: completion,
		TotalTokens:      prompt + completion,
		Cost:             pricing.Default().TotalCost(modelName, prompt, completion),
	}
	if tokenHash, ok := ctx.Value(middleware.ContextKeyTokenHash).(string); ok {
		data.APIKey = tokenHash
	}
	if userID, ok := ctx.Value(middleware.ContextKeyUserID).(string); ok {
		data.UserID = userID
	}
	if teamID, ok := ctx.Value(middleware.ContextKeyTeamID).(string); ok {
		data.TeamID = teamID
	}
	return data
}

// sseSpendReader wraps a streaming response body. It tees all bytes into a buffer
// while the reverse proxy streams them to the client. On Close, it parses the
// collected SSE events to extract usage and fires the spend callback.
type sseSpendReader struct {
	src          io.ReadCloser
	buf          bytes.Buffer
	providerName string
	requestModel string
	startTime    time.Time
	ctx          context.Context
	callbacks    *callback.Registry
}

func (r *sseSpendReader) Read(p []byte) (int, error) {
	n, err := r.src.Read(p)
	if n > 0 {
		r.buf.Write(p[:n])
	}
	return n, err
}

func (r *sseSpendReader) Close() error {
	err := r.src.Close()

	prompt, completion, modelName := parseSSEUsage(r.providerName, r.buf.Bytes())
	if modelName == "" {
		modelName = r.requestModel
	}
	go r.callbacks.LogSuccess(buildNativeLogData(
		r.ctx, r.providerName, modelName, r.startTime,
		prompt, completion,
	))
	return err
}

// parseSSEUsage scans SSE events for usage data.
// Anthropic: model in message_start, usage in message_delta.
func parseSSEUsage(providerName string, raw []byte) (prompt, completion int, modelName string) {
	// Split into lines and process "data: " prefixed lines.
	for _, line := range bytes.Split(raw, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if !bytes.HasPrefix(line, []byte("data: ")) {
			continue
		}
		data := line[6:] // strip "data: "

		switch providerName {
		case "anthropic":
			var event struct {
				Type    string `json:"type"`
				Message struct {
					Model string `json:"model"`
					Usage struct {
						InputTokens  int `json:"input_tokens"`
						OutputTokens int `json:"output_tokens"`
					} `json:"usage"`
				} `json:"message"`
				Usage struct {
					InputTokens  int `json:"input_tokens"`
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
			}
			if json.Unmarshal(data, &event) != nil {
				continue
			}
			if event.Type == "message_start" && event.Message.Model != "" {
				modelName = event.Message.Model
			}
			// message_start carries input_tokens in message.usage
			if event.Type == "message_start" && event.Message.Usage.InputTokens > 0 {
				prompt = event.Message.Usage.InputTokens
			}
			// message_delta carries output_tokens in root usage
			if event.Type == "message_delta" && event.Usage.OutputTokens > 0 {
				completion = event.Usage.OutputTokens
			}

		case "gemini":
			var event struct {
				ModelVersion  string `json:"modelVersion"`
				UsageMetadata struct {
					PromptTokenCount     int `json:"promptTokenCount"`
					CandidatesTokenCount int `json:"candidatesTokenCount"`
				} `json:"usageMetadata"`
			}
			if json.Unmarshal(data, &event) != nil {
				continue
			}
			if event.ModelVersion != "" {
				modelName = event.ModelVersion
			}
			// Each chunk may have usageMetadata; take the last one.
			if event.UsageMetadata.PromptTokenCount > 0 || event.UsageMetadata.CandidatesTokenCount > 0 {
				prompt = event.UsageMetadata.PromptTokenCount
				completion = event.UsageMetadata.CandidatesTokenCount
			}
		}
	}
	return
}

// resolveNativeUpstream finds the base URL and API key for a native provider.
func (h *Handlers) resolveNativeUpstream(providerName string) (string, string) {
	for _, m := range h.Config.ModelList {
		parts := strings.SplitN(m.TianjiParams.Model, "/", 2)
		if len(parts) >= 1 && parts[0] == providerName {
			apiKey := ""
			if m.TianjiParams.APIKey != nil {
				apiKey = *m.TianjiParams.APIKey
			}
			base := ""
			if m.TianjiParams.APIBase != nil {
				base = *m.TianjiParams.APIBase
			}
			if base == "" {
				base = defaultBaseURL(providerName)
			}
			return base, apiKey
		}
	}
	return "", ""
}

func defaultBaseURL(provider string) string {
	switch provider {
	case "openai":
		return "https://api.openai.com"
	case "anthropic":
		return "https://api.anthropic.com"
	case "gemini":
		return "https://generativelanguage.googleapis.com"
	case "cohere":
		return "https://api.cohere.ai"
	case "mistral":
		return "https://api.mistral.ai"
	default:
		return ""
	}
}

// AnthropicMessages handles POST /v1/messages (Anthropic native format).
func (h *Handlers) AnthropicMessages(w http.ResponseWriter, r *http.Request) {
	h.nativeProxy(w, r, "anthropic")
}

// AnthropicCountTokens handles POST /v1/messages/count_tokens.
func (h *Handlers) AnthropicCountTokens(w http.ResponseWriter, r *http.Request) {
	h.nativeProxy(w, r, "anthropic")
}

// GeminiGenerateContent handles POST /v1beta/models/{name}:generateContent.
func (h *Handlers) GeminiGenerateContent(w http.ResponseWriter, r *http.Request) {
	h.nativeProxy(w, r, "gemini")
}

// GeminiStreamGenerateContent handles POST /v1beta/models/{name}:streamGenerateContent.
func (h *Handlers) GeminiStreamGenerateContent(w http.ResponseWriter, r *http.Request) {
	h.nativeProxy(w, r, "gemini")
}

// GeminiCountTokens handles POST /v1beta/models/{name}:countTokens.
func (h *Handlers) GeminiCountTokens(w http.ResponseWriter, r *http.Request) {
	h.nativeProxy(w, r, "gemini")
}

// ImagesEdit handles POST /v1/images/edits.
func (h *Handlers) ImagesEdit(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// ImageVariation handles POST /v1/images/variations.
func (h *Handlers) ImageVariation(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}
