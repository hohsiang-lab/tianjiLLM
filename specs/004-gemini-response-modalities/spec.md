# Feature Specification: Gemini Response Modalities (Image Output)

**Feature Branch**: `004-gemini-response-modalities`
**Created**: 2026-02-26
**Status**: Draft
**Related Issue**: #17

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Text-to-Image Generation (Priority: P1)

A developer building the persona-factory pipeline wants to call `gemini-2-flash-exp` (or `gemini-3-pro-image-preview`) through Tianji to generate images. They send a chat completion request with `modalities: ["text", "image"]` in the request body, and expect to receive the generated image embedded in the assistant message in OpenAI-compatible format (`message.content[].image_url`), so their downstream code can process images using standard OpenAI SDK conventions.

**Why this priority**: This is the core use case that unblocks persona-factory. Without image output support, the pipeline cannot use Tianji as a transparent proxy for Gemini image generation.

**Independent Test**: Can be fully tested by sending a single chat completion request with `modalities: ["text", "image"]` to the Tianji proxy and verifying the response contains an `image_url` content part with a valid base64 data URL.

**Acceptance Scenarios**:

1. **Given** a request with `modalities: ["text", "image"]` and a text prompt, **When** Tianji proxies to Gemini and Gemini returns `inlineData` in the response, **Then** the Tianji response MUST contain `message.content` as an array including at least one item with `type: "image_url"` and a `data:` URL containing the base64-encoded image.
2. **Given** a request with `modalities: ["text", "image"]`, **When** Tianji builds the Gemini request, **Then** the upstream Gemini request MUST contain `generationConfig.responseModalities: ["TEXT", "IMAGE"]`.
3. **Given** a Gemini response that contains both text parts and image (`inlineData`) parts, **When** Tianji transforms the response, **Then** the OpenAI response MUST contain `message.content` as an array with both `type: "text"` and `type: "image_url"` entries, preserving order.

---

### User Story 2 - Image-to-Image (Round-Trip) (Priority: P2)

A developer wants to send an existing image as input and ask Gemini to generate a modified or new image as output — a classic image-to-image workflow. They send a multimodal request with an image in `message.content` (as `image_url`) alongside `modalities: ["text", "image"]`, and expect a generated image back.

**Why this priority**: Image-to-image is a key use case for persona-factory (e.g., style transfer, image editing). It builds on P1 (image output) combined with the already-supported image input.

**Independent Test**: Can be fully tested by sending a request that includes a base64 image in `message.content[].image_url` plus `modalities: ["text", "image"]`, and verifying the response includes an `image_url` content part.

**Acceptance Scenarios**:

1. **Given** a request where `messages[].content` contains an `image_url` part AND `modalities: ["text", "image"]`, **When** Tianji proxies to Gemini, **Then** the upstream request MUST include both the input image as `inlineData` and `generationConfig.responseModalities: ["TEXT", "IMAGE"]`.
2. **Given** a Gemini image-to-image response with `inlineData`, **When** Tianji transforms it, **Then** the output image MUST be returned as `image_url` with the correct MIME type preserved in the data URL (e.g., `data:image/png;base64,...`).

---

### User Story 3 - Backward Compatibility for Text-Only Flows (Priority: P3)

An existing Tianji user calling Gemini models for text-only tasks (no `modalities` field in request) MUST continue to receive plain string `message.content` responses, with no behavioral change.

**Why this priority**: Non-regression is critical — existing integrations must not break. This validates that the new feature is purely additive.

**Independent Test**: Send a standard text-only Gemini request (no `modalities` field) and verify the response `message.content` is a plain string, not an array.

**Acceptance Scenarios**:

1. **Given** a request without a `modalities` field, **When** Tianji proxies to Gemini and receives a text-only response, **Then** `message.content` MUST remain a plain string (not an array), unchanged from the current behavior.
2. **Given** a request with `modalities: ["text"]` (text only), **When** Tianji builds the Gemini request, **Then** `generationConfig.responseModalities` MUST be set to `["TEXT"]` and the response MUST remain a plain string.

