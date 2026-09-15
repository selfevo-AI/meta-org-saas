package aigateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

type OpenAIAdapter struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

const defaultOpenAIBaseURL = "https://api.openai.com"

func NewOpenAIAdapter(baseURL string, apiKey string, client *http.Client) *OpenAIAdapter {
	if baseURL == "" {
		baseURL = defaultOpenAIBaseURL
	}
	return &OpenAIAdapter{
		baseURL: baseURL,
		apiKey:  apiKey,
		client:  httpClientOrDefault(client),
	}
}

func (a *OpenAIAdapter) Invoke(ctx context.Context, req ProviderRequest) (*ProviderResponse, error) {
	req, names := prepareProviderTools(req)
	payload := a.payload(req, false)
	resp, err := postProviderJSON(ctx, a.client, providerEndpoint(a.baseURL, "v1", "chat/completions"), a.headers(), payload)
	if err != nil {
		return nil, err
	}

	var decoded openAIChatResponse
	if err := decodeProviderJSON("openai", resp, &decoded); err != nil {
		return nil, err
	}
	result, err := decoded.toProviderResponse()
	if err != nil {
		return nil, err
	}
	return restoreToolNames(result, names), nil
}

func (a *OpenAIAdapter) Stream(ctx context.Context, req ProviderRequest) (<-chan StreamEvent, error) {
	req, names := prepareProviderTools(req)
	payload := a.payload(req, true)
	resp, err := postProviderJSON(ctx, a.client, providerEndpoint(a.baseURL, "v1", "chat/completions"), a.headers(), payload)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		return nil, providerHTTPError("openai", resp)
	}
	state := &openAIStreamState{calls: map[int]*openAIToolCall{}}
	return streamProviderEvents(ctx, resp.Body, restoreStreamToolNames(state.convert, names)), nil
}

func (a *OpenAIAdapter) headers() map[string]string {
	return map[string]string{
		"Authorization": "Bearer " + a.apiKey,
	}
}

func (a *OpenAIAdapter) payload(req ProviderRequest, stream bool) map[string]any {
	payload := map[string]any{
		"model":    req.Model,
		"messages": openAIMessages(req.Messages),
		"stream":   stream,
	}
	if req.Temperature != nil {
		payload["temperature"] = *req.Temperature
	}
	if req.MaxTokens > 0 {
		if strings.HasPrefix(req.Model, "gpt-5") || strings.HasPrefix(req.Model, "o1") || strings.HasPrefix(req.Model, "o3") || strings.HasPrefix(req.Model, "o4") {
			payload["max_completion_tokens"] = req.MaxTokens
		} else {
			payload["max_tokens"] = req.MaxTokens
		}
	}
	if stream {
		payload["stream_options"] = map[string]any{"include_usage": true}
	}
	if len(req.Tools) > 0 {
		payload["tools"] = openAITools(req.Tools)
	}
	return payload
}

func openAIMessages(messages []Message) []map[string]any {
	result := make([]map[string]any, 0, len(messages))
	for _, msg := range messages {
		item := map[string]any{"role": msg.Role}
		switch msg.Role {
		case "tool":
			item["content"] = msg.Content
			item["tool_call_id"] = msg.ToolCallID
		default:
			item["content"] = openAIContent(msg)
			if msg.ReasoningContent != "" {
				item["reasoning_content"] = msg.ReasoningContent
			}
			if len(msg.ToolCalls) > 0 {
				calls := make([]map[string]any, 0, len(msg.ToolCalls))
				for i, call := range msg.ToolCalls {
					args, _ := json.Marshal(call.Arguments)
					calls = append(calls, map[string]any{
						"id":   call.normalizedID(i),
						"type": "function",
						"function": map[string]any{
							"name":      call.Name,
							"arguments": string(args),
						},
					})
				}
				item["tool_calls"] = calls
			}
		}
		result = append(result, item)
	}
	return result
}

func openAITools(tools []ToolDefinition) []map[string]any {
	result := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		result = append(result, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        tool.Name,
				"description": tool.Description,
				"parameters":  copyMap(tool.Schema),
			},
		})
	}
	return result
}

type openAIChatResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Message struct {
			Content          string           `json:"content"`
			ReasoningContent string           `json:"reasoning_content"`
			ToolCalls        []openAIToolCall `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage openAIUsage `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

type openAIToolCall struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func (r openAIChatResponse) toProviderResponse() (*ProviderResponse, error) {
	if r.Error != nil {
		return nil, fmt.Errorf("openai: %s", r.Error.Message)
	}
	if len(r.Choices) == 0 {
		return nil, fmt.Errorf("openai response contains no choices")
	}
	resp := &ProviderResponse{
		ProviderRequestID: r.ID,
		Usage: TokenUsage{
			InputTokens:  r.Usage.PromptTokens,
			OutputTokens: r.Usage.CompletionTokens,
		},
	}
	for _, choice := range r.Choices[:1] {
		if choice.FinishReason == "content_filter" || (choice.FinishReason == "length" && len(choice.Message.ToolCalls) > 0) {
			return nil, fmt.Errorf("openai generation incomplete: %s", choice.FinishReason)
		}
		resp.Content += choice.Message.Content
		resp.ReasoningContent += choice.Message.ReasoningContent
		for _, toolCall := range choice.Message.ToolCalls {
			call, err := toolCall.toToolCall()
			if err != nil {
				return nil, err
			}
			resp.ToolCalls = append(resp.ToolCalls, call)
		}
	}
	return resp, nil
}

func (c openAIToolCall) toToolCall() (ToolCall, error) {
	if c.ID == "" || c.Function.Name == "" {
		return ToolCall{}, fmt.Errorf("incomplete openai tool call")
	}
	args, err := parseToolArguments(c.Function.Arguments)
	if err != nil {
		return ToolCall{}, err
	}
	return ToolCall{
		ID:        c.ID,
		Name:      c.Function.Name,
		Arguments: args,
	}, nil
}

type openAIStreamState struct {
	calls    map[int]*openAIToolCall
	finished bool
}

func (s *openAIStreamState) convert(raw RawSSEEvent) []StreamEvent {
	if raw.Done || raw.Event == "eof" {
		if !s.finished {
			return []StreamEvent{{Type: "error", Error: "openai stream ended before completion"}}
		}
		return []StreamEvent{{Type: "done", Done: true}}
	}
	var chunk openAIStreamChunk
	if err := json.Unmarshal([]byte(raw.Data), &chunk); err != nil {
		return []StreamEvent{{Type: "error", Error: fmt.Sprintf("decode openai stream: %v", err)}}
	}
	if chunk.Error != nil {
		return []StreamEvent{{Type: "error", Error: chunk.Error.Message}}
	}
	events := []StreamEvent{}
	for _, choice := range chunk.Choices {
		if choice.Index != 0 {
			continue
		}
		if choice.Delta.Content != "" {
			events = append(events, StreamEvent{Type: "delta", Delta: choice.Delta.Content})
		}
		if choice.Delta.ReasoningContent != "" {
			events = append(events, StreamEvent{Type: "reasoning", ReasoningDelta: choice.Delta.ReasoningContent})
		}
		for _, toolCall := range choice.Delta.ToolCalls {
			call := s.calls[toolCall.Index]
			if call == nil {
				call = &openAIToolCall{}
				s.calls[toolCall.Index] = call
			}
			call.ID += toolCall.ID
			call.Function.Name += toolCall.Function.Name
			call.Function.Arguments += toolCall.Function.Arguments
			if len(call.Function.Arguments) > 1024*1024 || len(s.calls) > 128 {
				return []StreamEvent{{Type: "error", Error: "openai tool output exceeds limits"}}
			}
		}
		if choice.FinishReason != "" && !s.finished {
			if choice.FinishReason != "stop" && choice.FinishReason != "tool_calls" {
				return []StreamEvent{{Type: "error", Error: "openai generation incomplete: " + choice.FinishReason}}
			}
			indices := make([]int, 0, len(s.calls))
			for index := range s.calls {
				indices = append(indices, index)
			}
			sort.Ints(indices)
			for _, index := range indices {
				call, err := s.calls[index].toToolCall()
				if err != nil {
					return []StreamEvent{{Type: "error", Error: err.Error()}}
				}
				events = append(events, StreamEvent{Type: "tool_call", ToolCall: &call})
			}
			s.finished = true
		}
	}
	if chunk.Usage.PromptTokens > 0 || chunk.Usage.CompletionTokens > 0 {
		events = append(events, StreamEvent{Type: "usage", Usage: TokenUsage{InputTokens: chunk.Usage.PromptTokens, OutputTokens: chunk.Usage.CompletionTokens}})
	}
	return events
}

type openAIStreamChunk struct {
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Content          string           `json:"content"`
			ReasoningContent string           `json:"reasoning_content"`
			ToolCalls        []openAIToolCall `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage openAIUsage `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}
