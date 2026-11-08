package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultAnthropicAPIURL = "https://api.anthropic.com/v1/messages"
const anthropicVersion = "2023-06-01"

// AnthropicProvider implements Provider using the Anthropic Messages API.
// The API key is stored only in memory, never logged.
type AnthropicProvider struct {
	apiKey     string
	apiURL     string // defaults to defaultAnthropicAPIURL; overridable in tests
	httpClient *http.Client
}

func NewAnthropicProvider(apiKey string) *AnthropicProvider {
	return &AnthropicProvider{
		apiKey: apiKey,
		apiURL: defaultAnthropicAPIURL,
		httpClient: &http.Client{
			Timeout: 90 * time.Second,
		},
	}
}

// WithUserKey returns a shallow copy of the provider with the API key replaced.
// The original provider is not modified.
func (p *AnthropicProvider) WithUserKey(key string) *AnthropicProvider {
	clone := *p
	clone.apiKey = key
	return &clone
}

// ── Chat ──────────────────────────────────────────────────────────────────────

func (p *AnthropicProvider) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	msgs := make([]anthropicMessage, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = anthropicMessage{
			Role:    m.Role,
			Content: []anthropicContent{{Type: "text", Text: m.Content}},
		}
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 2048
	}

	body := anthropicRequest{
		Model:     req.Model,
		MaxTokens: maxTokens,
		Messages:  msgs,
	}
	if req.SystemPrompt != "" {
		body.System = req.SystemPrompt
	}

	return p.do(ctx, body)
}

// ── ChatWithImage ─────────────────────────────────────────────────────────────

func (p *AnthropicProvider) ChatWithImage(ctx context.Context, req ImageChatRequest) (ChatResponse, error) {
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 1024
	}

	body := anthropicRequest{
		Model:     req.Model,
		MaxTokens: maxTokens,
		Messages: []anthropicMessage{
			{
				Role: "user",
				Content: []anthropicContent{
					{
						Type: "image",
						Source: &anthropicImageSource{
							Type:      "base64",
							MediaType: req.MIMEType,
							Data:      base64.StdEncoding.EncodeToString(req.ImageData),
						},
					},
					{Type: "text", Text: req.Prompt},
				},
			},
		},
	}

	return p.do(ctx, body)
}

// ── ChatWithTools ─────────────────────────────────────────────────────────────

