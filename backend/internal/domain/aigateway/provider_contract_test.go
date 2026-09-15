package aigateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestProviderEndpointsPreserveConfiguredAPIRoot(t *testing.T) {
	for _, test := range []struct{ base, version, resource, want string }{
		{"https://example.test", "v1", "chat/completions", "https://example.test/v1/chat/completions"},
		{"https://example.test/v1/", "v1", "chat/completions", "https://example.test/v1/chat/completions"},
		{"https://example.test/api/v1", "v1", "chat/completions", "https://example.test/api/v1/chat/completions"},
		{"https://example.test/compatible-mode/v1", "v1", "chat/completions", "https://example.test/compatible-mode/v1/chat/completions"},
		{"https://example.test/api/v3", "v1", "chat/completions", "https://example.test/api/v3/chat/completions"},
		{"https://example.test/v1/chat/completions?api-version=test", "v1", "chat/completions", "https://example.test/v1/chat/completions?api-version=test"},
		{"https://example.test/v1", "v1", "messages", "https://example.test/v1/messages"},
	} {
		t.Run(test.base+test.resource, func(t *testing.T) {
			if got := providerEndpoint(test.base, test.version, test.resource); got != test.want {
				t.Fatalf("endpoint = %s, want %s", got, test.want)
			}
		})
	}
}

func TestProviderToolNamesRoundTripWithoutMutatingRequest(t *testing.T) {
	request := ProviderRequest{
		Tools: []ToolDefinition{{Name: "erp.action.execute"}, {Name: "erp_action_execute"}, {Name: "wire_literal"}},
		Messages: []Message{
			{Role: "assistant", ToolCalls: []ToolCall{{ID: "call", Name: "erp.action.execute"}}},
			{Role: "tool", ToolName: "erp.action.execute", ToolCallID: "call"},
		},
	}
	prepared, names := prepareProviderTools(request)
	if request.Tools[0].Name != "erp.action.execute" || request.Messages[0].ToolCalls[0].Name != "erp.action.execute" {
		t.Fatal("request was mutated")
	}
	seen := map[string]bool{}
	for _, tool := range prepared.Tools {
		if !providerToolName.MatchString(tool.Name) || seen[tool.Name] {
			t.Fatalf("invalid or colliding tool name %s", tool.Name)
		}
		seen[tool.Name] = true
	}
	if prepared.Messages[0].ToolCalls[0].Name != prepared.Tools[0].Name || prepared.Messages[1].ToolName != prepared.Tools[0].Name {
		t.Fatal("history tool names do not match definitions")
	}
	response := restoreToolNames(&ProviderResponse{ToolCalls: []ToolCall{{Name: prepared.Tools[0].Name}}}, names)
	if response.ToolCalls[0].Name != "erp.action.execute" {
		t.Fatal("name not restored")
	}
}

func TestProviderPayloadsPreserveSystemAndToolHistory(t *testing.T) {
	request := ProviderRequest{Model: "test", Messages: []Message{
		{Role: "system", Content: "Only use approved actions."},
		{Role: "developer", Content: "Use the ontology."},
		{Role: "user", Content: "Get an order."},
		{Role: "assistant", ToolCalls: []ToolCall{
			{ID: "c1", Name: "read", Arguments: map[string]any{"key": "PO"}, ThoughtSignature: "opaque-signature"},
			{ID: "c2", Name: "read", Arguments: map[string]any{"key": "SO"}},
		}, ReasoningContent: "provider continuation"},
		{Role: "tool", ToolCallID: "c1", ToolName: "read", Content: `{"key":"PO"}`},
		{Role: "tool", ToolCallID: "c2", ToolName: "read", Content: `{"key":"SO"}`},
	}}
	anthropic := NewAnthropicAdapter("", "", nil).payload(request, false)
	if anthropic["system"] != "Only use approved actions.\n\nUse the ontology." {
		t.Fatalf("system = %v", anthropic["system"])
	}
	ams := anthropic["messages"].([]map[string]any)
	if len(ams) != 3 || len(ams[2]["content"].([]map[string]any)) != 2 {
		t.Fatalf("tool results not grouped: %#v", ams)
	}
	gemini := NewGeminiAdapter("", "", nil).payload(request)
	if gemini["systemInstruction"] == nil {
		t.Fatal("missing Gemini system instruction")
	}
	contents := gemini["contents"].([]map[string]any)
	if len(contents) != 3 {
		t.Fatalf("contents = %#v", contents)
	}
	parts := contents[1]["parts"].([]map[string]any)
	if parts[0]["thoughtSignature"] != "opaque-signature" {
		t.Fatal("signature was dropped")
	}
	if parts[0]["functionCall"].(map[string]any)["id"] != "c1" {
		t.Fatal("call ID was dropped")
	}
	openai := openAIMessages(request.Messages)
	if openai[3]["reasoning_content"] != "provider continuation" {
		t.Fatal("reasoning continuation was dropped")
	}
}

