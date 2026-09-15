package aigateway

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIAdapterInvokeParsesContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Fatalf("Authorization = %q, want bearer key", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"req_1","choices":[{"message":{"content":"hello"}}],"usage":{"prompt_tokens":2,"completion_tokens":3}}`))
	}))
	defer server.Close()

	adapter := NewOpenAIAdapter(server.URL, "sk-test", server.Client())
	resp, err := adapter.Invoke(context.Background(), ProviderRequest{Model: "gpt-test", Messages: []Message{{Role: "user", Content: "hi"}}})
	if err != nil {
		t.Fatalf("Invoke returned error: %v", err)
	}
	if resp.ProviderRequestID != "req_1" {
		t.Fatalf("ProviderRequestID = %q, want req_1", resp.ProviderRequestID)
	}
	if resp.Content != "hello" {
		t.Fatalf("Content = %q, want hello", resp.Content)
	}
	if resp.Usage.InputTokens != 2 || resp.Usage.OutputTokens != 3 {
		t.Fatalf("usage = %+v, want 2/3", resp.Usage)
	}
}

func TestProviderAdaptersDefaultBaseURLs(t *testing.T) {
	if got := NewOpenAIAdapter("", "sk-test", nil).baseURL; got != defaultOpenAIBaseURL {
		t.Fatalf("OpenAI baseURL = %q, want %q", got, defaultOpenAIBaseURL)
	}
	if got := NewAnthropicAdapter("", "sk-test", nil).baseURL; got != defaultAnthropicBaseURL {
		t.Fatalf("Anthropic baseURL = %q, want %q", got, defaultAnthropicBaseURL)
	}
	if got := NewGeminiAdapter("", "sk-test", nil).baseURL; got != defaultGeminiBaseURL {
		t.Fatalf("Gemini baseURL = %q, want %q", got, defaultGeminiBaseURL)
	}
}

func TestOpenAIAdapterInvokeReturnsProviderError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "invalid api key", http.StatusUnauthorized)
	}))
	defer server.Close()

	adapter := NewOpenAIAdapter(server.URL, "bad-key", server.Client())
	_, err := adapter.Invoke(context.Background(), ProviderRequest{Model: "gpt-test", Messages: []Message{{Role: "user", Content: "hi"}}})
	if err == nil {
		t.Fatalf("Invoke returned nil error")
	}
	var providerErr *ProviderError
	if !errors.As(err, &providerErr) {
		t.Fatalf("error type = %T, want *ProviderError", err)
	}
	if providerErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("StatusCode = %d, want 401", providerErr.StatusCode)
	}
	if !strings.Contains(providerErr.Body, "invalid api key") {
		t.Fatalf("Body = %q, want provider body summary", providerErr.Body)
	}
}

func TestAnthropicAdapterInvokeParsesContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-api-key"); got != "sk-test" {
			t.Fatalf("x-api-key = %q, want sk-test", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"msg_1","content":[{"type":"text","text":"hello"}],"usage":{"input_tokens":4,"output_tokens":5}}`))
	}))
	defer server.Close()

	adapter := NewAnthropicAdapter(server.URL, "sk-test", server.Client())
	resp, err := adapter.Invoke(context.Background(), ProviderRequest{Model: "claude-test", Messages: []Message{{Role: "user", Content: "hi"}}})
	if err != nil {
		t.Fatalf("Invoke returned error: %v", err)
	}
	if resp.ProviderRequestID != "msg_1" || resp.Content != "hello" {
		t.Fatalf("response = %+v, want msg_1/hello", resp)
	}
	if resp.Usage.InputTokens != 4 || resp.Usage.OutputTokens != 5 {
		t.Fatalf("usage = %+v, want 4/5", resp.Usage)
	}
}

func TestGeminiAdapterInvokeParsesContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-goog-api-key"); got != "sk-test" || r.URL.Query().Has("key") {
			t.Fatalf("API key must be sent only in the header, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"hello"}]}}],"usageMetadata":{"promptTokenCount":6,"candidatesTokenCount":7}}`))
	}))
	defer server.Close()

	adapter := NewGeminiAdapter(server.URL, "sk-test", server.Client())
	resp, err := adapter.Invoke(context.Background(), ProviderRequest{Model: "gemini-test", Messages: []Message{{Role: "user", Content: "hi"}}})
	if err != nil {
		t.Fatalf("Invoke returned error: %v", err)
	}
	if resp.Content != "hello" {
		t.Fatalf("Content = %q, want hello", resp.Content)
	}
	if resp.Usage.InputTokens != 6 || resp.Usage.OutputTokens != 7 {
		t.Fatalf("usage = %+v, want 6/7", resp.Usage)
	}
}

func TestProviderStreamsNormalizeDeltaAndDone(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		adapter func(baseURL string, client *http.Client) ProviderAdapter
	}{
		{
			name: "openai",
			body: "data: {\"choices\":[{\"delta\":{\"content\":\"he\"}}]}\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"llo\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":2}}\n\ndata: [DONE]\n\n",
			adapter: func(baseURL string, client *http.Client) ProviderAdapter {
				return NewOpenAIAdapter(baseURL, "sk-test", client)
			},
		},
		{
			name: "anthropic",
			body: "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"he\"}}\n\nevent: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"llo\"}}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n",
			adapter: func(baseURL string, client *http.Client) ProviderAdapter {
				return NewAnthropicAdapter(baseURL, "sk-test", client)
			},
		},
		{
			name: "gemini",
			body: "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"he\"}]}}]}\n\ndata: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"llo\"}]},\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":1,\"candidatesTokenCount\":2}}\n\n",
			adapter: func(baseURL string, client *http.Client) ProviderAdapter {
				return NewGeminiAdapter(baseURL, "sk-test", client)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			events, err := tt.adapter(server.URL, server.Client()).Stream(context.Background(), ProviderRequest{Model: "test", Messages: []Message{{Role: "user", Content: "hi"}}})
			if err != nil {
				t.Fatalf("Stream returned error: %v", err)
			}
			collected := collectStreamEvents(events)
			if joined := collectedText(collected); joined != "hello" {
				t.Fatalf("stream text = %q, want hello; events=%+v", joined, collected)
			}
			if !hasDoneEvent(collected) {
				t.Fatalf("stream did not emit done event: %+v", collected)
			}
			if count := doneEventCount(collected); count != 1 {
				t.Fatalf("done event count = %d, want 1; events=%+v", count, collected)
			}
		})
	}
}

func TestStreamParserEmitsDeltaAndDone(t *testing.T) {
	raw := strings.NewReader("data: {\"type\":\"delta\",\"text\":\"he\"}\n\ndata: {\"type\":\"delta\",\"text\":\"llo\"}\n\ndata: [DONE]\n\n")
	events, err := ParseSSE(raw)
	if err != nil {
		t.Fatalf("ParseSSE returned error: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("events length = %d, want 3", len(events))
	}
	if events[0].Data == "" || !events[2].Done {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func collectStreamEvents(ch <-chan StreamEvent) []StreamEvent {
	events := []StreamEvent{}
	for event := range ch {
		events = append(events, event)
	}
	return events
}

func collectedText(events []StreamEvent) string {
	var b strings.Builder
	for _, event := range events {
		b.WriteString(event.Delta)
	}
	return b.String()
}

func hasDoneEvent(events []StreamEvent) bool {
	for _, event := range events {
		if event.Done {
			return true
		}
	}
	return false
}

func doneEventCount(events []StreamEvent) int {
	count := 0
	for _, event := range events {
		if event.Done {
			count++
		}
	}
	return count
}
