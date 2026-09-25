package chatgptcodex

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

type imageGenerationEvent struct {
	Type     string                    `json:"type"`
	Item     imageGenerationOutputItem `json:"item"`
	Response struct {
		Output []imageGenerationOutputItem `json:"output"`
		Error  *codexError                 `json:"error"`
		Usage  codexUsage                  `json:"usage"`
	} `json:"response"`
	Usage   codexUsage  `json:"usage"`
	Error   *codexError `json:"error"`
	Message string      `json:"message"`
}

type imageGenerationOutputItem struct {
	Type          string `json:"type"`
	Result        string `json:"result"`
	RevisedPrompt string `json:"revised_prompt"`
}

type ImageGenerationStreamResult struct {
	Images []model.ImageData
	Usage  model.Usage
}

func TransformImageGenerationStream(body io.Reader) (ImageGenerationStreamResult, error) {
	var result ImageGenerationStreamResult
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64*1024), 64*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) < len("data: ") || string(line[:len("data: ")]) != "data: " {
			continue
		}
		data := line[len("data: "):]
		if string(data) == "[DONE]" {
			continue
		}

		var event imageGenerationEvent
		if err := json.Unmarshal(data, &event); err != nil {
			return ImageGenerationStreamResult{}, fmt.Errorf("parse ChatGPT Codex image generation event: %w", err)
		}
		logImageGenerationEvent(event)
		if event.Type == "response.failed" || event.Type == "error" {
			return ImageGenerationStreamResult{}, imageGenerationFailure(event)
		}
		if event.Type == "response.output_item.done" {
			if image, ok := imageDataFromCodexOutputItem(event.Item); ok {
				result.Images = append(result.Images, image)
			}
			continue
		}
		if event.Type == "response.completed" {
			result.Usage = imageGenerationUsageFromEvent(event)
		}
		if event.Type == "response.completed" && len(result.Images) == 0 {
			for _, item := range event.Response.Output {
				if image, ok := imageDataFromCodexOutputItem(item); ok {
					result.Images = append(result.Images, image)
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return ImageGenerationStreamResult{}, fmt.Errorf("read ChatGPT Codex image generation stream: %w", err)
	}
	if len(result.Images) == 0 {
		return ImageGenerationStreamResult{}, fmt.Errorf("ChatGPT Codex image generation response contained no images")
	}

	return result, nil
}

func imageDataFromCodexOutputItem(item imageGenerationOutputItem) (model.ImageData, bool) {
	if item.Type != "image_generation_call" || item.Result == "" {
		return model.ImageData{}, false
	}
	return model.ImageData{
		B64JSON:       item.Result,
		RevisedPrompt: item.RevisedPrompt,
	}, true
}

func imageGenerationUsageFromEvent(event imageGenerationEvent) model.Usage {
	usage := event.Response.Usage
	if usage.TotalTokens == 0 && event.Usage.TotalTokens > 0 {
		usage = event.Usage
	}
	totalTokens := usage.TotalTokens
	if totalTokens == 0 {
		totalTokens = usage.InputTokens + usage.OutputTokens
	}
	return model.Usage{
		PromptTokens:     usage.InputTokens,
		CompletionTokens: usage.OutputTokens,
		TotalTokens:      totalTokens,
	}
}

func logImageGenerationEvent(event imageGenerationEvent) {
	if os.Getenv("TIANJI_DEBUG_CODEX_IMAGE") != "1" {
		return
	}
	usage := event.Response.Usage
	if usage.TotalTokens == 0 && event.Usage.TotalTokens > 0 {
		usage = event.Usage
	}
	log.Printf(
		"debug: codex-image event type=%s item_type=%s result_len=%d revised_prompt_len=%d response_output_count=%d usage_input=%d usage_output=%d usage_total=%d error_set=%t message_len=%d",
		event.Type,
		event.Item.Type,
		len(event.Item.Result),
		len(event.Item.RevisedPrompt),
		len(event.Response.Output),
		usage.InputTokens,
		usage.OutputTokens,
		usage.TotalTokens,
		event.Error != nil || event.Response.Error != nil,
		len(event.Message),
	)
}

func imageGenerationFailure(event imageGenerationEvent) error {
	errDetail := event.Error
	if errDetail == nil {
		errDetail = event.Response.Error
	}
	message := event.Message
	errType := "api_error"
	code := ""
	if errDetail != nil {
		if errDetail.Message != "" {
			message = errDetail.Message
		}
		if errDetail.Type != "" {
			errType = errDetail.Type
		}
		code = errDetail.Code
	}
	if message == "" {
		message = "ChatGPT Codex image generation failed"
	}
	return newCodexTianjiError(http.StatusBadGateway, message, errType, code)
}