func TestOpenAIStreamAssemblesParallelToolCallsAndUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/compatible-mode/v1/chat/completions" {
			t.Errorf("path = %s", r.URL.Path)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
			return
		}
		if payload["stream_options"].(map[string]any)["include_usage"] != true {
			t.Error("usage not requested")
		}
		tools := payload["tools"].([]any)
		name := tools[0].(map[string]any)["function"].(map[string]any)["name"].(string)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call1\",\"function\":{\"name\":%q,\"arguments\":\"{\\\"key\\\":\"}},{\"index\":1,\"id\":\"call2\",\"function\":{\"name\":%q,\"arguments\":\"{\"}}]}}]}\n\n", name, name)
		fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":1,\"function\":{\"arguments\":\"\\\"key\\\":\\\"SO\\\"}\"}},{\"index\":0,\"function\":{\"arguments\":\"\\\"PO\\\"}\"}}]},\"finish_reason\":\"tool_calls\"}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":15,\"completion_tokens\":7}}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	events, err := NewOpenAIAdapter(server.URL+"/compatible-mode/v1", "test", server.Client()).Stream(context.Background(), ProviderRequest{
		Model: "test", Messages: []Message{{Role: "user", Content: "orders"}}, Tools: []ToolDefinition{{Name: "ontology.objects.get", Schema: map[string]any{"type": "object"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var calls []ToolCall
	var usage TokenUsage
	collected := collectStreamEvents(events)
	for _, event := range collected {
		if event.Error != "" {
			t.Fatal(event.Error)
		}
		if event.ToolCall != nil {
			calls = append(calls, *event.ToolCall)
		}
		usage = mergeTokenUsage(usage, event.Usage)
	}
	if len(calls) != 2 || calls[0].ID != "call1" || calls[1].ID != "call2" || calls[0].Arguments["key"] != "PO" || calls[1].Arguments["key"] != "SO" {
		t.Fatalf("calls = %#v", calls)
	}
	if calls[0].Name != "ontology.objects.get" || calls[1].Name != "ontology.objects.get" {
		t.Fatal("wire names leaked")
	}
	if usage.InputTokens != 15 || usage.OutputTokens != 7 || doneEventCount(collected) != 1 {
		t.Fatalf("usage/events = %#v / %#v", usage, collected)
	}
}

func TestAnthropicStreamAssemblesToolArgumentsAndInputUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"type":"message_start","message":{"usage":{"input_tokens":12,"output_tokens":1}}}

data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"call1","name":"read","input":{}}}

data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"key\":"}}

data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"\"PO\"}"}}

data: {"type":"content_block_stop","index":0}

data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":9}}

data: {"type":"message_stop"}

