package guardrail

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// ContentFilter is a built-in content filter using regex patterns.
type ContentFilter struct {
	categories map[string]*regexp.Regexp
	threshold  int // 0=off, 1=low, 2=medium, 3=high
}

// NewContentFilter creates a content filter with default patterns.
func NewContentFilter(threshold int) *ContentFilter {
	return &ContentFilter{
		categories: map[string]*regexp.Regexp{
			"violence":  regexp.MustCompile(`(?i)\b(kill|murder|assault|weapon|bomb|attack|shoot|stab)\b`),
			"hate":      regexp.MustCompile(`(?i)\b(slur|racist|bigot|discriminat|supremac)\b`),
			"self_harm": regexp.MustCompile(`(?i)\b(suicide|self.harm|cut\s+my|end\s+my\s+life)\b`),
			"sexual":    regexp.MustCompile(`(?i)\b(explicit|pornograph|sexual\s+content)\b`),
		},
		threshold: threshold,
	}
}

func (c *ContentFilter) Name() string { return "content_filter" }

func (c *ContentFilter) SupportedHooks() []Hook { return []Hook{HookPreCall, HookPostCall} }

func (c *ContentFilter) Run(ctx context.Context, hook Hook, req *model.ChatCompletionRequest, resp *model.ModelResponse) (Result, error) {
	if c.threshold == 0 {
		return Result{Passed: true}, nil
	}

	content := extractContent(hook, req, resp)

	if content == "" {
		return Result{Passed: true}, nil
	}

	var flagged []string
	for category, pattern := range c.categories {
		if pattern.MatchString(content) {
			flagged = append(flagged, category)
		}
	}

	if len(flagged) > 0 {
		return Result{
			Passed:  false,
			Message: fmt.Sprintf("Content flagged: %s", strings.Join(flagged, ", ")),
		}, nil
	}

	return Result{Passed: true}, nil
}