func (p *AnthropicProvider) ChatWithTools(ctx context.Context, req ChatWithToolsRequest) (ChatWithToolsResponse, error) {
	msgs := make([]anthropicMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		if len(m.ContentBlocks) > 0 {
			msgs = append(msgs, anthropicMessage{Role: m.Role, Content: m.ContentBlocks})
		} else if strings.TrimSpace(m.Content) != "" {
			msgs = append(msgs, anthropicMessage{
				Role:    m.Role,
				Content: []anthropicContent{{Type: "text", Text: m.Content}},
			})
		}
		// skip messages with no content to avoid API validation errors
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 2048
	}

	body := anthropicToolUseRequest{
		Model:     req.Model,
		System:    req.SystemPrompt,
		Messages:  msgs,
		Tools:     req.Tools,
		MaxTokens: maxTokens,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return ChatWithToolsResponse{}, fmt.Errorf("anthropic: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.apiURL, bytes.NewReader(payload))
	if err != nil {
		return ChatWithToolsResponse{}, fmt.Errorf("anthropic: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return ChatWithToolsResponse{}, fmt.Errorf("anthropic: http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return ChatWithToolsResponse{}, fmt.Errorf("anthropic: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return ChatWithToolsResponse{}, fmt.Errorf("anthropic: api error %d: %s", resp.StatusCode, string(respBody))
	}

	var result anthropicToolUseResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return ChatWithToolsResponse{}, fmt.Errorf("anthropic: unmarshal response: %w", err)
	}

	var textParts []string
	toolUses := []ToolUseBlock{}
	for _, block := range result.Content {
		switch block.Type {
		case "text":
			if block.Text != "" {
				textParts = append(textParts, block.Text)
			}
		case "tool_use":
			toolUses = append(toolUses, ToolUseBlock{
				ID:    block.ID,
				Name:  block.Name,
				Input: block.Input,
			})
		}
	}

	return ChatWithToolsResponse{
		Content:    strings.Join(textParts, ""),
		StopReason: result.StopReason,
		ToolUses:   toolUses,
	}, nil
}

// ── ChatStructured ────────────────────────────────────────────────────────────

// structuredMaxRetries is the number of extra attempts after the first one
// (3 attempts total) for transient errors (429, 5xx, 529, network failures).
const structuredMaxRetries = 2

// ChatStructured sends a request whose response is constrained to match a
// JSON Schema (Anthropic's output_config.format = json_schema). Unlike do(),
// it retries on transient errors: HTTP 429/500/502/503/529, or a network
// error from the HTTP client. Permanent errors (4xx other than 429) are
// returned immediately without retrying.
func (p *AnthropicProvider) ChatStructured(ctx context.Context, req StructuredRequest) (StructuredResponse, error) {
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 8000
	}

	var content interface{}
	if len(req.ImageData) > 0 {
		content = []anthropicContent{
			{
				Type: "image",
				Source: &anthropicImageSource{
					Type:      "base64",
					MediaType: req.MIMEType,
					Data:      base64.StdEncoding.EncodeToString(req.ImageData),
				},
			},
			{Type: "text", Text: req.UserText},
		}
	} else {
		content = req.UserText
	}

	body := anthropicStructuredRequest{
		Model:     req.Model,
		MaxTokens: maxTokens,
		Messages: []anthropicMessage{
			{Role: "user", Content: content},
		},
		OutputConfig: anthropicOutputConfig{
			Format: anthropicOutputFormat{
				Type:   "json_schema",
				Schema: req.Schema,
			},
		},
	}
	if req.SystemPrompt != "" {
		body.System = []anthropicSystemBlock{
			{Type: "text", Text: req.SystemPrompt, CacheControl: &anthropicCacheControl{Type: "ephemeral"}},
		}
	}
	if req.Effort != "" {
		body.OutputConfig.Effort = req.Effort
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return StructuredResponse{}, fmt.Errorf("anthropic: marshal request: %w", err)
	}

	var lastErr error
	for attempt := 1; attempt <= structuredMaxRetries+1; attempt++ {
		if attempt > 1 {
			if waitErr := sleepOrCancel(ctx, time.Duration(attempt-1)*time.Second); waitErr != nil {
				return StructuredResponse{}, waitErr
			}
		}

		resp, retryable, doErr := p.doStructuredOnce(ctx, payload)
		if doErr == nil {
			return resp, nil
		}
		lastErr = doErr
		if !retryable {
			return StructuredResponse{}, lastErr
		}
	}

	return StructuredResponse{}, lastErr
}

// doStructuredOnce performs a single HTTP attempt for ChatStructured. The
// retryable flag tells the caller whether it is worth retrying the request.
func (p *AnthropicProvider) doStructuredOnce(ctx context.Context, payload []byte) (StructuredResponse, bool, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.apiURL, bytes.NewReader(payload))
	if err != nil {
		return StructuredResponse{}, false, fmt.Errorf("anthropic: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return StructuredResponse{}, true, fmt.Errorf("anthropic: http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return StructuredResponse{}, true, fmt.Errorf("anthropic: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		retryable := isRetryableStatus(resp.StatusCode)
		return StructuredResponse{}, retryable, fmt.Errorf("anthropic: api error %d: %s", resp.StatusCode, string(respBody))
	}

	var result anthropicStructuredResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return StructuredResponse{}, false, fmt.Errorf("anthropic: unmarshal response: %w", err)
	}

	if len(result.Content) == 0 {
		return StructuredResponse{}, false, errors.New("anthropic: empty response content")
	}

	text, err := structuredResponseText(result.Content)
	if err != nil {
		return StructuredResponse{}, false, err
	}

	return StructuredResponse{
		JSON:         text,
		StopReason:   result.StopReason,
		InputTokens:  result.Usage.InputTokens,
		OutputTokens: result.Usage.OutputTokens,
	}, false, nil
}

// structuredResponseText finds the model's actual answer among the response's
// content blocks. content[0] is NOT reliably the text block: Claude Sonnet 5
// runs adaptive thinking by default (thinking is on even when the request
// never sets the "thinking" field), and when it decides to think, a
// "thinking" block precedes the "text" block carrying the JSON. Blindly
// reading content[0].Text silently returns an empty string whenever the model
// thinks — which effort level and prompt make more or less likely, so the
// failure is intermittent rather than a hard reproduction. Scanning for the
// first block with type "text" is correct regardless of how many blocks (or
// which kind) come before it.
func structuredResponseText(content []anthropicContentBlock) (string, error) {
	for _, block := range content {
		if block.Type == "text" {
			return block.Text, nil
		}
	}
	return "", errors.New("anthropic: no text block in response content")
}

// isRetryableStatus reports whether an HTTP status code from the Anthropic
// API indicates a transient error worth retrying.
func isRetryableStatus(status int) bool {
	switch status {
	case http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, 529:
		return true
	default:
		return false
	}
}

// sleepOrCancel waits for d, returning early with a wrapped context error if
// ctx is cancelled first.
func sleepOrCancel(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("anthropic: retry wait cancelled: %w", ctx.Err())
	}
}

// ── internal ──────────────────────────────────────────────────────────────────

func (p *AnthropicProvider) do(ctx context.Context, body anthropicRequest) (ChatResponse, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("anthropic: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.apiURL, bytes.NewReader(payload))
	if err != nil {
		return ChatResponse{}, fmt.Errorf("anthropic: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("anthropic: http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("anthropic: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return ChatResponse{}, fmt.Errorf("anthropic: api error %d: %s", resp.StatusCode, string(respBody))
	}

	var result anthropicResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return ChatResponse{}, fmt.Errorf("anthropic: unmarshal response: %w", err)
	}

	if len(result.Content) == 0 {
		return ChatResponse{}, errors.New("anthropic: empty response content")
	}

	return ChatResponse{Content: result.Content[0].Text}, nil
}

// ── wire types ────────────────────────────────────────────────────────────────

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type anthropicContent struct {
	Type   string                `json:"type"`
	Text   string                `json:"text,omitempty"`
	Source *anthropicImageSource `json:"source,omitempty"`
}

type anthropicImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

type anthropicResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
}

type anthropicToolUseRequest struct {
	Model     string             `json:"model"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
	Tools     []ToolDefinition   `json:"tools"`
	MaxTokens int                `json:"max_tokens"`
}

type anthropicContentBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

type anthropicToolUseResponse struct {
	Content    []anthropicContentBlock `json:"content"`
	StopReason string                  `json:"stop_reason"`
}

type anthropicStructuredRequest struct {
	Model        string                 `json:"model"`
	MaxTokens    int                    `json:"max_tokens"`
	System       []anthropicSystemBlock `json:"system,omitempty"`
	Messages     []anthropicMessage     `json:"messages"`
	OutputConfig anthropicOutputConfig  `json:"output_config"`
}

// anthropicSystemBlock is the system prompt as a content block rather than a
// plain string — the only shape that accepts cache_control. Every
// ChatStructured call marks its (often reused, per-user) system prompt as an
// ephemeral cache breakpoint: receipt extraction and recipe generation embed
// the user's catalog in the system prompt, so repeated calls for the same
// user within the cache TTL reuse it instead of paying full input price
// again. Only this struct changes — anthropicRequest/anthropicToolUseRequest
// (Chat/ChatWithImage/ChatWithTools) keep a plain string System, per the
// design constraint that those methods don't change.
type anthropicSystemBlock struct {
	Type         string                 `json:"type"`
	Text         string                 `json:"text"`
	CacheControl *anthropicCacheControl `json:"cache_control,omitempty"`
}

type anthropicCacheControl struct {
	Type string `json:"type"`
}

type anthropicOutputConfig struct {
	Format anthropicOutputFormat `json:"format"`
	Effort string                `json:"effort,omitempty"`
}

type anthropicOutputFormat struct {
	Type   string          `json:"type"`
	Schema json.RawMessage `json:"schema"`
}

type anthropicStructuredResponse struct {
	Content    []anthropicContentBlock `json:"content"`
	StopReason string                  `json:"stop_reason"`
	Usage      anthropicUsage          `json:"usage"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}
