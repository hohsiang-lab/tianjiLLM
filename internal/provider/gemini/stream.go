package gemini

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// ParseStreamChunk parses a Gemini streaming response chunk.
// Gemini uses SSE format with JSON objects per line.
func ParseStreamChunk(data []byte) (*model.StreamChunk, bool, error) {
	data = []byte(strings.TrimSpace(string(data)))

	if len(data) == 0 {
		return nil, false, nil
	}

	var resp geminiResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, false, err
	}

	if len(resp.Candidates) == 0 {
		return nil, false, nil
	}

	candidate := resp.Candidates[0]

	var delta model.Delta

	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			delta.Content = &part.Text
		}
		if part.FunctionCall != nil {
			args, _ := json.Marshal(part.FunctionCall.Args)
			idx := 0
			delta.ToolCalls = append(delta.ToolCalls, model.ToolCall{
				ID:   fmt.Sprintf("call_%d", idx),
				Type: "function",
				Function: model.ToolCallFunction{
					Name:      part.FunctionCall.Name,
					Arguments: string(args),
				},
				Index: &idx,
			})
		}
	}

	var finishReason *string
	if candidate.FinishReason != "" {
		fr := mapFinishReason(candidate.FinishReason)
		finishReason = &fr
	}

	chunk := &model.StreamChunk{
		Object: "chat.completion.chunk",
		Choices: []model.StreamChoice{
			{
				Index:        0,
				Delta:        delta,
				FinishReason: finishReason,
			},
		},
	}

	if resp.UsageMetadata != nil {
		chunk.Usage = &model.Usage{
			PromptTokens:     resp.UsageMetadata.PromptTokenCount,
			CompletionTokens: resp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      resp.UsageMetadata.TotalTokenCount,
		}
	}

	isDone := finishReason != nil && *finishReason == "stop"
	return chunk, isDone, nil
}
