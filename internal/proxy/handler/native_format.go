package handler

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync/atomic"
	"time"

	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/pricing"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/anthropic"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/middleware"
	"github.com/praxisllmlab/tianjiLLM/internal/ratelimitstate"
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
					var teamID *string
					if tid, ok := ctx.Value(middleware.ContextKeyTeamID).(string); ok && tid != "" {
						teamID = &tid
					}
					go func() {
						_ = h.DB.InsertErrorLog(context.Background(), db.InsertErrorLogParams{
							RequestID:    requestID,
							ApiKeyHash:   apiKeyHash,
							Model:        "",
							Provider:     providerName,
							StatusCode:   int32(resp.StatusCode),
							ErrorType:    "upstream_error",
							ErrorMessage: errMsg,
							TeamID:       teamID,
						})
					}()
				}
				return nil
			}

			if providerName == "anthropic" {
				state := callback.ParseAnthropicRateLimitHeaders(resp.Header)
				if h.DiscordAlerter != nil {
					h.DiscordAlerter.CheckAndAlert(state)
				}
				// Store rate limit state per apiKey for Usage page widget.
				if apiKey != "" {
					hash := sha256.Sum256([]byte(apiKey))
					keyHash := hex.EncodeToString(hash[:])
					snap := &ratelimitstate.Snapshot{
						CapturedAt: time.Now().UTC(),
					}
					if state.InputTokensLimit >= 0 || state.InputTokensRemaining >= 0 {
						snap.InputTokens = &ratelimitstate.DimensionState{
							Limit:     state.InputTokensLimit,
							Remaining: state.InputTokensRemaining,
						}
					}
					if state.OutputTokensLimit >= 0 || state.OutputTokensRemaining >= 0 {
						snap.OutputTokens = &ratelimitstate.DimensionState{
							Limit:     state.OutputTokensLimit,
							Remaining: state.OutputTokensRemaining,
						}
					}
					if state.RequestsLimit >= 0 || state.RequestsRemaining >= 0 {
						snap.Requests = &ratelimitstate.DimensionState{
							Limit:     state.RequestsLimit,
							Remaining: state.RequestsRemaining,
						}
					}
					ratelimitstate.GetOrCreate(keyHash).Set(snap)
				}
			}

			if h.Callbacks == nil {
				return nil
			}

			// Transparently decompress gzip responses (like LiteLLM/httpx).
			// Go's ReverseProxy passes through compressed bytes as-is, but we
			// need plaintext to parse usage tokens and for consistent client behavior.
			if resp.Header.Get("Content-Encoding") == "gzip" {
				gr, gzErr := gzip.NewReader(resp.Body)
				if gzErr == nil {
					resp.Body = &gzipReadCloser{gz: gr, orig: resp.Body}
					resp.Header.Del("Content-Encoding")
					resp.Header.Del("Content-Length") // length changes after decompression
				}
			}

			streaming := strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream")

			if streaming {
				// Wrap body: tee all bytes while streaming to client,
				// parse usage on Close after stream ends.
				//
				// IMPORTANT: We wrap in readCloserOnly to prevent io.Copy
				// from using the dst's ReadFrom optimization (e.g. chi's
				// WrapResponseWriter implements io.ReaderFrom). Without
				// this wrapper, io.Copy calls dst.ReadFrom(src) which
				// reads directly from the underlying body via splice/sendfile,
				// bypassing our Read() method and leaving buf empty.
				ssr := &sseSpendReader{
					src:          resp.Body,
					providerName: providerName,
					startTime:    startTime,
					ctx:          ctx,
					callbacks:    h.Callbacks,
					requestModel: requestModel,
				}
				resp.Body = readCloserOnly{ssr}
				return nil
			}

			// Non-streaming: read body, parse usage, restore body.
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil
			}
			resp.Body = io.NopCloser(bytes.NewReader(body))

			prompt, completion, cacheRead, cacheCreation, modelName := parseUsage(providerName, body)
			if modelName == "" {
				modelName = requestModel
			}
			go h.Callbacks.LogSuccess(buildNativeLogData(
				ctx, providerName, modelName, startTime,
				prompt, completion, cacheRead, cacheCreation,
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
func parseUsage(providerName string, body []byte) (prompt, completion, cacheRead, cacheCreation int, modelName string) {
	switch providerName {
	case "anthropic":
		var parsed struct {
			Model string `json:"model"`
			Usage struct {
				InputTokens              int `json:"input_tokens"`
				OutputTokens             int `json:"output_tokens"`
				CacheReadInputTokens     int `json:"cache_read_input_tokens"`
				CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
			} `json:"usage"`
		}
		if json.Unmarshal(body, &parsed) == nil {
			cr := parsed.Usage.CacheReadInputTokens
			cc := parsed.Usage.CacheCreationInputTokens
			return parsed.Usage.InputTokens + cr + cc, parsed.Usage.OutputTokens, cr, cc, parsed.Model
		}
	case "gemini":
		var parsed struct {
			UsageMetadata struct {
				PromptTokenCount     int `json:"promptTokenCount"`
				CandidatesTokenCount int `json:"candidatesTokenCount"`
			} `json:"usageMetadata"`
		}
		if json.Unmarshal(body, &parsed) == nil {
			return parsed.UsageMetadata.PromptTokenCount, parsed.UsageMetadata.CandidatesTokenCount, 0, 0, ""
		}
	case "openai", "openrouter", "deepseek", "groq", "together":
		var parsed struct {
			Model string `json:"model"`
			Usage struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
			} `json:"usage"`
		}
		if json.Unmarshal(body, &parsed) == nil {
			return parsed.Usage.PromptTokens, parsed.Usage.CompletionTokens, 0, 0, parsed.Model
		}
	default:
		// Fallback: try OpenAI-compatible format for unknown providers
		var parsed struct {
			Model string `json:"model"`
			Usage struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
			} `json:"usage"`
		}
		if json.Unmarshal(body, &parsed) == nil && (parsed.Usage.PromptTokens > 0 || parsed.Usage.CompletionTokens > 0) {
			return parsed.Usage.PromptTokens, parsed.Usage.CompletionTokens, 0, 0, parsed.Model
		}
	}
	return 0, 0, 0, 0, ""
}

