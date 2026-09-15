package aigateway

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
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
	ReasoningContent  string     `json:"reasoning_content,omitempty"`
}

type ToolCall struct {
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	Arguments        map[string]any `json:"arguments"`
	ThoughtSignature string         `json:"thought_signature,omitempty"`
}

func (c ToolCall) normalizedID(index int) string {
	if c.ID != "" {
		return c.ID
	}
	return fmt.Sprintf("tool_call_%d", index+1)
}

type StreamEvent struct {
	Type           string     `json:"type"`
	Delta          string     `json:"delta,omitempty"`
	ReasoningDelta string     `json:"reasoning_delta,omitempty"`
	Usage          TokenUsage `json:"usage,omitempty"`
	ToolCall       *ToolCall  `json:"tool_call,omitempty"`
	Error          string     `json:"error,omitempty"`
	Done           bool       `json:"done,omitempty"`
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

// A configured path is an API root (for example /compatible-mode/v1).
// Only bare origins receive the protocol's default version.
func providerEndpoint(baseURL, version, resource string) string {
	u, err := url.Parse(baseURL)
	if err != nil {
		return baseURL
	}
	root := strings.TrimRight(u.Path, "/")
	if strings.HasSuffix(root, "/"+resource) {
		return u.String()
	}
	if root == "" {
		root = "/" + version
	}
	u.Path = root + "/" + resource
	u.RawPath = ""
	return u.String()
}

var providerToolName = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// Keep internal tool names stable while satisfying all three wire protocols.
func prepareProviderTools(req ProviderRequest) (ProviderRequest, map[string]string) {
	names, reverse := map[string]string{}, map[string]string{}
	encode := func(name string) string {
		if name == "" {
			return ""
		}
		if encoded, ok := names[name]; ok {
			return encoded
		}
		encoded := name
		if !providerToolName.MatchString(name) || strings.HasPrefix(name, "wire_") {
			encoded = fmt.Sprintf("wire_%x", sha256.Sum256([]byte(name)))[:61]
		}
		names[name], reverse[encoded] = encoded, name
		return encoded
	}

	req.Tools = append([]ToolDefinition(nil), req.Tools...)
	for i := range req.Tools {
		req.Tools[i].Name = encode(req.Tools[i].Name)
	}
	req.Messages = append([]Message(nil), req.Messages...)
	for i := range req.Messages {
		message := &req.Messages[i]
		message.ToolName = encode(message.ToolName)
		message.ToolCalls = append([]ToolCall(nil), message.ToolCalls...)
		for j := range message.ToolCalls {
			message.ToolCalls[j].Name = encode(message.ToolCalls[j].Name)
		}
	}
	return req, reverse
}

func restoreToolNames(resp *ProviderResponse, names map[string]string) *ProviderResponse {
	for i := range resp.ToolCalls {
		if name, ok := names[resp.ToolCalls[i].Name]; ok {
			resp.ToolCalls[i].Name = name
		}
	}
	return resp
}

func restoreStreamToolNames(convert func(RawSSEEvent) []StreamEvent, names map[string]string) func(RawSSEEvent) []StreamEvent {
	return func(raw RawSSEEvent) []StreamEvent {
		events := convert(raw)
		for i := range events {
			if call := events[i].ToolCall; call != nil {
				if name, ok := names[call.Name]; ok {
					call.Name = name
				}
			}
		}
		return events
	}
}

func providerSystemPrompt(messages []Message) string {
	parts := []string{}
	for _, message := range messages {
		if message.Role == "system" || message.Role == "developer" {
			parts = append(parts, message.Content)
		}
	}
	return strings.Join(parts, "\n\n")
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
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
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

func parseToolArguments(raw string) (map[string]any, error) {
	if raw == "" {
		return map[string]any{}, nil
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		return nil, fmt.Errorf("invalid tool arguments: %w", err)
	}
	if args == nil {
		return nil, fmt.Errorf("tool arguments must be a JSON object")
	}
	return args, nil
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
		streamFailed := errors.New("provider stream failed")
		emit := func(raw RawSSEEvent) error {
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
				if event.Error != "" {
					return streamFailed
				}
			}
			return nil
		}
		err := ScanSSE(body, emit)
		if err != nil && !errors.Is(err, streamFailed) && ctx.Err() == nil {
			sendStreamEvent(ctx, out, StreamEvent{Type: "error", Error: err.Error()})
			return
		}
		if err == nil && !done && ctx.Err() == nil {
			_ = emit(RawSSEEvent{Event: "eof"})
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
