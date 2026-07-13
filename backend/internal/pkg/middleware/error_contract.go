package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

const RequestIDHeader = "X-Request-ID"

type requestIDContextKey struct{}

type errorEnvelope struct {
	Error     string `json:"error"`
	Code      string `json:"code"`
	RequestID string `json:"request_id"`
}

func APIErrorContract(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := normalizedRequestID(r.Header.Get(RequestIDHeader))
		w.Header().Set(RequestIDHeader, requestID)
		r = r.WithContext(context.WithValue(r.Context(), requestIDContextKey{}, requestID))
		writer := &errorContractWriter{ResponseWriter: w}
		next.ServeHTTP(writer, r)
		writer.finish(requestID)
	})
}

func RequestIDFromContext(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDContextKey{}).(string)
	return requestID
}

type errorContractWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
	capture     bool
	body        bytes.Buffer
}

func (w *errorContractWriter) WriteHeader(status int) {
	if status >= 100 && status < 200 {
		w.ResponseWriter.WriteHeader(status)
		return
	}
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.status = status
	w.capture = status >= http.StatusBadRequest
	if !w.capture {
		w.ResponseWriter.WriteHeader(status)
	}
}

func (w *errorContractWriter) Write(value []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if w.capture {
		return w.body.Write(value)
	}
	return w.ResponseWriter.Write(value)
}

func (w *errorContractWriter) Flush() {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if w.capture {
		return
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *errorContractWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *errorContractWriter) finish(requestID string) {
	if !w.capture {
		return
	}
	message, existingCode, ok := decodeErrorPayload(w.body.Bytes())
	if !ok {
		w.ResponseWriter.WriteHeader(w.status)
		_, _ = w.ResponseWriter.Write(w.body.Bytes())
		return
	}
	code := existingCode
	if code == "" {
		code = errorCodeForStatus(w.status)
	}
	if w.status >= http.StatusInternalServerError && !isStablePublicError(message) {
		log.Printf("api request failed: request_id=%s status=%d error=%s", requestID, w.status, message)
		message = publicMessageForCode(code)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Del("Content-Length")
	w.Header().Set("X-Error-Code", code)
	w.ResponseWriter.WriteHeader(w.status)
	if err := json.NewEncoder(w.ResponseWriter).Encode(errorEnvelope{Error: message, Code: code, RequestID: requestID}); err != nil {
		log.Printf("api error response failed: request_id=%s error=%v", requestID, err)
	}
}

func decodeErrorPayload(body []byte) (string, string, bool) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", "", false
	}
	message, ok := payload["error"].(string)
	if !ok || strings.TrimSpace(message) == "" {
		return "", "", false
	}
	code, _ := payload["code"].(string)
	return strings.TrimSpace(message), strings.TrimSpace(code), true
}

func normalizedRequestID(value string) string {
	requestID, err := uuid.Parse(strings.TrimSpace(value))
	if err != nil {
		return uuid.NewString()
	}
	return requestID.String()
}

func errorCodeForStatus(status int) string {
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return "invalid_request"
	case http.StatusUnauthorized:
		return "authentication_required"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusMethodNotAllowed:
		return "method_not_allowed"
	case http.StatusConflict:
		return "conflict"
	case http.StatusRequestEntityTooLarge:
		return "payload_too_large"
	case http.StatusUnsupportedMediaType:
		return "unsupported_media_type"
	case http.StatusTooManyRequests:
		return "rate_limited"
	case http.StatusBadGateway:
		return "upstream_unavailable"
	case http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return "service_unavailable"
	default:
		if status >= http.StatusInternalServerError {
			return "internal_error"
		}
		return "request_failed"
	}
}

func publicMessageForCode(code string) string {
	switch code {
	case "service_unavailable":
		return "service unavailable"
	case "upstream_unavailable":
		return "upstream service unavailable"
	default:
		return "internal server error"
	}
}

func isStablePublicError(message string) bool {
	if message == "" || len(message) > 96 {
		return false
	}
	for _, value := range message {
		if (value >= 'a' && value <= 'z') || (value >= '0' && value <= '9') || value == '_' || value == '-' || value == '.' {
			continue
		}
		return false
	}
	return true
}
