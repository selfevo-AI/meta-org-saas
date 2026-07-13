package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestAPIErrorContractAddsStableCodeAndRequestID(t *testing.T) {
	requestID := uuid.NewString()
	handler := APIErrorContract(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if RequestIDFromContext(r.Context()) != requestID {
			t.Fatalf("request id context = %q", RequestIDFromContext(r.Context()))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"document status cannot transition"}`))
	}))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/projects", nil)
	request.Header.Set(RequestIDHeader, requestID)
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusConflict || recorder.Header().Get(RequestIDHeader) != requestID || recorder.Header().Get("X-Error-Code") != "conflict" {
		t.Fatalf("status/headers = %d %#v", recorder.Code, recorder.Header())
	}
	var response errorEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Error != "document status cannot transition" || response.Code != "conflict" || response.RequestID != requestID {
		t.Fatalf("response = %#v", response)
	}
}

func TestAPIErrorContractHidesInternalErrorDetails(t *testing.T) {
	handler := APIErrorContract(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"password authentication failed for postgres at 10.0.0.5"}`))
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/private", nil))

	var response errorEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Error != "internal server error" || response.Code != "internal_error" || response.RequestID == "" {
		t.Fatalf("response = %#v", response)
	}
}

func TestAPIErrorContractPreservesStructuredUnavailableResponse(t *testing.T) {
	handler := APIErrorContract(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"status":"unavailable","platform_database":{"status":"unavailable"}}`))
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))

	if recorder.Code != http.StatusServiceUnavailable || recorder.Header().Get("X-Error-Code") != "" {
		t.Fatalf("status/headers = %d %#v", recorder.Code, recorder.Header())
	}
	if recorder.Body.String() != `{"status":"unavailable","platform_database":{"status":"unavailable"}}` {
		t.Fatalf("body = %s", recorder.Body.String())
	}
}

func TestAPIErrorContractPreservesSuccessfulStreaming(t *testing.T) {
	handler := APIErrorContract(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("response writer does not implement http.Flusher")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: done\ndata: {}\n\n"))
		flusher.Flush()
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/stream", nil))

	if recorder.Code != http.StatusOK || recorder.Body.String() != "event: done\ndata: {}\n\n" {
		t.Fatalf("response = %d %q", recorder.Code, recorder.Body.String())
	}
}

func TestRequestIDFromContextWithoutMiddlewareIsEmpty(t *testing.T) {
	if value := RequestIDFromContext(context.Background()); value != "" {
		t.Fatalf("request id = %q", value)
	}
}
