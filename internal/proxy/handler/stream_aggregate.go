package handler

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

type streamAggregate struct {
	id        string
	model     string
	created   int64
	choices   map[int]*streamChoiceAggregate
	usage     model.Usage
	hasUsage  bool
	completed bool
}

type streamChoiceAggregate struct {
	content      strings.Builder
	refusal      strings.Builder
	toolCalls    map[int]*model.ToolCall
	finishReason *string
	sawContent   bool
}

func (a *streamAggregate) Add(chunk *model.StreamChunk) {
	if chunk == nil {
		return
	}
	if a.id == "" {
		a.id = chunk.ID
	}
	if a.model == "" {
		a.model = chunk.Model
	}
	if a.created == 0 {
		a.created = chunk.Created
	}
	if chunk.Usage != nil {
		a.hasUsage = true
	}
	mergeStreamUsage(&a.usage, chunk.Usage)
	for _, choice := range chunk.Choices {
		aggregate := a.choice(choice.Index)
		if choice.Delta.Content != nil {
			aggregate.sawContent = true
			aggregate.content.WriteString(*choice.Delta.Content)
		}
		if choice.Delta.Refusal != nil {
			aggregate.refusal.WriteString(*choice.Delta.Refusal)
		}
		for i, fragment := range choice.Delta.ToolCalls {
			index := i
			if fragment.Index != nil {
				index = *fragment.Index
			}
			if aggregate.toolCalls == nil {
				aggregate.toolCalls = make(map[int]*model.ToolCall)
			}
			call := aggregate.toolCalls[index]
			if call == nil {
				call = &model.ToolCall{}
				idx := index
				call.Index = &idx
				aggregate.toolCalls[index] = call
			}
			if fragment.ID != "" {
				call.ID = fragment.ID
			}
			if fragment.Type != "" {
				call.Type = fragment.Type
			}
			call.Function.Name += fragment.Function.Name
			call.Function.Arguments += fragment.Function.Arguments
		}
		if choice.FinishReason != nil {
			reason := *choice.FinishReason
			aggregate.finishReason = &reason
			a.completed = true
		}
	}
}

func (a *streamAggregate) choice(index int) *streamChoiceAggregate {
	if a.choices == nil {
		a.choices = make(map[int]*streamChoiceAggregate)
	}
	if a.choices[index] == nil {
		a.choices[index] = &streamChoiceAggregate{}
	}
	return a.choices[index]
}

func (a *streamAggregate) Complete() {
	a.completed = true
}

func (a *streamAggregate) CompleteWithFinishReason(reason string) {
	a.completed = true
	for _, choice := range a.choices {
		if choice.finishReason == nil {
			choice.finishReason = &reason
		}
	}
}

func (a *streamAggregate) HasToolCalls() bool {
	for _, choice := range a.choices {
		if len(choice.toolCalls) > 0 {
			return true
		}
	}
	return false
}

func (a *streamAggregate) Response(fallbackModel string) (*model.ModelResponse, error) {
	if !a.completed || len(a.choices) == 0 {
		return nil, errors.New("upstream stream ended before completion")
	}
	indexes := make([]int, 0, len(a.choices))
	for index := range a.choices {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	choices := make([]model.Choice, 0, len(indexes))
	for _, index := range indexes {
		choice, err := a.choices[index].Response(index)
		if err != nil {
			return nil, err
		}
		choices = append(choices, choice)
	}

	modelName := a.model
	if modelName == "" {
		modelName = fallbackModel
	}
	return &model.ModelResponse{
		ID:      a.id,
		Object:  "chat.completion",
		Created: a.created,
		Model:   modelName,
		Choices: choices,
		Usage:   a.usage,
	}, nil
}

func (a *streamAggregate) ResponseWithUsage(fallbackModel string) (*model.ModelResponse, error) {
	if !a.hasUsage {
		return nil, errors.New("upstream stream ended before usage")
	}
	return a.Response(fallbackModel)
}

func (a *streamChoiceAggregate) Response(index int) (model.Choice, error) {
	if a.finishReason == nil || strings.TrimSpace(*a.finishReason) == "" {
		return model.Choice{}, errors.New("upstream stream ended before finish reason")
	}

	message := &model.Message{Role: "assistant"}
	if a.sawContent {
		message.Content = a.content.String()
	}
	if a.refusal.Len() > 0 {
		refusal := a.refusal.String()
		message.Refusal = &refusal
	}
	if len(a.toolCalls) > 0 {
		indexes := make([]int, 0, len(a.toolCalls))
		for index := range a.toolCalls {
			indexes = append(indexes, index)
		}
		sort.Ints(indexes)
		message.ToolCalls = make([]model.ToolCall, 0, len(indexes))
		for _, index := range indexes {
			call := *a.toolCalls[index]
			call.Index = nil
			message.ToolCalls = append(message.ToolCalls, call)
		}
	}
	return model.Choice{
		Index:        index,
		Message:      message,
		FinishReason: a.finishReason,
	}, nil
}

type streamChunkTransform func([]byte) (*model.StreamChunk, bool, error)

func consumeCompletionStream(body io.Reader, transform streamChunkTransform, onEvent func(*model.StreamChunk, bool) error) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	sawFinishReason := false
	for scanner.Scan() {
		line := scanner.Bytes()
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		data := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
		if len(data) == 0 {
			continue
		}
		chunk, done, err := transform(data)
		if err != nil {
			return err
		}
		if chunk != nil {
			for _, choice := range chunk.Choices {
				if choice.FinishReason != nil && strings.TrimSpace(*choice.FinishReason) != "" {
					sawFinishReason = true
				}
			}
		}
		if (chunk != nil || done) && onEvent != nil {
			if err := onEvent(chunk, done); err != nil {
				return err
			}
		}
		if done {
			return nil
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read upstream stream: %w", err)
	}
	if sawFinishReason {
		return nil
	}
	return errors.New("upstream stream ended before completion")
}