---

### Edge Cases

- What happens when Gemini returns an `inlineData` part with an unexpected or unsupported MIME type (e.g., `video/mp4`)? The proxy should pass through the data URL with the original MIME type rather than failing.
- What happens if `modalities` is present but empty (`[]`)? The field should be ignored and the request proceeds as text-only.
- What happens when Gemini returns a response with only image parts and no text? `message.content` MUST be an array containing only `image_url` items, and `finish_reason` should be `"stop"`.
- What happens when streaming is enabled along with `modalities: ["image"]`? Image data in streaming responses (if Gemini returns `inlineData` in stream chunks) MUST be forwarded correctly.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The proxy MUST accept an optional `modalities` field (array of strings) in the chat completion request body.
- **FR-002**: When `modalities` contains `"image"`, the proxy MUST include `generationConfig.responseModalities: ["TEXT", "IMAGE"]` in the upstream Gemini request.
- **FR-003**: When `modalities` contains only `"text"` (or is absent), the proxy MUST NOT set `responseModalities` in the upstream Gemini request (preserving current behavior).
- **FR-004**: The proxy MUST parse Gemini response parts that contain `inlineData` (base64-encoded image blobs with a MIME type).
- **FR-005**: Each `inlineData` response part MUST be mapped to an OpenAI `ContentPart` with `type: "image_url"` and `image_url.url` set to a data URL in the format `data:<mimeType>;base64,<data>`.
- **FR-006**: When a Gemini response contains a mix of text and image parts, the proxy MUST return `message.content` as a JSON array preserving the original ordering of text and image parts.
- **FR-007**: When a Gemini response contains only text parts (no `inlineData`), the proxy MUST continue to return `message.content` as a plain string (no breaking change).
- **FR-008**: The proxy MUST correctly handle image input (`image_url` in request messages) combined with image output (`modalities: ["text", "image"]`) in the same request.

### Key Entities

- **Modalities**: An optional array of output type strings in the OpenAI request (`"text"`, `"image"`). Maps to Gemini's `generationConfig.responseModalities` (`"TEXT"`, `"IMAGE"`).
- **InlineData Part**: A Gemini response part containing a base64-encoded binary blob with a MIME type. Corresponds to a generated image.
- **Content Part (image_url)**: The OpenAI representation of an inline image — a `ContentPart` with `type: "image_url"` and a `data:` URL encoding the binary data.
- **Mixed Content Response**: A `message.content` that is an array of `ContentPart` objects (text and/or image), as opposed to a plain string. Required whenever the response contains any image parts.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A developer can call a Gemini image-generation model through Tianji and receive a valid image in the response without writing any provider-specific code — the standard OpenAI SDK chat completion call works as-is.
- **SC-002**: All existing Gemini text-only integration tests continue to pass without modification after this change is deployed.
- **SC-003**: A round-trip image-to-image request (image input + image output) completes successfully and the output image data URL contains the correct MIME type declared by Gemini.
- **SC-004**: Unit tests cover all three mapping paths: (a) text-only response → plain string content, (b) image-only response → array with image_url, (c) mixed response → array preserving order.
- **SC-005**: The proxy adds zero latency overhead beyond the base Gemini API call for text-only requests (no regression in the common path).

## Assumptions

- Gemini uses `"TEXT"` and `"IMAGE"` (uppercase) as values for `responseModalities`; OpenAI uses lowercase `"text"` and `"image"` for `modalities`.
- Streaming image output from Gemini (if supported by the API) uses the same `inlineData` structure in stream chunks; the streaming path should handle this identically to the non-streaming path.
- The `data:` URL format (`data:<mimeType>;base64,<data>`) is the correct OpenAI-compatible encoding for inline images, as used by GPT-4o vision responses.
- `modalities` is an additive field; existing proxy configurations and model configs do not need to declare it explicitly — it is passed through from the client request.
- Image generation models like `gemini-2-flash-exp` follow the same Gemini API format as text models, just with `inlineData` parts in the response.
