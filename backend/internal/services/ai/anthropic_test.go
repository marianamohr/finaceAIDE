package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func newTestProvider(t *testing.T, server *httptest.Server) *AnthropicProvider {
	t.Helper()

	p := NewAnthropicProvider("test-key")
	p.apiURL = server.URL
	p.httpClient = &http.Client{Timeout: 5 * time.Second}

	return p
}

func TestAnthropicProvider_ChatStructured_success(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"items":{"type":"array"}}}`)

	var capturedBody anthropicStructuredRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}

		resp := anthropicStructuredResponse{
			Content: []anthropicContentBlock{
				{Type: "text", Text: `{"items":[]}`},
			},
			StopReason: "end_turn",
			Usage:      anthropicUsage{InputTokens: 123, OutputTokens: 456},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	p := newTestProvider(t, server)

	got, err := p.ChatStructured(context.Background(), StructuredRequest{
		Model:        "claude-sonnet-5",
		SystemPrompt: "extract items",
		UserText:     "note fiscal text",
		Schema:       schema,
		Effort:       "low",
	})
	if err != nil {
		t.Fatalf("ChatStructured() error = %v, want nil", err)
	}

	if got.JSON != `{"items":[]}` {
		t.Errorf("JSON = %q, want %q", got.JSON, `{"items":[]}`)
	}
	if got.StopReason != "end_turn" {
		t.Errorf("StopReason = %q, want %q", got.StopReason, "end_turn")
	}
	if got.InputTokens != 123 {
		t.Errorf("InputTokens = %d, want 123", got.InputTokens)
	}
	if got.OutputTokens != 456 {
		t.Errorf("OutputTokens = %d, want 456", got.OutputTokens)
	}

	if capturedBody.OutputConfig.Format.Type != "json_schema" {
		t.Errorf("output_config.format.type = %q, want %q", capturedBody.OutputConfig.Format.Type, "json_schema")
	}
	if string(capturedBody.OutputConfig.Format.Schema) != string(schema) {
		t.Errorf("output_config.format.schema = %s, want %s", capturedBody.OutputConfig.Format.Schema, schema)
	}
	if capturedBody.OutputConfig.Effort != "low" {
		t.Errorf("output_config.effort = %q, want %q", capturedBody.OutputConfig.Effort, "low")
	}
}

// TestAnthropicProvider_ChatStructured_skipsLeadingThinkingBlock guards
// against reading content[0] directly: adaptive thinking (on by default for
// claude-sonnet-5, even with no "thinking" field in the request) can prepend
// a "thinking" block before the "text" block that carries the JSON. Found via
// a real recipe-generation call that failed intermittently with "unexpected
// end of JSON input" — reproducible once effort was raised enough that the
// model actually thought before answering.
func TestAnthropicProvider_ChatStructured_skipsLeadingThinkingBlock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := anthropicStructuredResponse{
			Content: []anthropicContentBlock{
				{Type: "thinking", Text: ""},
				{Type: "text", Text: `{"title":"Arroz com feijão"}`},
			},
			StopReason: "end_turn",
			Usage:      anthropicUsage{InputTokens: 10, OutputTokens: 20},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	p := newTestProvider(t, server)

	got, err := p.ChatStructured(context.Background(), StructuredRequest{
		Model:        "claude-sonnet-5",
		SystemPrompt: "generate a recipe",
		UserText:     "gere a receita",
		Schema:       json.RawMessage(`{"type":"object"}`),
		Effort:       "medium",
	})
	if err != nil {
		t.Fatalf("ChatStructured() error = %v, want nil", err)
	}
	if got.JSON != `{"title":"Arroz com feijão"}` {
		t.Errorf("JSON = %q, want the text block's content, not the empty thinking block", got.JSON)
	}
}

func TestAnthropicProvider_ChatStructured_retriesOnTransientError(t *testing.T) {
	var callCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		if n <= 2 {
			w.WriteHeader(529)
			_, _ = w.Write([]byte(`{"error":"overloaded"}`))
			return
		}

		resp := anthropicStructuredResponse{
			Content:    []anthropicContentBlock{{Type: "text", Text: `{"ok":true}`}},
			StopReason: "end_turn",
			Usage:      anthropicUsage{InputTokens: 1, OutputTokens: 1},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := newTestProvider(t, server)

	// The retry backoff sleeps 1s then 2s; give the test enough headroom.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	got, err := p.ChatStructured(ctx, StructuredRequest{
		Model:    "claude-sonnet-5",
		UserText: "hello",
		Schema:   json.RawMessage(`{"type":"object"}`),
	})
	if err != nil {
		t.Fatalf("ChatStructured() error = %v, want nil", err)
	}
	if got.JSON != `{"ok":true}` {
		t.Errorf("JSON = %q, want %q", got.JSON, `{"ok":true}`)
	}

	if n := atomic.LoadInt32(&callCount); n != 3 {
		t.Errorf("server called %d times, want 3", n)
	}
}

func TestAnthropicProvider_ChatStructured_permanentErrorNoRetry(t *testing.T) {
	var callCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid schema"}`))
	}))
	defer server.Close()

	p := newTestProvider(t, server)

	_, err := p.ChatStructured(context.Background(), StructuredRequest{
		Model:    "claude-sonnet-5",
		UserText: "hello",
		Schema:   json.RawMessage(`{"type":"object"}`),
	})
	if err == nil {
		t.Fatal("ChatStructured() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("error = %v, want it to mention status 400", err)
	}

	if n := atomic.LoadInt32(&callCount); n != 1 {
		t.Errorf("server called %d times, want 1 (no retry on permanent error)", n)
	}
}

func TestAnthropicProvider_ChatStructured_contextCancelledDuringRetryWait(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"unavailable"}`))
	}))
	defer server.Close()

	p := newTestProvider(t, server)

	// Short enough to expire during the 1s wait before the second attempt,
	// but long enough for the first HTTP round trip to complete.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		_, err := p.ChatStructured(ctx, StructuredRequest{
			Model:    "claude-sonnet-5",
			UserText: "hello",
			Schema:   json.RawMessage(`{"type":"object"}`),
		})
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("ChatStructured() error = nil, want context-related error")
		}
		if !strings.Contains(err.Error(), "context") {
			t.Errorf("error = %v, want it to mention context cancellation", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ChatStructured() did not return within 5s — likely hung on retry wait")
	}
}
