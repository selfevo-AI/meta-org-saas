package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recoverer converts a handler panic into a 500 that follows the API error
// contract instead of letting net/http drop the connection. The stack trace
// goes to the log with the request id so the client-visible message can stay
// generic.
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}
			if err, ok := recovered.(error); ok && err == http.ErrAbortHandler {
				panic(recovered)
			}
			slog.Error("panic serving request", "method", r.Method, "path", r.URL.Path, "request_id", RequestIDFromContext(r.Context()), "panic", recovered, "stack", string(debug.Stack()))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal_error", "code": "internal_error"})
		}()
		next.ServeHTTP(w, r)
	})
}

// LimitRequestBody caps every request body at maxBytes. Handlers that read
// the body get an error once the cap is exceeded; handlers with tighter
// per-route limits keep applying their own http.MaxBytesReader on top.
func LimitRequestBody(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil && r.Body != http.NoBody {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}
