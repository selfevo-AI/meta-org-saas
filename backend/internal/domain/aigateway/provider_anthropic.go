package aigateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type AnthropicAdapter struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

const defaultAnthropicBaseURL = "https://api.anthropic.com"

func NewAnthropicAdapter(baseURL string, apiKey string, client *http.Client) *AnthropicAdapter {
	if baseURL == "" {
		baseURL = defaultAnthropicBaseURL
	}
	return &AnthropicAdapter{
		baseURL: baseURL,
		apiKey:  apiKey,
		client:  httpClientOrDefault(client),
	}
}

func (a *AnthropicAdapter) Invoke(ctx context.Context, req ProviderRequest) (*ProviderResponse, error) {
	req, names := prepareProviderTools(req)
	payload := a.payload(req, false)
	resp, err := postProviderJSON(ctx, a.client, providerEndpoint(a.baseURL, "v1", "messages"), a.headers(), payload)
	if err != nil {
		return nil, err
	}

	var decoded anthropicMessageResponse
	if err := decodeProviderJSON("anthropic", resp, &decoded); err != nil {
		return nil, err
	}
	if decoded.Error != nil {
		return nil, fmt.Errorf("anthropic: %s", decoded.Error.Message)
	}
	if len(decoded.Content) == 0 {
		return nil, fmt.Errorf("anthropic response contains no content")
	}
	if decoded.StopReason == "max_tokens" {
		return nil, fmt.Errorf("anthropic generation exceeded max_tokens")
	}
	return restoreToolNames(decoded.toProviderResponse(), names), nil
}

func (a *AnthropicAdapter) Stream(ctx context.Context, req ProviderRequest) (<-chan StreamEvent, error) {
	req, names := prepareProviderTools(req)
	payload := a.payload(req, true)
	resp, err := postProviderJSON(ctx, a.client, providerEndpoint(a.baseURL, "v1", "messages"), a.headers(), payload)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		return nil, providerHTTPError("anthropic", resp)
	}
	state := &anthropicStreamState{calls: map[int]*anthropicPendingCall{}}
	return streamProviderEvents(ctx, resp.Body, restoreStreamToolNames(state.convert, names)), nil
}

func (a *AnthropicAdapter) headers() map[string]string {
	return map[string]string{
		"x-api-key":         a.apiKey,
		"anthropic-version": "2023-06-01",
	}
}

func (a *AnthropicAdapter) payload(req ProviderRequest, stream bool) map[string]any {
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1024
	}
	payload := map[string]any{
		"model":      req.Model,
		"messages":   anthropicMessages(req.Messages),
		"max_tokens": maxTokens,
		"stream":     stream,
	}
	if system := providerSystemPrompt(req.Messages); system != "" {
		payload["system"] = system
	}
	if req.Temperature != nil {
		payload["temperature"] = *req.Temperature
	}
	if len(req.Tools) > 0 {
		payload["tools"] = anthropicTools(req.Tools)
	}
	return payload
}

func anthropicMessages(messages []Message) []map[string]any {
	result := make([]map[string]any, 0, len(messages))
	for _, msg := range messages {
		if msg.Role == "system" || msg.Role == "developer" {
			continue
		}
		if msg.Role == "tool" {
			block := map[string]any{
				"type": "tool_result", "tool_use_id": msg.ToolCallID, "content": msg.Content,
			}
			if len(result) > 0 && result[len(result)-1]["role"] == "user" {
				if blocks, ok := result[len(result)-1]["content"].([]map[string]any); ok {
					result[len(result)-1]["content"] = append(blocks, block)
					continue
				}
			}
			result = append(result, map[string]any{
				"role":    "user",
				"content": []map[string]any{block},
			})
			continue
		}
		role := msg.Role
		if role == "assistant" {
			role = "assistant"
		}
		content := any(msg.Content)
		if len(msg.ToolCalls) > 0 || len(msg.Attachments) > 0 {
			blocks := make([]map[string]any, 0, len(msg.ToolCalls)+1)
			if msg.Content != "" {
				blocks = append(blocks, map[string]any{"type": "text", "text": msg.Content})
			}
			blocks = append(blocks, anthropicAttachments(msg.Attachments)...)
			for i, call := range msg.ToolCalls {
				blocks = append(blocks, map[string]any{
					"type":  "tool_use",
					"id":    call.normalizedID(i),
					"name":  call.Name,
					"input": copyMap(call.Arguments),
				})
			}
			content = blocks
		}
		result = append(result, map[string]any{
			"role":    role,
			"content": content,
		})
	}
	return result
}

func anthropicTools(tools []ToolDefinition) []map[string]any {
	result := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		result = append(result, map[string]any{
			"name":         tool.Name,
			"description":  tool.Description,
			"input_schema": copyMap(tool.Schema),
		})
	}
	return result
}

