package handler

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/pricing"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/anthropic"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/middleware"
	"github.com/praxisllmlab/tianjiLLM/internal/security/redact"
)

// clearInboundProviderAuthHeaders removes client-supplied credentials before
// injecting the selected native provider credential.
func clearInboundProviderAuthHeaders(req *http.Request) {
	if req == nil {
		return
	}
	for _, header := range []string{"Authorization", "api-key", "x-api-key", "x-goog-api-key", "Proxy-Authorization"} {
		req.Header.Del(header)
	}
}

// nativeProxy creates a reverse proxy to a specific provider's base URL.
func (h *Handlers) nativeProxy(w http.ResponseWriter, r *http.Request, providerName string) {
	requestModel := extractRequestModel(r)

	upstreams := h.resolveAllNativeUpstreams(r.Context(), providerName)
	upstream, throttleErr := h.selectUpstreamWithThrottle(r.Context(), providerName, upstreams, requestModel)
	if throttleErr != nil {
		if ate, ok := throttleErr.(*allTokensThrottledError); ok {
			retryAfter := int(time.Until(ate.resetAt).Seconds())
			if retryAfter < 1 {
				retryAfter = 60
			}
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			writeJSON(w, http.StatusTooManyRequests, model.ErrorResponse{
				Error: model.ErrorDetail{
					Message: "all OAuth tokens throttled",
					Type:    "rate_limit_error",
				},
			})
			return
		}
		log.Printf("ERROR upstream selection failed for %s: %v", providerName, throttleErr)
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "upstream selection failed", Type: "internal_error"},
		})
		return
	}
	if upstream.BaseURL == "" {
		writeJSON(w, http.StatusNotImplemented, model.ErrorResponse{
			Error: model.ErrorDetail{Message: providerName + " not configured", Type: "not_supported"},
		})
		return
	}
	baseURL, apiKey := upstream.BaseURL, upstream.APIKey

	// Inject upstream token key into context for spend/error logging.
	// buildBaseLogData() will pick this up alongside other context values.
	if apiKey != "" {
		sum := sha256.Sum256([]byte(apiKey))
		r = r.WithContext(context.WithValue(r.Context(), middleware.ContextKeyUpstreamToken, hex.EncodeToString(sum[:6])))
	}

	target, err := url.Parse(baseURL)
	if err != nil {
		log.Printf("native proxy: invalid upstream URL for %s %q: %v", providerName, baseURL, err)
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid upstream URL", Type: "internal_error"},
		})
		return
	}

	startTime := time.Now()
	ctx := r.Context()

	proxy := &httputil.ReverseProxy{
		Rewrite: func(proxyReq *httputil.ProxyRequest) {
			req := proxyReq.Out
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host

			// Remove client's auth headers before injecting provider credentials.
			clearInboundProviderAuthHeaders(req)

			switch providerName {
			case "anthropic":
				if anthropic.IsOAuthToken(apiKey) {
					anthropic.SetOAuthHeaders(req, apiKey)
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
					body, readErr := io.ReadAll(resp.Body)
					if readErr != nil {
						log.Printf("native proxy: failed to read error body from %s (status %d): %v", providerName, resp.StatusCode, readErr)
						// Use empty body on read failure to avoid storing truncated content.
						body = nil
					}
					resp.Body = io.NopCloser(bytes.NewReader(body))
					errMsg := fmt.Sprintf("upstream error: status %d", resp.StatusCode)
					if len(body) > 0 {
						errMsg = string(body)
					}
					params := errorLogParamsFromContext(ctx)
					params.Model = requestModel
					params.Provider = providerName
					params.StatusCode = int32(resp.StatusCode)
					params.ErrorType = "upstream_error"
					params.ErrorMessage = redact.String(errMsg)
					go func() {
						insertCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
						defer cancel()
						if err := h.DB.InsertErrorLog(insertCtx, params); err != nil {
							log.Printf("ERROR InsertErrorLog failed for request %s: %v", params.RequestID, err)
						}
					}()
				}
				// FR-019: parse rate limit headers and update usage stores on non-200 responses (e.g. 429).
				// Must NOT early return before this block — 429 carries the most important rate limit signal.
				if providerName == "anthropic" {
					h.recordRateLimitUsage(apiKey, resp.Header)
				}
				return nil
			}

			if providerName == "anthropic" {
				h.recordRateLimitUsage(apiKey, resp.Header)
			}

			if h.Callbacks == nil {
				return nil
			}

			// Transparently decompress gzip responses (like LiteLLM/httpx).
			// Go's ReverseProxy passes through compressed bytes as-is, but we
			// need plaintext to parse usage tokens and for consistent client behavior.
			if resp.Header.Get("Content-Encoding") == "gzip" {
				gr, gzErr := gzip.NewReader(resp.Body)
				if gzErr != nil {
					return fmt.Errorf("native proxy: gzip decode failed (%s): %w", providerName, gzErr)
				}
				resp.Body = &gzipReadCloser{gz: gr, orig: resp.Body}
				resp.Header.Del("Content-Encoding")
				resp.Header.Del("Content-Length") // length changes after decompression
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
				return fmt.Errorf("native proxy: failed to read response body (%s): %w", providerName, err)
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
		log.Printf("native proxy: failed to read request body for model extraction: %v", err)
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
	data := buildBaseLogData(ctx, startTime)
	data.Model = modelName
	data.Provider = providerName
	data.EndTime = endTime
	data.Latency = endTime.Sub(startTime)
	data.PromptTokens = prompt
	data.CompletionTokens = completion
	data.TotalTokens = prompt + completion
	data.CacheReadInputTokens = cacheRead
	data.CacheCreationInputTokens = cacheCreation
	data.Cost = promptCost + completionCost
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
	if err != nil {
		log.Printf("native proxy: SSE stream closed with error (%s), skipping spend log: %v", r.providerName, err)
		return err
	}

	prompt, completion, cacheRead, cacheCreation, modelName := parseSSEUsage(r.providerName, r.buf.Bytes())
	if modelName == "" {
		modelName = r.requestModel
	}
	go r.callbacks.LogSuccess(buildNativeLogData(
		r.ctx, r.providerName, modelName, r.startTime,
		prompt, completion, cacheRead, cacheCreation,
	))
	return nil
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

// recordRateLimitUsage parses rate limit headers, updates stores, sends Discord alerts, and persists org ID.
// Handles both OAuth tokens (unified headers) and legacy API keys (per-type headers).
// Uses context.Background() for the DB goroutine since r.Context() is cancelled after the handler returns.
func (h *Handlers) recordRateLimitUsage(apiKey string, header http.Header) {
	tokenKey := callback.RateLimitCacheKey(apiKey)
	rlState := callback.ParseAnthropicOAuthRateLimitHeaders(header, tokenKey)
	if h.RateLimitStore != nil {
		h.RateLimitStore.Set(tokenKey, rlState)
	}
	if h.DiscordAlerter != nil {
		h.DiscordAlerter.CheckAndAlert(rlState)
	}
	if rlState.OrganizationID != "" && h.DB != nil {
		orgID := rlState.OrganizationID
		tk := tokenKey
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := h.DB.UpsertOAuthTokenMetadata(ctx, db.UpsertOAuthTokenMetadataParams{TokenKey: tk, OrgID: orgID}); err != nil {
				log.Printf("ERROR upsert oauth token metadata for %s: %v", tk, err)
			}
		}()
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
	h.handleImagesEdit(w, r)
}

// ImageVariation handles POST /v1/images/variations.
func (h *Handlers) ImageVariation(w http.ResponseWriter, r *http.Request) {
	h.openAIEndpointProxy(w, r)
}
