package ai

import (
	"context"
	"encoding/json"
)

// Message represents a single turn in a conversation.
// ContentBlocks, when non-nil, is used directly as the message content
// (e.g. a pre-encoded tool_use / tool_result block array). Content is
// ignored in that case.
type Message struct {
	Role          string          `json:"role"`
	Content       string          `json:"content"`
	ContentBlocks json.RawMessage `json:"content_blocks,omitempty"`
}

// ChatRequest is the input to the AI provider.
type ChatRequest struct {
	Model        string
	SystemPrompt string
	Messages     []Message
	MaxTokens    int
}

// ChatResponse is the output from the AI provider.
type ChatResponse struct {
	Content string
}

// ImageChatRequest is used for vision tasks (OCR).
type ImageChatRequest struct {
	Model     string
	Prompt    string
	ImageData []byte
	MIMEType  string
	MaxTokens int
}

// ToolDefinition describes a tool the model can call.
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// ToolUseBlock is returned when the model wants to call a tool.
type ToolUseBlock struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// ToolResultBlock carries the result of a tool call back to the model.
type ToolResultBlock struct {
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
}

// ChatWithToolsRequest is the request for tool-enabled chat.
type ChatWithToolsRequest struct {
	Model        string
	SystemPrompt string
	Messages     []Message
	Tools        []ToolDefinition
	MaxTokens    int
}

// ChatWithToolsResponse is the response from the model.
// StopReason is "end_turn" or "tool_use".
// ToolUses is populated when StopReason == "tool_use".
type ChatWithToolsResponse struct {
	Content    string
	StopReason string
	ToolUses   []ToolUseBlock
}

// StructuredRequest is a request whose response is constrained to match a JSON
// Schema exactly (Anthropic's output_config.format = json_schema), optionally
// with a vision input.
type StructuredRequest struct {
	Model        string
	SystemPrompt string
	UserText     string          // sent as the user message text; the image block (if any) comes first
	ImageData    []byte          // optional — omit for a text-only structured request
	MIMEType     string          // required when ImageData is set
	Schema       json.RawMessage // the JSON Schema object (goes into output_config.format.schema)
	Effort       string          // "low" | "medium" | "high" | "xhigh" | "max" — empty means omit the field (provider default)
	MaxTokens    int             // 0 means use the method's own default
}

// StructuredResponse is the result of a StructuredRequest. JSON is the raw text
// of the model's response — schema-valid per the API's guarantee, but still
// worth validating with encoding/json.Unmarshal against a concrete struct at
// the call site, since "valid JSON matching the schema" does not mean "the
// values make business sense".
type StructuredResponse struct {
	JSON         string
	StopReason   string
	InputTokens  int
	OutputTokens int
}

// Provider is the interface every AI backend must satisfy.
type Provider interface {
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
	ChatWithImage(ctx context.Context, req ImageChatRequest) (ChatResponse, error)
	ChatWithTools(ctx context.Context, req ChatWithToolsRequest) (ChatWithToolsResponse, error)
	ChatStructured(ctx context.Context, req StructuredRequest) (StructuredResponse, error)
}
