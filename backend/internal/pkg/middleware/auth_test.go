package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type staticSessionVersions struct {
	version int64
	err     error
	calls   int
}

func (s *staticSessionVersions) SessionVersion(context.Context, string) (int64, error) {
	s.calls++
	return s.version, s.err
}

func signTestToken(t *testing.T, secret string, claims map[string]any) string {
	t.Helper()
	header, err := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	headerB64 := base64.RawURLEncoding.EncodeToString(header)
	payloadB64 := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(headerB64 + "." + payloadB64))
	return headerB64 + "." + payloadB64 + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func sessionTestClaims(userType string, version *int64) map[string]any {
	claims := map[string]any{
		"sub":  "5a4a0f4e-0000-0000-0000-000000000001",
		"type": userType,
		"name": "Test User",
		"exp":  time.Now().Add(time.Hour).Unix(),
	}
	if version != nil {
		claims["ver"] = *version
	}
	return claims
}

func runAuthRequest(t *testing.T, secret string, sessions SessionVersionResolver, claims map[string]any) (*httptest.ResponseRecorder, bool) {
	t.Helper()
	reached := false
	handler := AuthMiddleware(secret, sessions)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	}))
	request := httptest.NewRequest(http.MethodGet, "/thing", nil)
	request.Header.Set("Authorization", "Bearer "+signTestToken(t, secret, claims))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder, reached
}

func TestAuthMiddlewareAcceptsMatchingSessionVersion(t *testing.T) {
	version := int64(3)
	sessions := &staticSessionVersions{version: 3}
	recorder, reached := runAuthRequest(t, "test-secret", sessions, sessionTestClaims("human", &version))
	if recorder.Code != http.StatusOK || !reached {
		t.Fatalf("status = %d reached = %v, want 200 and handler reached", recorder.Code, reached)
	}
}

func TestAuthMiddlewareTreatsMissingVersionAsZero(t *testing.T) {
	sessions := &staticSessionVersions{version: 0}
	recorder, reached := runAuthRequest(t, "test-secret", sessions, sessionTestClaims("human", nil))
	if recorder.Code != http.StatusOK || !reached {
		t.Fatalf("status = %d reached = %v, want legacy token without ver accepted while version is 0", recorder.Code, reached)
	}
}

func TestAuthMiddlewareRejectsRevokedSession(t *testing.T) {
	version := int64(0)
	sessions := &staticSessionVersions{version: 1}
	recorder, reached := runAuthRequest(t, "test-secret", sessions, sessionTestClaims("human", &version))
	if recorder.Code != http.StatusUnauthorized || reached {
		t.Fatalf("status = %d reached = %v, want 401 and handler not reached", recorder.Code, reached)
	}
	if body := recorder.Body.String(); !strings.Contains(body, "session_revoked") {
		t.Fatalf("body = %q, want session_revoked code", body)
	}
}

func TestAuthMiddlewareRejectsRevokedLegacyToken(t *testing.T) {
	sessions := &staticSessionVersions{version: 2}
	recorder, _ := runAuthRequest(t, "test-secret", sessions, sessionTestClaims("human", nil))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 for a legacy token after a session bump", recorder.Code)
	}
}

func TestAuthMiddlewareFailsClosedWhenVersionLookupFails(t *testing.T) {
	version := int64(0)
	sessions := &staticSessionVersions{err: errors.New("database down")}
	recorder, reached := runAuthRequest(t, "test-secret", sessions, sessionTestClaims("human", &version))
	if recorder.Code != http.StatusServiceUnavailable || reached {
		t.Fatalf("status = %d reached = %v, want 503 and handler not reached", recorder.Code, reached)
	}
}

func TestAuthMiddlewareSkipsSessionCheckForAgents(t *testing.T) {
	sessions := &staticSessionVersions{version: 9}
	recorder, reached := runAuthRequest(t, "test-secret", sessions, sessionTestClaims("ai", nil))
	if recorder.Code != http.StatusOK || !reached {
		t.Fatalf("status = %d reached = %v, want agent token accepted without version check", recorder.Code, reached)
	}
	if sessions.calls != 0 {
		t.Fatalf("resolver calls = %d, want 0 for an agent token", sessions.calls)
	}
}
