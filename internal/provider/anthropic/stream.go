package anthropic

import (
	"encoding/json"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// StreamEvent represents an Anthropic SSE event.
type StreamEvent struct {
	Type         string          `json:"type"`
	Message      json.RawMessage `json:"message,omitempty"`
	Index        int             `json:"index,omitempty"`
	ContentBlock json.RawMessage `json:"content_block,omitempty"`
	Delta        json.RawMessage `json:"delta,omitempty"`
	Usage        *anthropicUsage `json:"usage,omitempty"`
}

// ParseStreamEvent parses an Anthropic SSE data line into an OpenAI-compatible StreamChunk.
// Returns (chunk, isDone, error).
func ParseStreamEvent(data []byte) (*model.StreamChunk, bool, error) {
	var event StreamEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, false, err
	}

	switch event.Type {
	case "message_start":
		return handleMessageStart(event)

	case "content_block_start":
		return handleContentBlockStart(event)

	case "content_block_delta":
		return handleContentBlockDelta(event)

	case "content_block_stop":
		return nil, false, nil

	case "message_delta":
		return handleMessageDelta(event)

	case "message_stop":
		return nil, true, nil

	case "error":
		return nil, true, nil

	default:
		return nil, false, nil
	}
}

func handleMessageStart(event StreamEvent) (*model.StreamChunk, bool, error) {
	var msg struct {
		ID    string `json:"id"`
		Model string `json:"model"`
	}
	if event.Message != nil {
		_ = json.Unmarshal(event.Message, &msg)
	}

	role := "assistant"
	return &model.StreamChunk{
		ID:     msg.ID,
		Object: "chat.completion.chunk",
		Model:  msg.Model,
		Choices: []model.StreamChoice{
			{
				Index: 0,
				Delta: model.Delta{
					Role: &role,
				},
			},
		},
	}, false, nil
}

func handleContentBlockStart(event StreamEvent) (*model.StreamChunk, bool, error) {
	var block struct {
		Type string `json:"type"`
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if event.ContentBlock != nil {
		_ = json.Unmarshal(event.ContentBlock, &block)
	}

	if block.Type == "tool_use" {
		idx := event.Index
		return &model.StreamChunk{
			Object: "chat.completion.chunk",
			Choices: []model.StreamChoice{
				{
					Index: 0,
					Delta: model.Delta{
						ToolCalls: []model.ToolCall{
							{
								ID:   block.ID,
								Type: "function",
								Function: model.ToolCallFunction{
									Name:      block.Name,
									Arguments: "",
								},
								Index: &idx,
							},
						},
					},
				},
			},
		}, false, nil
	}

	return nil, false, nil
}

func handleContentBlockDelta(event StreamEvent) (*model.StreamChunk, bool, error) {
	var delta struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		PartialJSON string `json:"partial_json"`
	}
	if event.Delta != nil {
		_ = json.Unmarshal(event.Delta, &delta)
	}

	switch delta.Type {
	case "text_delta":
		return &model.StreamChunk{
			Object: "chat.completion.chunk",
			Choices: []model.StreamChoice{
				{
					Index: 0,
					Delta: model.Delta{
						Content: &delta.Text,
					},
				},
			},
		}, false, nil

	case "input_json_delta":
		idx := event.Index
		return &model.StreamChunk{
			Object: "chat.completion.chunk",
			Choices: []model.StreamChoice{
				{
					Index: 0,
					Delta: model.Delta{
						ToolCalls: []model.ToolCall{
							{
								Function: model.ToolCallFunction{
									Arguments: delta.PartialJSON,
								},
								Index: &idx,
							},
						},
					},
				},
			},
		}, false, nil
	}

	return nil, false, nil
}

func handleMessageDelta(event StreamEvent) (*model.StreamChunk, bool, error) {
	var delta struct {
		StopReason string `json:"stop_reason"`
	}
	if event.Delta != nil {
		_ = json.Unmarshal(event.Delta, &delta)
	}

	finishReason := mapStopReason(delta.StopReason)

	chunk := &model.StreamChunk{
		Object: "chat.completion.chunk",
		Choices: []model.StreamChoice{
			{
				Index:        0,
				Delta:        model.Delta{},
				FinishReason: &finishReason,
			},
		},
	}

	if event.Usage != nil {
		chunk.Usage = &model.Usage{
			PromptTokens:     event.Usage.InputTokens,
			CompletionTokens: event.Usage.OutputTokens,
			TotalTokens:      event.Usage.InputTokens + event.Usage.OutputTokens,
		}
	}

	return chunk, false, nil
}
