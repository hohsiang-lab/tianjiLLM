package prompt

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// LangfuseSource fetches prompts from Langfuse API.
type LangfuseSource struct {
	publicKey string
	secretKey string
	baseURL   string

	mu    sync.RWMutex
	cache map[string]cachedPrompt
	ttl   time.Duration
}

type cachedPrompt struct {
	prompt    *ResolvedPrompt
	expiresAt time.Time
}

func init() {
	Register("langfuse", newLangfuseSource)
}

func newLangfuseSource(cfg map[string]any) (PromptSource, error) {
	publicKey, _ := cfg["public_key"].(string)
	secretKey, _ := cfg["secret_key"].(string)
	baseURL, _ := cfg["base_url"].(string)
	if baseURL == "" {
		baseURL = "https://cloud.langfuse.com"
	}

	ttl := 5 * time.Minute
	if t, ok := cfg["cache_ttl"].(int); ok && t > 0 {
		ttl = time.Duration(t) * time.Second
	}

	return &LangfuseSource{
		publicKey: publicKey,
		secretKey: secretKey,
		baseURL:   baseURL,
		cache:     make(map[string]cachedPrompt),
		ttl:       ttl,
	}, nil
}

func (l *LangfuseSource) Name() string { return "langfuse" }

func (l *LangfuseSource) GetPrompt(ctx context.Context, promptID string, opts PromptOptions) (*ResolvedPrompt, error) {
	cacheKey := l.cacheKey(promptID, opts)

	l.mu.RLock()
	if cached, ok := l.cache[cacheKey]; ok && time.Now().Before(cached.expiresAt) {
		l.mu.RUnlock()
		// Apply variables to a copy
		return l.applyVariables(cached.prompt, opts.Variables), nil
	}
	l.mu.RUnlock()

	// Fetch from Langfuse
	url := fmt.Sprintf("%s/api/public/v2/prompts/%s", l.baseURL, promptID)
	if opts.Version != nil {
		url += fmt.Sprintf("?version=%d", *opts.Version)
	} else if opts.Label != nil {
		url += fmt.Sprintf("?label=%s", *opts.Label)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(l.publicKey, l.secretKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("langfuse: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("langfuse: status %d for prompt %q", resp.StatusCode, promptID)
	}

	var result struct {
		Prompt any    `json:"prompt"`
		Type   string `json:"type"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("langfuse: decode: %w", err)
	}

	resolved, err := l.parsePrompt(result.Prompt, result.Type)
	if err != nil {
		return nil, err
	}

	// Cache the result
	l.mu.Lock()
	l.cache[cacheKey] = cachedPrompt{
		prompt:    resolved,
		expiresAt: time.Now().Add(l.ttl),
	}
	l.mu.Unlock()

	return l.applyVariables(resolved, opts.Variables), nil
}

func (l *LangfuseSource) cacheKey(promptID string, opts PromptOptions) string {
	key := promptID
	if opts.Version != nil {
		key += fmt.Sprintf(":v%d", *opts.Version)
	}
	if opts.Label != nil {
		key += ":" + *opts.Label
	}
	return key
}

func (l *LangfuseSource) parsePrompt(prompt any, promptType string) (*ResolvedPrompt, error) {
	resolved := &ResolvedPrompt{
		Metadata: make(map[string]string),
	}

	switch promptType {
	case "chat":
		// Chat prompts: array of {role, content}
		data, _ := json.Marshal(prompt)
		var msgs []model.Message
		if err := json.Unmarshal(data, &msgs); err != nil {
			return nil, fmt.Errorf("langfuse: parse chat prompt: %w", err)
		}
		resolved.Messages = msgs

	case "text":
		// Text prompts: single string → user message
		if s, ok := prompt.(string); ok {
			resolved.Messages = []model.Message{
				{Role: "user", Content: s},
			}
		}

	default:
		// Try to parse as chat messages
		data, _ := json.Marshal(prompt)
		var msgs []model.Message
		if err := json.Unmarshal(data, &msgs); err == nil && len(msgs) > 0 {
			resolved.Messages = msgs
		} else if s, ok := prompt.(string); ok {
			resolved.Messages = []model.Message{
				{Role: "user", Content: s},
			}
		}
	}

	return resolved, nil
}

func (l *LangfuseSource) applyVariables(p *ResolvedPrompt, vars map[string]string) *ResolvedPrompt {
	if len(vars) == 0 {
		return p
	}

	result := &ResolvedPrompt{
		Messages: make([]model.Message, len(p.Messages)),
		Metadata: p.Metadata,
	}
	for i, msg := range p.Messages {
		content := msg.Content
		if s, ok := content.(string); ok {
			for k, v := range vars {
				s = strings.ReplaceAll(s, "{{"+k+"}}", v)
			}
			content = s
		}
		result.Messages[i] = model.Message{
			Role:    msg.Role,
			Content: content,
		}
	}
	return result
}
