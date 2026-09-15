package aigateway

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAdaptersSendImagesAndPDFsWithoutDroppingText(t *testing.T) {
	for _, kind := range []string{ProviderOpenAI, ProviderAnthropic, ProviderGemini} {
		t.Run(kind, func(t *testing.T) {
			var request map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
				}
				w.Header().Set("Content-Type", "application/json")
				switch kind {
				case ProviderOpenAI:
					_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`))
				case ProviderAnthropic:
					_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`))
				case ProviderGemini:
					_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"ok"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":1,"candidatesTokenCount":1}}`))
				}
			}))
			defer server.Close()
			var adapter ProviderAdapter
			switch kind {
			case ProviderOpenAI:
				adapter = NewOpenAIAdapter(server.URL, "test", server.Client())
			case ProviderAnthropic:
				adapter = NewAnthropicAdapter(server.URL, "test", server.Client())
			case ProviderGemini:
				adapter = NewGeminiAdapter(server.URL, "test", server.Client())
			}
			_, err := adapter.Invoke(context.Background(), ProviderRequest{Model: "vision", MaxTokens: 100, Messages: []Message{{Role: "user", Content: "Extract the document", Attachments: []Attachment{
				{Name: "photo.png", MediaType: "image/png", Data: []byte("image bytes")},
				{Name: "invoice.pdf", MediaType: "application/pdf", Data: []byte("%PDF-1.4")},
			}}}})
			if err != nil {
				t.Fatal(err)
			}
			payload, _ := json.Marshal(request)
			for _, value := range []string{"Extract the document", base64.StdEncoding.EncodeToString([]byte("image bytes")), base64.StdEncoding.EncodeToString([]byte("%PDF-1.4"))} {
				if !strings.Contains(string(payload), value) {
					t.Fatalf("attachment/text missing: %s", payload)
				}
			}
			switch kind {
			case ProviderOpenAI:
				message := request["messages"].([]any)[0].(map[string]any)
				parts := message["content"].([]any)
				if parts[1].(map[string]any)["type"] != "image_url" || parts[2].(map[string]any)["type"] != "file" {
					t.Fatalf("invalid OpenAI parts: %#v", parts)
				}
			case ProviderAnthropic:
				parts := request["messages"].([]any)[0].(map[string]any)["content"].([]any)
				if parts[1].(map[string]any)["type"] != "image" || parts[2].(map[string]any)["type"] != "document" {
					t.Fatalf("invalid Anthropic parts: %#v", parts)
				}
			case ProviderGemini:
				parts := request["contents"].([]any)[0].(map[string]any)["parts"].([]any)
				if parts[2].(map[string]any)["inlineData"].(map[string]any)["mimeType"] != "application/pdf" {
					t.Fatalf("invalid Gemini parts: %#v", parts)
				}
			}
		})
	}
}

func TestAttachmentLimitsAndRoleAreValidated(t *testing.T) {
	valid := Attachment{Name: "photo.png", MediaType: "image/png", Data: []byte("test")}
	if err := validateMessageAttachments([]Message{{Role: "user", Attachments: []Attachment{valid}}}); err != nil {
		t.Fatal(err)
	}
	for _, message := range []Message{
		{Role: "system", Attachments: []Attachment{valid}},
		{Role: "user", Attachments: []Attachment{{MediaType: "text/html", Data: []byte("test")}}},
		{Role: "user", Attachments: []Attachment{{MediaType: "image/png"}}},
		{Role: "user", Attachments: []Attachment{valid, valid, valid, valid, valid, valid}},
	} {
		if err := validateMessageAttachments([]Message{message}); err == nil {
			t.Fatal("invalid attachments accepted")
		}
	}
}
