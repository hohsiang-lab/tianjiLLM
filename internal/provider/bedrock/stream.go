package bedrock

import (
	"encoding/json"
	"strings"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// StreamEvent represents a Bedrock Converse stream event.
type StreamEvent struct {
	ContentBlockStart *struct {
		ContentBlockIndex int `json:"contentBlockIndex"`
		Start             struct {
			ToolUse *struct {
				ToolUseID string `json:"toolUseId"`
				Name      string `json:"name"`
			} `json:"toolUse,omitempty"`
		} `json:"start"`
	} `json:"contentBlockStart,omitempty"`

	ContentBlockDelta *struct {
		ContentBlockIndex int `json:"contentBlockIndex"`
		Delta             struct {
			Text    string `json:"text,omitempty"`
			ToolUse *struct {
				Input string `json:"input"`
			} `json:"toolUse,omitempty"`
		} `json:"delta"`
	} `json:"contentBlockDelta,omitempty"`

	ContentBlockStop *struct {
		ContentBlockIndex int `json:"contentBlockIndex"`
	} `json:"contentBlockStop,omitempty"`

	MessageStart *struct {
		Role string `json:"role"`
	} `json:"messageStart,omitempty"`

	MessageStop *struct {
		StopReason string `json:"stopReason"`
	} `json:"messageStop,omitempty"`

	Metadata *struct {
		Usage converseUsage `json:"usage"`
	} `json:"metadata,omitempty"`
}

// ParseStreamEvent parses a Bedrock Converse stream event.
func ParseStreamEvent(data []byte) (*model.StreamChunk, bool, error) {
	data = []byte(strings.TrimSpace(string(data)))

	var event StreamEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, false, err
	}

	switch {
	case event.MessageStart != nil:
		role := event.MessageStart.Role
		return &model.StreamChunk{
			Object: "chat.completion.chunk",
			Choices: []model.StreamChoice{
				{
					Index: 0,
					Delta: model.Delta{Role: &role},
				},
			},
		}, false, nil

	case event.ContentBlockDelta != nil:
		delta := event.ContentBlockDelta.Delta
		if delta.Text != "" {
			return &model.StreamChunk{
				Object: "chat.completion.chunk",
				Choices: []model.StreamChoice{
					{
						Index: 0,
						Delta: model.Delta{Content: &delta.Text},
					},
				},
			}, false, nil
		}
		if delta.ToolUse != nil {
			idx := event.ContentBlockDelta.ContentBlockIndex
			return &model.StreamChunk{
				Object: "chat.completion.chunk",
				Choices: []model.StreamChoice{
					{
						Index: 0,
						Delta: model.Delta{
							ToolCalls: []model.ToolCall{
								{
									Function: model.ToolCallFunction{
										Arguments: delta.ToolUse.Input,
									},
									Index: &idx,
								},
							},
						},
					},
				},
			}, false, nil
		}

	case event.ContentBlockStart != nil:
		if event.ContentBlockStart.Start.ToolUse != nil {
			tu := event.ContentBlockStart.Start.ToolUse
			idx := event.ContentBlockStart.ContentBlockIndex
			return &model.StreamChunk{
				Object: "chat.completion.chunk",
				Choices: []model.StreamChoice{
					{
						Index: 0,
						Delta: model.Delta{
							ToolCalls: []model.ToolCall{
								{
									ID:   tu.ToolUseID,
									Type: "function",
									Function: model.ToolCallFunction{
										Name: tu.Name,
									},
									Index: &idx,
								},
							},
						},
					},
				},
			}, false, nil
		}

	case event.MessageStop != nil:
		finishReason := mapStopReason(event.MessageStop.StopReason)
		return &model.StreamChunk{
			Object: "chat.completion.chunk",
			Choices: []model.StreamChoice{
				{
					Index:        0,
					Delta:        model.Delta{},
					FinishReason: &finishReason,
				},
			},
		}, true, nil

	case event.Metadata != nil:
		return &model.StreamChunk{
			Object: "chat.completion.chunk",
			Usage: &model.Usage{
				PromptTokens:     event.Metadata.Usage.InputTokens,
				CompletionTokens: event.Metadata.Usage.OutputTokens,
				TotalTokens:      event.Metadata.Usage.TotalTokens,
			},
		}, false, nil
	}

	return nil, false, nil
}
