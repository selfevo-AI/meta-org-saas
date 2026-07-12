package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/authlimit"
)

type requestLimiterCall struct {
	scope string
	key   string
}

type fakeRequestLimiter struct {
	decision authlimit.Decision
	err      error
	calls    []requestLimiterCall
}

func (f *fakeRequestLimiter) Consume(_ context.Context, scope, key string, _ authlimit.Policy) (authlimit.Decision, error) {
	f.calls = append(f.calls, requestLimiterCall{scope: scope, key: key})
	return f.decision, f.err
}

func (*fakeRequestLimiter) RecordFailure(context.Context, string, string, authlimit.Policy) (authlimit.Decision, error) {
	return authlimit.Decision{}, nil
}
func (*fakeRequestLimiter) Reset(context.Context, string, string) error { return nil }
func (*fakeRequestLimiter) Stats() authlimit.Stats                      { return authlimit.Stats{} }

func TestAuthenticatedSensitiveRateLimitUsesActorAndClientBuckets(t *testing.T) {
	limiter := &fakeRequestLimiter{decision: authlimit.Decision{Allowed: true}}
	middleware := RequestRateLimit(limiter, &ClientIPResolver{}, authlimit.Policy{}, AuthenticatedSensitiveRateLimitBuckets)
	nextCalled := false
	handler := middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { nextCalled = true }))
	request := httptest.NewRequest(http.MethodPost, "/api/v1/platform/admin/model-providers/provider-id/rotate-key", nil)
	request.RemoteAddr = "203.0.113.9:443"
	request = request.WithContext(context.WithValue(request.Context(), UserContextKey, AuthenticatedUser{ID: "user-1", Type: "human"}))

	handler.ServeHTTP(httptest.NewRecorder(), request)

	if !nextCalled {
		t.Fatal("next handler was not called")
	}
	if len(limiter.calls) != 2 || limiter.calls[0].scope != "sensitive_model_secret_rotation_actor" || limiter.calls[0].key != "user-1" || limiter.calls[1].key != "203.0.113.9" {
		t.Fatalf("rate limit calls = %#v", limiter.calls)
	}
}

func TestPublicInvitationRateLimitHashesTokenThroughLimiterBucket(t *testing.T) {
	limiter := &fakeRequestLimiter{decision: authlimit.Decision{Allowed: true}}
	handler := RequestRateLimit(limiter, &ClientIPResolver{}, authlimit.Policy{}, PublicInvitationRateLimitBuckets)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/invitations/secret-token/accept", nil)
	request.RemoteAddr = "198.51.100.7:1234"
	handler.ServeHTTP(httptest.NewRecorder(), request)

	if len(limiter.calls) != 2 || limiter.calls[1].scope != "invitation_accept_token" || limiter.calls[1].key != "secret-token" {
		t.Fatalf("rate limit calls = %#v", limiter.calls)
	}
}

func TestRequestRateLimitReturnsRetryAfterAndFailsClosed(t *testing.T) {
	limiter := &fakeRequestLimiter{decision: authlimit.Decision{Allowed: false, RetryAfter: 1500 * time.Millisecond}}
	handler := RequestRateLimit(limiter, &ClientIPResolver{}, authlimit.Policy{}, AIGatewayCompatibleRateLimitBuckets)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	request.RemoteAddr = "192.0.2.5:80"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") != "1" {
		t.Fatalf("rate limited response = %d retry-after=%q", response.Code, response.Header().Get("Retry-After"))
	}

	limiter.decision = authlimit.Decision{}
	limiter.err = errors.New("database unavailable")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("store error response = %d, want 503", response.Code)
	}
}

func TestNonSensitiveRequestDoesNotConsumeBucket(t *testing.T) {
	limiter := &fakeRequestLimiter{decision: authlimit.Decision{Allowed: true}}
	nextCalled := false
	handler := RequestRateLimit(limiter, &ClientIPResolver{}, authlimit.Policy{}, AuthenticatedSensitiveRateLimitBuckets)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { nextCalled = true }))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/platform/admin/models", nil))
	if !nextCalled || len(limiter.calls) != 0 {
		t.Fatalf("nextCalled=%v calls=%#v", nextCalled, limiter.calls)
	}
}

func TestSensitiveOperationCategoryCoverage(t *testing.T) {
	tests := []struct {
		method string
		path   string
		want   string
	}{
		{http.MethodPost, "/api/v1/organizations/org-1/invitations", "invitation_create"},
		{http.MethodPost, "/api/v1/platform/admin/ai-gateway/access-tokens", "access_token_create"},
		{http.MethodPost, "/api/v1/platform/admin/model-providers", "model_configuration"},
		{http.MethodPatch, "/api/v1/platform/admin/model-provider-channels/channel-1", "model_configuration"},
		{http.MethodPost, "/api/v1/platform/admin/model-provider-channels/channel-1/rotate-key", "model_secret_rotation"},
		{http.MethodPost, "/api/v1/platform/admin/ai-gateway/balance-adjustments", "gateway_balance_adjustment"},
		{http.MethodPost, "/api/v1/platform/admin/users/user-1/reset-password", "account_administration"},
		{http.MethodPost, "/api/v1/platform/admin/database-maintenance/jobs", "database_maintenance"},
		{http.MethodPost, "/api/v1/platform/admin/organizations/org-1/close", "platform_administration"},
		{http.MethodPost, "/api/v1/platform/admin/assistant/sessions/session-1/runs", ""},
		{http.MethodGet, "/api/v1/platform/admin/models", ""},
		{http.MethodPost, "/api/v1/projects", ""},
	}
	for _, test := range tests {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			if got := sensitiveOperationCategory(test.method, test.path); got != test.want {
				t.Fatalf("sensitiveOperationCategory() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestAIGatewayCompatibleRateLimitUsesTokenAndIPBuckets(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	request.RemoteAddr = "192.0.2.8:443"
	request.Header.Set("Authorization", "Bearer access-token-secret")
	buckets := AIGatewayCompatibleRateLimitBuckets(request, &ClientIPResolver{})
	if len(buckets) != 2 || buckets[0].Key != "192.0.2.8" || buckets[1].Scope != "ai_gateway_compatible_token" || buckets[1].Key != "access-token-secret" {
		t.Fatalf("compatible API buckets = %#v", buckets)
	}
}
