package hook

import (
	"context"
	"fmt"
	"strings"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// BannedKeywords rejects requests containing banned keywords in messages.
type BannedKeywords struct {
	keywords []string
}

func init() {
	RegisterConstructor("banned_keywords", func(config map[string]any) (Hook, error) {
		kw, _ := config["keywords"].([]any)
		keywords := make([]string, 0, len(kw))
		for _, v := range kw {
			if s, ok := v.(string); ok {
				keywords = append(keywords, strings.ToLower(s))
			}
		}
		if len(keywords) == 0 {
			return nil, fmt.Errorf("banned_keywords: no keywords configured")
		}
		return &BannedKeywords{keywords: keywords}, nil
	})
}

func (b *BannedKeywords) Name() string { return "banned_keywords" }

func (b *BannedKeywords) PreCall(_ context.Context, req *model.ChatCompletionRequest) error {
	for _, msg := range req.Messages {
		var content string
		switch v := msg.Content.(type) {
		case string:
			content = v
		default:
			continue
		}
		lower := strings.ToLower(content)
		for _, kw := range b.keywords {
			if strings.Contains(lower, kw) {
				return fmt.Errorf("request contains banned keyword")
			}
		}
	}
	return nil
}

func (b *BannedKeywords) PostCall(_ context.Context, _ *model.ChatCompletionRequest, _ *model.ModelResponse) error {
	return nil
}
