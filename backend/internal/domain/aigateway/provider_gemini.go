package aigateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type GeminiAdapter struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

const defaultGeminiBaseURL = "https://generativelanguage.googleapis.com"

func NewGeminiAdapter(baseURL string, apiKey string, client *http.Client) *GeminiAdapter {
	if baseURL == "" {
		baseURL = defaultGeminiBaseURL
	}
	return &GeminiAdapter{
		baseURL: baseURL,
		apiKey:  apiKey,
		client:  httpClientOrDefault(client),
	}
}

func (a *GeminiAdapter) Invoke(ctx context.Context, req ProviderRequest) (*ProviderResponse, error) {
	req, names := prepareProviderTools(req)
	resp, err := postProviderJSON(ctx, a.client, a.endpoint(req.Model, false), map[string]string{"x-goog-api-key": a.apiKey}, a.payload(req))
	if err != nil {
		return nil, err
	}

	var decoded geminiResponse
	if err := decodeProviderJSON("gemini", resp, &decoded); err != nil {
		return nil, err
	}
	if err := decoded.validate(); err != nil {
		return nil, err
	}
	if len(decoded.Candidates) == 0 {
		return nil, fmt.Errorf("gemini response contains no candidates")
	}
	providerResp := restoreToolNames(decoded.toProviderResponse(), names)
	providerResp.ProviderRequestID = resp.Header.Get("x-request-id")
	return providerResp, nil
}

func (a *GeminiAdapter) Stream(ctx context.Context, req ProviderRequest) (<-chan StreamEvent, error) {
	req, names := prepareProviderTools(req)
	resp, err := postProviderJSON(ctx, a.client, a.endpoint(req.Model, true), map[string]string{"x-goog-api-key": a.apiKey}, a.payload(req))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		return nil, providerHTTPError("gemini", resp)
	}
	state := &geminiStreamState{}
	return streamProviderEvents(ctx, resp.Body, restoreStreamToolNames(state.convert, names)), nil
}

func (a *GeminiAdapter) endpoint(model string, stream bool) string {
	action := "generateContent"
	if stream {
		action = "streamGenerateContent"
	}
	endpoint := providerEndpoint(a.baseURL, "v1beta", "models/"+strings.TrimPrefix(model, "models/")+":"+action)
	u, err := url.Parse(endpoint)
	if err != nil {
		return endpoint
	}
	values := u.Query()
	values.Del("key")
	if stream {
		values.Set("alt", "sse")
	}
	u.RawQuery = values.Encode()
	return u.String()
}

func (a *GeminiAdapter) payload(req ProviderRequest) map[string]any {
	payload := map[string]any{
		"contents": geminiContents(req.Messages),
	}
	if system := providerSystemPrompt(req.Messages); system != "" {
		payload["systemInstruction"] = map[string]any{"parts": []map[string]any{{"text": system}}}
	}
	if req.Temperature != nil || req.MaxTokens > 0 {
		config := map[string]any{}
		if req.Temperature != nil {
			config["temperature"] = *req.Temperature
		}
		if req.MaxTokens > 0 {
			config["maxOutputTokens"] = req.MaxTokens
		}
		payload["generationConfig"] = config
	}
	if len(req.Tools) > 0 {
		payload["tools"] = geminiTools(req.Tools)
	}
	return payload
}

func geminiContents(messages []Message) []map[string]any {
	result := make([]map[string]any, 0, len(messages))
	for _, msg := range messages {
		if msg.Role == "system" || msg.Role == "developer" {
			continue
		}
		role := "user"
		if msg.Role == "assistant" || msg.Role == "model" {
			role = "model"
		}
		parts := []map[string]any{}
		if msg.Role == "tool" {
			name := msg.ToolName
			if name == "" {
				name = msg.ToolCallID
			}
			parts = append(parts, map[string]any{
				"functionResponse": map[string]any{
					"name":     name,
					"id":       msg.ToolCallID,
					"response": map[string]any{"result": msg.Content},
				},
			})
		} else {
			if msg.Content != "" {
				parts = append(parts, map[string]any{"text": msg.Content})
			}
			parts = append(parts, geminiAttachments(msg.Attachments)...)
			for index, call := range msg.ToolCalls {
				part := map[string]any{
					"functionCall": map[string]any{
						"name": call.Name,
						"id":   call.normalizedID(index),
						"args": copyMap(call.Arguments),
					},
				}
				if call.ThoughtSignature != "" {
					part["thoughtSignature"] = call.ThoughtSignature
				}
				parts = append(parts, part)
			}
		}
		if len(parts) == 0 {
			parts = append(parts, map[string]any{"text": ""})
		}
		if len(result) > 0 && result[len(result)-1]["role"] == role {
			result[len(result)-1]["parts"] = append(result[len(result)-1]["parts"].([]map[string]any), parts...)
			continue
		}
		result = append(result, map[string]any{
			"role":  role,
			"parts": parts,
		})
	}
	return result
}

