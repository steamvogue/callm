package client

// Message represents an OpenAI-compatible chat message.
type Message struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // string or []ContentPart for multimodal
}

// ContentPart represents a text or image content part for multimodal messages.
type ContentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

// ImageURL holds URL or base64 data URI for vision models.
type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

// ResponseFormat specifies structured outputs e.g. json_object.
type ResponseFormat struct {
	Type string `json:"type"`
}

// ChatRequest is the payload sent to /v1/chat/completions.
type ChatRequest struct {
	Model          string          `json:"model"`
	Messages       []Message       `json:"messages"`
	Stream         bool            `json:"stream,omitempty"`
	Temperature    *float64        `json:"temperature,omitempty"`
	MaxTokens      *int            `json:"max_tokens,omitempty"`
	TopP           *float64        `json:"top_p,omitempty"`
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"`
	Stop           []string        `json:"stop,omitempty"`
}

// Usage holds token count and cost metadata.
type Usage struct {
	PromptTokens        int         `json:"prompt_tokens"`
	CompletionTokens    int         `json:"completion_tokens"`
	TotalTokens         int         `json:"total_tokens"`
	Cost                interface{} `json:"cost,omitempty"` // float64 or string or nil
	CacheReadInputTokens int        `json:"cache_read_input_tokens,omitempty"`
}

// GetCostFloat returns cost as float64 if available.
func (u *Usage) GetCostFloat() float64 {
	if u == nil || u.Cost == nil {
		return 0.0
	}
	switch v := u.Cost.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	default:
		return 0.0
	}
}

// ChatChoice represents a completion choice.
type ChatChoice struct {
	Index        int      `json:"index"`
	Message      RespMsg  `json:"message"`
	FinishReason string   `json:"finish_reason"`
}

// RespMsg holds the assistant response content and reasoning.
type RespMsg struct {
	Role             string `json:"role"`
	Content          string `json:"content"`
	Reasoning        string `json:"reasoning,omitempty"`
	ReasoningContent string `json:"reasoning_content,omitempty"`
}

// ChatResponse is the response from non-streaming /v1/chat/completions.
type ChatResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []ChatChoice `json:"choices"`
	Usage   *Usage       `json:"usage,omitempty"`
	Error   *APIError    `json:"error,omitempty"`
}

// StreamDelta holds streaming token delta.
type StreamDelta struct {
	Role             string `json:"role,omitempty"`
	Content          string `json:"content,omitempty"`
	Reasoning        string `json:"reasoning,omitempty"`
	ReasoningContent string `json:"reasoning_content,omitempty"`
}

// StreamChoice is a chunk choice.
type StreamChoice struct {
	Index        int         `json:"index"`
	Delta        StreamDelta `json:"delta"`
	FinishReason *string     `json:"finish_reason"`
}

// StreamChunk is an SSE data chunk from /v1/chat/completions.
type StreamChunk struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []StreamChoice `json:"choices"`
	Usage   *Usage         `json:"usage,omitempty"`
	Error   *APIError      `json:"error,omitempty"`
}

// APIError represents an error returned by OpenAI-compatible API.
type APIError struct {
	Message string      `json:"message"`
	Type    string      `json:"type,omitempty"`
	Code    interface{} `json:"code,omitempty"`
}

// ModelListResponse represents the list of models from /v1/models.
type ModelListResponse struct {
	Data  []ModelInfo `json:"data"`
	Error *APIError   `json:"error,omitempty"`
}

// ModelInfo contains model catalog information.
type ModelInfo struct {
	ID                  string        `json:"id"`
	CanonicalSlug       string        `json:"canonical_slug,omitempty"`
	Name                string        `json:"name,omitempty"`
	Description         string        `json:"description,omitempty"`
	ContextLength       int64         `json:"context_length"`
	Architecture        *Architecture `json:"architecture,omitempty"`
	Pricing             *ModelPricing `json:"pricing,omitempty"`
	SupportedParameters []string      `json:"supported_parameters,omitempty"`
}

// Architecture details modalities and tokenizer.
type Architecture struct {
	Modality         string   `json:"modality,omitempty"`
	InputModalities  []string `json:"input_modalities,omitempty"`
	OutputModalities []string `json:"output_modalities,omitempty"`
}

// ModelPricing contains per-token pricing.
type ModelPricing struct {
	Prompt          interface{} `json:"prompt"`
	Completion      interface{} `json:"completion"`
	InputCacheRead  interface{} `json:"input_cache_read,omitempty"`
	InputCacheWrite interface{} `json:"input_cache_write,omitempty"`
}
