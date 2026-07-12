package identity

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/authlimit"
)

type fakeAuthLimiter struct {
	consumeDecision authlimit.Decision
	consumeErr      error
	failureCalls    int
	resetCalls      int
}

func (f *fakeAuthLimiter) Consume(context.Context, string, string, authlimit.Policy) (authlimit.Decision, error) {
	return f.consumeDecision, f.consumeErr
}

func (f *fakeAuthLimiter) RecordFailure(context.Context, string, string, authlimit.Policy) (authlimit.Decision, error) {
	f.failureCalls++
	return authlimit.Decision{}, nil
}

func (f *fakeAuthLimiter) Reset(context.Context, string, string) error {
	f.resetCalls++
	return nil
}

func (f *fakeAuthLimiter) Stats() authlimit.Stats {
	return authlimit.Stats{}
}

func TestConsumeBucketsReturnsRetryAfterWhenBlocked(t *testing.T) {
	limiter := &fakeAuthLimiter{consumeDecision: authlimit.Decision{Allowed: false, RetryAfter: 1500 * time.Millisecond}}
	handler := &Handler{limiter: limiter}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	response := httptest.NewRecorder()

	allowed := handler.consumeBuckets(response, request, []rateLimitBucket{{scope: "user_login_ip", key: "127.0.0.1"}}, authlimit.Policy{})

	if allowed {
		t.Fatal("consumeBuckets allowed blocked request")
	}
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", response.Code)
	}
	if got := response.Header().Get("Retry-After"); got != "2" {
		t.Fatalf("Retry-After = %q, want 2", got)
	}
}

func TestConsumeBucketsFailsClosedWhenStoreUnavailable(t *testing.T) {
	limiter := &fakeAuthLimiter{consumeErr: errors.New("database unavailable")}
	handler := &Handler{limiter: limiter}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	response := httptest.NewRecorder()

	allowed := handler.consumeBuckets(response, request, []rateLimitBucket{{scope: "user_login_ip", key: "127.0.0.1"}}, authlimit.Policy{})

	if allowed {
		t.Fatal("consumeBuckets allowed request when store failed")
	}
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", response.Code)
	}
}

func TestRecordFailuresAndResetUseAllRequestedBuckets(t *testing.T) {
	limiter := &fakeAuthLimiter{}
	handler := &Handler{limiter: limiter}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	buckets := []rateLimitBucket{{scope: "ip", key: "127.0.0.1"}, {scope: "subject", key: "user"}}

	handler.recordFailures(request, buckets, authlimit.Policy{})
	handler.resetBucket(request, buckets[1])

	if limiter.failureCalls != 2 || limiter.resetCalls != 1 {
		t.Fatalf("failure/reset calls = %d/%d", limiter.failureCalls, limiter.resetCalls)
	}
}