type anthropicMessageResponse struct {
	ID      string                  `json:"id"`
	Content []anthropicContentBlock `json:"content"`
	Usage   struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	StopReason string `json:"stop_reason"`
	Error      *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type anthropicContentBlock struct {
	Type  string         `json:"type"`
	Text  string         `json:"text"`
	ID    string         `json:"id"`
	Name  string         `json:"name"`
	Input map[string]any `json:"input"`
}

func (r anthropicMessageResponse) toProviderResponse() *ProviderResponse {
	resp := &ProviderResponse{
		ProviderRequestID: r.ID,
		Usage: TokenUsage{
			InputTokens:  r.Usage.InputTokens,
			OutputTokens: r.Usage.OutputTokens,
		},
	}
	for _, block := range r.Content {
		switch block.Type {
		case "text":
			resp.Content += block.Text
		case "tool_use":
			resp.ToolCalls = append(resp.ToolCalls, ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: copyMap(block.Input),
			})
		}
	}
	return resp
}

type anthropicPendingCall struct {
	call      ToolCall
	arguments strings.Builder
}

type anthropicStreamState struct {
	calls map[int]*anthropicPendingCall
}

func (s *anthropicStreamState) convert(raw RawSSEEvent) []StreamEvent {
	if raw.Event == "eof" || raw.Done {
		return []StreamEvent{{Type: "error", Error: "anthropic stream ended before message_stop"}}
	}
	var event anthropicStreamEvent
	if err := json.Unmarshal([]byte(raw.Data), &event); err != nil {
		return []StreamEvent{{Type: "error", Error: fmt.Sprintf("decode anthropic stream: %v", err)}}
	}
	eventType := event.Type
	if eventType == "" {
		eventType = raw.Event
	}
	switch eventType {
	case "message_start":
		return []StreamEvent{{Type: "usage", Usage: TokenUsage{InputTokens: event.Message.Usage.InputTokens, OutputTokens: event.Message.Usage.OutputTokens}}}
	case "content_block_start":
		if event.ContentBlock.Type == "tool_use" {
			if len(s.calls) >= 128 {
				return []StreamEvent{{Type: "error", Error: "anthropic tool output exceeds limits"}}
			}
			s.calls[event.Index] = &anthropicPendingCall{call: ToolCall{ID: event.ContentBlock.ID, Name: event.ContentBlock.Name, Arguments: copyMap(event.ContentBlock.Input)}}
		}
	case "content_block_delta":
		if event.Delta.Type == "input_json_delta" {
			call := s.calls[event.Index]
			if call == nil {
				return []StreamEvent{{Type: "error", Error: "anthropic tool delta has no start"}}
			}
			call.arguments.WriteString(event.Delta.PartialJSON)
			if call.arguments.Len() > 1024*1024 {
				return []StreamEvent{{Type: "error", Error: "anthropic tool arguments exceed limits"}}
			}
		}
		if event.Delta.Text != "" {
			return []StreamEvent{{Type: "delta", Delta: event.Delta.Text}}
		}
	case "content_block_stop":
		if pending := s.calls[event.Index]; pending != nil {
			call := pending.call
			if call.ID == "" || call.Name == "" {
				return []StreamEvent{{Type: "error", Error: "incomplete anthropic tool call"}}
			}
			if pending.arguments.Len() > 0 {
				args, err := parseToolArguments(pending.arguments.String())
				if err != nil {
					return []StreamEvent{{Type: "error", Error: err.Error()}}
				}
				call.Arguments = args
			}
			delete(s.calls, event.Index)
			return []StreamEvent{{Type: "tool_call", ToolCall: &call}}
		}
	case "message_delta":
		if event.Delta.StopReason == "max_tokens" {
			return []StreamEvent{{Type: "error", Error: "anthropic generation exceeded max_tokens"}}
		}
		if event.Usage.OutputTokens > 0 {
			return []StreamEvent{{Type: "usage", Usage: TokenUsage{OutputTokens: event.Usage.OutputTokens}}}
		}
	case "message_stop":
		if len(s.calls) != 0 {
			return []StreamEvent{{Type: "error", Error: "anthropic stream contains incomplete tool calls"}}
		}
		return []StreamEvent{{Type: "done", Done: true}}
	case "error":
		if event.Error.Message != "" {
			return []StreamEvent{{Type: "error", Error: event.Error.Message}}
		}
	}
	return nil
}

type anthropicStreamEvent struct {
	Type         string                   `json:"type"`
	Index        int                      `json:"index"`
	ContentBlock anthropicContentBlock    `json:"content_block"`
	Message      anthropicMessageResponse `json:"message"`
	Delta        struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		PartialJSON string `json:"partial_json"`
		StopReason  string `json:"stop_reason"`
	} `json:"delta"`
	Usage struct {
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}
