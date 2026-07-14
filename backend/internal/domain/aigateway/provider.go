package aigateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/panicguard"
)

type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Schema      map[string]any `json:"schema"`
}

type ProviderRequest struct {
	Model       string           `json:"model"`
	Messages    []Message        `json:"messages"`
	Temperature *float64         `json:"temperature,omitempty"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
}

type ProviderResponse struct {
	ProviderRequestID string     `json:"provider_request_id"`
	Content           string     `json:"content"`
	Usage             TokenUsage `json:"usage"`
	ToolCalls         []ToolCall `json:"tool_calls,omitempty"`
}

type ToolCall struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

func (c ToolCall) normalizedID(index int) string {
	if c.ID != "" {
		return c.ID
	}
	return fmt.Sprintf("tool_call_%d", index+1)
}

type StreamEvent struct {
	Type     string     `json:"type"`
	Delta    string     `json:"delta,omitempty"`
	Usage    TokenUsage `json:"usage,omitempty"`
	ToolCall *ToolCall  `json:"tool_call,omitempty"`
	Error    string     `json:"error,omitempty"`
	Done     bool       `json:"done,omitempty"`
}

type ProviderAdapter interface {
	Invoke(ctx context.Context, req ProviderRequest) (*ProviderResponse, error)
	Stream(ctx context.Context, req ProviderRequest) (<-chan StreamEvent, error)
}

type ProviderError struct {
	Provider   string
	StatusCode int
	Body       string
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("%s provider returned HTTP %d: %s", e.Provider, e.StatusCode, e.Body)
}

func httpClientOrDefault(client *http.Client) *http.Client {
	if client != nil {
		return client
	}
	return &http.Client{Timeout: 60 * time.Second}
}

func joinEndpoint(baseURL string, path string) string {
	return strings.TrimRight(baseURL, "/") + path
}

func postProviderJSON(ctx context.Context, client *http.Client, endpoint string, headers map[string]string, payload any) (*http.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal provider request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create provider request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		httpReq.Header.Set(key, value)
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send provider request: %w", err)
	}
	return resp, nil
}

func decodeProviderJSON(provider string, resp *http.Response, out any) error {
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return providerHTTPError(provider, resp)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s provider response: %w", provider, err)
	}
	return nil
}

func providerHTTPError(provider string, resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	summary := strings.TrimSpace(string(body))
	if len(summary) > 512 {
		summary = summary[:512]
	}
	if summary == "" {
		summary = http.StatusText(resp.StatusCode)
	}
	return &ProviderError{Provider: provider, StatusCode: resp.StatusCode, Body: summary}
}

func parseToolArguments(raw string) map[string]any {
	if raw == "" {
		return map[string]any{}
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		return map[string]any{"raw": raw}
	}
	return args
}

func copyMap(input map[string]any) map[string]any {
	if input == nil {
		return map[string]any{}
	}
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func streamProviderEvents(ctx context.Context, body io.ReadCloser, convert func(RawSSEEvent) []StreamEvent) <-chan StreamEvent {
	out := make(chan StreamEvent)
	go func() {
		defer panicguard.Recover("aigateway provider stream")
		defer close(out)
		defer body.Close()

		done := false
		err := ScanSSE(body, func(raw RawSSEEvent) error {
			for _, event := range convert(raw) {
				if event.Done {
					if done {
						continue
					}
					done = true
				}
				if !sendStreamEvent(ctx, out, event) {
					return ctx.Err()
				}
			}
			return nil
		})
		if err != nil && ctx.Err() == nil {
			sendStreamEvent(ctx, out, StreamEvent{Type: "error", Error: err.Error()})
			return
		}
		if !done && ctx.Err() == nil {
			sendStreamEvent(ctx, out, StreamEvent{Type: "done", Done: true})
		}
	}()
	return out
}

func sendStreamEvent(ctx context.Context, out chan<- StreamEvent, event StreamEvent) bool {
	select {
	case <-ctx.Done():
		return false
	case out <- event:
		return true
	}
}