// buildNativeLogData constructs a LogData from native proxy usage info.
func buildNativeLogData(ctx context.Context, providerName, modelName string, startTime time.Time, prompt, completion, cacheRead, cacheCreation int) callback.LogData {
	endTime := time.Now()
	// For cost calc: PromptTokens in TokenUsage = regular input (not total)
	// prompt here is already total (input + cache_read + cache_creation)
	regularInput := prompt - cacheRead - cacheCreation
	if regularInput < 0 {
		regularInput = 0
	}
	tokenUsage := pricing.TokenUsage{
		PromptTokens:             regularInput,
		CompletionTokens:         completion,
		CacheReadInputTokens:     cacheRead,
		CacheCreationInputTokens: cacheCreation,
	}
	promptCost, completionCost := pricing.Default().Cost(modelName, tokenUsage)
	data := callback.LogData{
		Model:                    modelName,
		Provider:                 providerName,
		StartTime:                startTime,
		EndTime:                  endTime,
		Latency:                  endTime.Sub(startTime),
		PromptTokens:             prompt,
		CompletionTokens:         completion,
		TotalTokens:              prompt + completion,
		CacheReadInputTokens:     cacheRead,
		CacheCreationInputTokens: cacheCreation,
		Cost:                     promptCost + completionCost,
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

// gzipReadCloser wraps a gzip.Reader and closes both the gzip reader and
// the original response body.
type gzipReadCloser struct {
	gz   *gzip.Reader
	orig io.ReadCloser
}

func (g *gzipReadCloser) Read(p []byte) (int, error) { return g.gz.Read(p) }
func (g *gzipReadCloser) Close() error {
	g.gz.Close()
	return g.orig.Close()
}

// readCloserOnly wraps an io.ReadCloser to hide any additional interfaces
// (like io.WriterTo). This prevents io.Copy from using the destination's
// ReadFrom optimization, which would bypass our tee buffer.
type readCloserOnly struct{ io.ReadCloser }

func (r *sseSpendReader) Close() error {
	err := r.src.Close()

	prompt, completion, cacheRead, cacheCreation, modelName := parseSSEUsage(r.providerName, r.buf.Bytes())
	if modelName == "" {
		modelName = r.requestModel
	}
	go r.callbacks.LogSuccess(buildNativeLogData(
		r.ctx, r.providerName, modelName, r.startTime,
		prompt, completion, cacheRead, cacheCreation,
	))
	return err
}

// parseSSEUsage scans SSE events for usage data.
// Anthropic: model in message_start, usage in message_delta.
func parseSSEUsage(providerName string, raw []byte) (prompt, completion, cacheRead, cacheCreation int, modelName string) {
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
						InputTokens              int `json:"input_tokens"`
						CacheReadInputTokens     int `json:"cache_read_input_tokens"`
						CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
					} `json:"usage"`
				} `json:"message"`
				Usage struct {
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
			}
			if json.Unmarshal(data, &event) != nil {
				continue
			}
			if event.Type == "message_start" {
				if event.Message.Model != "" {
					modelName = event.Message.Model
				}
				cacheRead = event.Message.Usage.CacheReadInputTokens
				cacheCreation = event.Message.Usage.CacheCreationInputTokens
				// PromptTokens = total (input + cache_read + cache_creation)
				prompt = event.Message.Usage.InputTokens + cacheRead + cacheCreation
			}
			// message_delta carries output_tokens in root usage
			if event.Type == "message_delta" && event.Usage.OutputTokens > 0 {
				completion = event.Usage.OutputTokens
			}

		case "openai":
			var event struct {
				Model string `json:"model"`
				Usage struct {
					PromptTokens     int `json:"prompt_tokens"`
					CompletionTokens int `json:"completion_tokens"`
				} `json:"usage"`
			}
			if json.Unmarshal(data, &event) != nil {
				continue
			}
			if event.Model != "" {
				modelName = event.Model
			}
			if event.Usage.PromptTokens > 0 || event.Usage.CompletionTokens > 0 {
				prompt = event.Usage.PromptTokens
				completion = event.Usage.CompletionTokens
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

// nativeRoundRobinCounter is used for round-robin selection among multiple upstream entries.
var nativeRoundRobinCounter uint64

// resolveNativeUpstream finds the base URL and API key for a native provider.
// For providers with multiple entries (e.g. multiple Anthropic OAuth tokens), it uses
// round-robin selection so rate limit attribution is spread correctly.
func (h *Handlers) resolveNativeUpstream(providerName string) (string, string) {
	type entry struct{ base, apiKey string }
	var entries []entry
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
			entries = append(entries, entry{base: base, apiKey: apiKey})
		}
	}
	if len(entries) == 0 {
		return "", ""
	}
	idx := atomic.AddUint64(&nativeRoundRobinCounter, 1) % uint64(len(entries))
	e := entries[idx]
	return e.base, e.apiKey
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