`)
	}))
	defer server.Close()
	events, err := NewAnthropicAdapter(server.URL+"/v1", "test", server.Client()).Stream(context.Background(), ProviderRequest{Model: "test"})
	if err != nil {
		t.Fatal(err)
	}
	var usage TokenUsage
	var calls []ToolCall
	for event := range events {
		if event.Error != "" {
			t.Fatal(event.Error)
		}
		usage = mergeTokenUsage(usage, event.Usage)
		if event.ToolCall != nil {
			calls = append(calls, *event.ToolCall)
		}
	}
	if usage.InputTokens != 12 || usage.OutputTokens != 9 {
		t.Fatalf("usage = %#v", usage)
	}
	if len(calls) != 1 || calls[0].Arguments["key"] != "PO" {
		t.Fatalf("calls = %#v", calls)
	}
}

func TestGeminiInvokePreservesSignatureAndUsesHeaderCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1beta/models/gemini-test:generateContent" || strings.Contains(r.URL.String(), "secret") || r.Header.Get("x-goog-api-key") != "secret" {
			t.Errorf("incorrect endpoint or credentials: %s", r.URL)
		}
		fmt.Fprint(w, `{"candidates":[{"content":{"parts":[{"thought":true,"text":"private continuation"},{"thoughtSignature":"signature","functionCall":{"id":"call1","name":"read","args":{"key":"PO"}}}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":2,"candidatesTokenCount":3,"thoughtsTokenCount":4}}`)
	}))
	defer server.Close()
	resp, err := NewGeminiAdapter(server.URL+"/v1beta", "secret", server.Client()).Invoke(context.Background(), ProviderRequest{Model: "models/gemini-test"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "" || len(resp.ToolCalls) != 1 || resp.ToolCalls[0].ThoughtSignature != "signature" || resp.ToolCalls[0].ID != "call1" || resp.Usage.OutputTokens != 7 {
		t.Fatalf("response = %#v", resp)
	}
}

func TestProvidersRejectTruncatedStreamsAndErrorEnvelopes(t *testing.T) {
	for _, test := range []struct {
		name, body string
		adapter    func(string, string, *http.Client) ProviderAdapter
	}{
		{"openai-truncated", `data: {"choices":[{"delta":{"content":"partial"}}]}` + "\n\n", func(u, k string, c *http.Client) ProviderAdapter { return NewOpenAIAdapter(u, k, c) }},
		{"openai-error", `data: {"error":{"message":"upstream failure"}}` + "\n\n", func(u, k string, c *http.Client) ProviderAdapter { return NewOpenAIAdapter(u, k, c) }},
		{"anthropic-truncated", `data: {"type":"content_block_delta","delta":{"text":"partial"}}` + "\n\n", func(u, k string, c *http.Client) ProviderAdapter { return NewAnthropicAdapter(u, k, c) }},
		{"gemini-blocked", `data: {"promptFeedback":{"blockReason":"SAFETY"}}` + "\n\n", func(u, k string, c *http.Client) ProviderAdapter { return NewGeminiAdapter(u, k, c) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, test.body) }))
			defer server.Close()
			ch, err := test.adapter(server.URL, "test", server.Client()).Stream(context.Background(), ProviderRequest{Model: "test"})
			if err != nil {
				t.Fatal(err)
			}
			events := collectStreamEvents(ch)
			failed := false
			for _, event := range events {
				failed = failed || event.Error != ""
				if event.ToolCall != nil {
					t.Fatal("incomplete tool emitted")
				}
			}
			if !failed || hasDoneEvent(events) {
				t.Fatalf("unexpected successful stream: %#v", events)
			}
		})
	}
}

func TestToolArgumentsMustBeCompleteObjects(t *testing.T) {
	for _, raw := range []string{`{"key":`, `null`, `[]`, `"value"`} {
		if _, err := parseToolArguments(raw); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	if value, err := parseToolArguments(`{"key":"PO"}`); err != nil || !reflect.DeepEqual(value, map[string]any{"key": "PO"}) {
		t.Fatalf("value = %#v, %v", value, err)
	}
}
