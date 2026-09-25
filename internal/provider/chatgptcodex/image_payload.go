package chatgptcodex

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

const (
	defaultImageGenerationResponsesModel = "gpt-5.5"
	imageGenerationInstructions          = "You are an image generation assistant."
)

func PrepareImageGenerationPayload(req *model.ImageGenerationRequest) (PreparedResponsesPayload, error) {
	payload, err := BuildImageGenerationPayload(req)
	if err != nil {
		return PreparedResponsesPayload{}, err
	}
	return PrepareResponsesPayload(payload, true)
}

func BuildImageGenerationPayload(req *model.ImageGenerationRequest) (map[string]any, error) {
	if err := validateSupportedImageGeneration(req); err != nil {
		return nil, err
	}
	tool := imageGenerationTool(req.Model, req.Size, req.Quality, req.OutputFormat, req.Background, req.OutputCompression)
	return imageGenerationResponsesPayload([]map[string]any{{
		"type": "input_text",
		"text": req.Prompt,
	}}, tool), nil
}

func BuildImageEditPayload(req *model.ImageEditRequest) (map[string]any, error) {
	if err := validateSupportedImageEdit(req); err != nil {
		return nil, err
	}
	tool := imageGenerationTool(req.Model, req.Size, req.Quality, req.OutputFormat, req.Background, req.OutputCompression)
	content := []map[string]any{{
		"type": "input_text",
		"text": req.Prompt,
	}}
	for _, image := range req.Images {
		content = append(content, map[string]any{
			"type":      "input_image",
			"image_url": imageEditDataURL(image),
		})
	}
	return imageGenerationResponsesPayload(content, tool), nil
}

func imageGenerationTool(modelName string, size, quality, outputFormat, background *string, outputCompression *int) map[string]any {
	tool := map[string]any{
		"type":  "image_generation",
		"model": normalizeModel(modelName),
	}
	if size != nil && strings.TrimSpace(*size) != "" {
		tool["size"] = strings.TrimSpace(*size)
	}
	if quality != nil && strings.TrimSpace(*quality) != "" {
		tool["quality"] = strings.TrimSpace(*quality)
	}
	if outputFormat != nil && strings.TrimSpace(*outputFormat) != "" {
		tool["output_format"] = strings.TrimSpace(*outputFormat)
	}
	if background != nil && strings.TrimSpace(*background) != "" {
		tool["background"] = strings.TrimSpace(*background)
	}
	if outputCompression != nil {
		tool["output_compression"] = *outputCompression
	}
	return tool
}

func imageGenerationResponsesPayload(content []map[string]any, tool map[string]any) map[string]any {
	return map[string]any{
		"model": defaultImageGenerationResponsesModel,
		"input": []map[string]any{{
			"type":    "message",
			"role":    "user",
			"content": content,
		}},
		"instructions": imageGenerationInstructions,
		"tools":        []map[string]any{tool},
		"tool_choice":  map[string]any{"type": "image_generation"},
		"stream":       true,
		"store":        false,
	}
}

func validateSupportedImageGeneration(req *model.ImageGenerationRequest) error {
	if len(req.ExtraParams) > 0 {
		return unsupportedPayloadError("unknown extra parameters")
	}
	if req.ResponseFormat != nil {
		value := strings.TrimSpace(*req.ResponseFormat)
		if value != "" && value != "b64_json" {
			return unsupportedPayloadError("response_format")
		}
	}
	if req.Style != nil && strings.TrimSpace(*req.Style) != "" {
		return unsupportedPayloadError("style")
	}
	if req.Moderation != nil && strings.TrimSpace(*req.Moderation) != "" {
		return unsupportedPayloadError("moderation")
	}
	if req.OutputCompression != nil && (*req.OutputCompression < 0 || *req.OutputCompression > 100) {
		return unsupportedPayloadError("output_compression")
	}
	return nil
}

func validateSupportedImageEdit(req *model.ImageEditRequest) error {
	if strings.TrimSpace(req.Prompt) == "" {
		return unsupportedPayloadError("prompt")
	}
	if len(req.Images) == 0 {
		return unsupportedPayloadError("image")
	}
	if len(req.ExtraParams) > 0 {
		return unsupportedPayloadError("unknown extra parameters")
	}
	if req.ResponseFormat != nil {
		value := strings.TrimSpace(*req.ResponseFormat)
		if value != "" && value != "b64_json" {
			return unsupportedPayloadError("response_format")
		}
	}
	if req.Mask != nil {
		return unsupportedPayloadError("mask")
	}
	if req.OutputCompression != nil && (*req.OutputCompression < 0 || *req.OutputCompression > 100) {
		return unsupportedPayloadError("output_compression")
	}
	return nil
}

func imageEditDataURL(file model.ImageEditFile) string {
	contentType := strings.TrimSpace(file.ContentType)
	if contentType == "" {
		contentType = http.DetectContentType(file.Data)
	}
	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(file.Data)
}

func imageGenerationPayloadSummary(payload map[string]any) string {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Sprintf("marshal_error=%q", err.Error())
	}
	var summary struct {
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
		Store  bool   `json:"store"`
		Input  []struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"input"`
		Tools []map[string]any `json:"tools"`
	}
	if err := json.Unmarshal(raw, &summary); err != nil {
		return fmt.Sprintf("parse_error=%q", err.Error())
	}
	promptLen := 0
	if len(summary.Input) > 0 && len(summary.Input[0].Content) > 0 {
		promptLen = len(summary.Input[0].Content[0].Text)
	}
	tool := map[string]any{}
	if len(summary.Tools) > 0 {
		tool = summary.Tools[0]
	}
	return fmt.Sprintf("model=%s stream=%t store=%t prompt_len=%d tool=%v", summary.Model, summary.Stream, summary.Store, promptLen, tool)
}