func geminiTools(tools []ToolDefinition) []map[string]any {
	declarations := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		declarations = append(declarations, map[string]any{
			"name":        tool.Name,
			"description": tool.Description,
			"parameters":  copyMap(tool.Schema),
		})
	}
	return []map[string]any{{"functionDeclarations": declarations}}
}

type geminiResponse struct {
	Candidates []struct {
		Content      geminiContent `json:"content"`
		FinishReason string        `json:"finishReason"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
		ThoughtsTokenCount   int `json:"thoughtsTokenCount"`
	} `json:"usageMetadata"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
	PromptFeedback struct {
		BlockReason string `json:"blockReason"`
	} `json:"promptFeedback"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text             string `json:"text"`
	Thought          bool   `json:"thought"`
	ThoughtSignature string `json:"thoughtSignature"`
	FunctionCall     *struct {
		ID   string         `json:"id"`
		Name string         `json:"name"`
		Args map[string]any `json:"args"`
	} `json:"functionCall"`
}

func (r geminiResponse) toProviderResponse() *ProviderResponse {
	resp := &ProviderResponse{
		Usage: TokenUsage{
			InputTokens:  r.UsageMetadata.PromptTokenCount,
			OutputTokens: r.UsageMetadata.CandidatesTokenCount + r.UsageMetadata.ThoughtsTokenCount,
		},
	}
	for index, candidate := range r.Candidates {
		if index != 0 {
			break
		}
		for _, part := range candidate.Content.Parts {
			if !part.Thought {
				resp.Content += part.Text
			}
			if part.FunctionCall != nil {
				resp.ToolCalls = append(resp.ToolCalls, ToolCall{
					ID:               part.FunctionCall.ID,
					Name:             part.FunctionCall.Name,
					Arguments:        copyMap(part.FunctionCall.Args),
					ThoughtSignature: part.ThoughtSignature,
				})
			}
		}
	}
	return resp
}

func (r geminiResponse) validate() error {
	if r.Error != nil {
		return fmt.Errorf("gemini: %s", r.Error.Message)
	}
	if r.PromptFeedback.BlockReason != "" {
		return fmt.Errorf("gemini prompt blocked: %s", r.PromptFeedback.BlockReason)
	}
	for _, candidate := range r.Candidates {
		if candidate.FinishReason != "" && candidate.FinishReason != "STOP" {
			return fmt.Errorf("gemini generation incomplete: %s", candidate.FinishReason)
		}
	}
	return nil
}

type geminiStreamState struct{ finished bool }

func (s *geminiStreamState) convert(raw RawSSEEvent) []StreamEvent {
	if raw.Event == "eof" || raw.Done {
		if !s.finished {
			return []StreamEvent{{Type: "error", Error: "gemini stream ended before completion"}}
		}
		return []StreamEvent{{Type: "done", Done: true}}
	}
	var chunk geminiResponse
	if err := json.Unmarshal([]byte(raw.Data), &chunk); err != nil {
		return []StreamEvent{{Type: "error", Error: fmt.Sprintf("decode gemini stream: %v", err)}}
	}
	if err := chunk.validate(); err != nil {
		return []StreamEvent{{Type: "error", Error: err.Error()}}
	}
	for _, candidate := range chunk.Candidates {
		if candidate.FinishReason == "STOP" {
			s.finished = true
		}
	}
	events := []StreamEvent{}
	resp := chunk.toProviderResponse()
	if resp.Content != "" {
		events = append(events, StreamEvent{Type: "delta", Delta: resp.Content})
	}
	for _, toolCall := range resp.ToolCalls {
		call := toolCall
		events = append(events, StreamEvent{Type: "tool_call", ToolCall: &call})
	}
	if resp.Usage.InputTokens > 0 || resp.Usage.OutputTokens > 0 {
		events = append(events, StreamEvent{Type: "usage", Usage: resp.Usage})
	}
	return events
}
