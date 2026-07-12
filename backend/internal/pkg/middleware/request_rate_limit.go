package middleware

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/authlimit"
)

type RequestRateLimitBucket struct {
	Scope string
	Key   string
}

type RequestRateLimitBucketResolver func(*http.Request, *ClientIPResolver) []RequestRateLimitBucket

func RequestRateLimit(
	limiter authlimit.Limiter,
	clientIPs *ClientIPResolver,
	policy authlimit.Policy,
	resolve RequestRateLimitBucketResolver,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if limiter == nil || resolve == nil {
				writeRateLimitError(w, http.StatusServiceUnavailable, "rate_limit_not_configured", 0)
				return
			}
			for _, bucket := range resolve(r, clientIPs) {
				if strings.TrimSpace(bucket.Scope) == "" || strings.TrimSpace(bucket.Key) == "" {
					continue
				}
				decision, err := limiter.Consume(r.Context(), bucket.Scope, bucket.Key, policy)
				if err != nil {
					log.Printf("request rate limit check failed scope=%s: %v", bucket.Scope, err)
					writeRateLimitError(w, http.StatusServiceUnavailable, "rate_limit_unavailable", 0)
					return
				}
				if !decision.Allowed {
					retryAfter := retryAfterSeconds(decision.RetryAfter)
					log.Printf("request rate limited scope=%s attempts=%d retry_after_seconds=%d", bucket.Scope, decision.Attempts, retryAfter)
					writeRateLimitError(w, http.StatusTooManyRequests, "rate_limited", retryAfter)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func PublicInvitationRateLimitBuckets(r *http.Request, clientIPs *ClientIPResolver) []RequestRateLimitBucket {
	if r.Method != http.MethodPost {
		return nil
	}
	segments := pathSegments(r.URL.Path)
	if len(segments) != 6 || segments[0] != "api" || segments[1] != "v1" || segments[2] != "auth" || segments[3] != "invitations" || segments[5] != "accept" {
		return nil
	}
	return []RequestRateLimitBucket{
		{Scope: "invitation_accept_ip", Key: clientIPs.Resolve(r)},
		{Scope: "invitation_accept_token", Key: segments[4]},
	}
}

func AuthenticatedSensitiveRateLimitBuckets(r *http.Request, clientIPs *ClientIPResolver) []RequestRateLimitBucket {
	category := sensitiveOperationCategory(r.Method, r.URL.Path)
	if category == "" {
		return nil
	}
	user, ok := UserFromContext(r.Context())
	if !ok || user.ID == "" {
		return []RequestRateLimitBucket{{Scope: "sensitive_" + category + "_ip", Key: clientIPs.Resolve(r)}}
	}
	return []RequestRateLimitBucket{
		{Scope: "sensitive_" + category + "_actor", Key: user.ID},
		{Scope: "sensitive_" + category + "_ip", Key: clientIPs.Resolve(r)},
	}
}

func AIGatewayCompatibleRateLimitBuckets(r *http.Request, clientIPs *ClientIPResolver) []RequestRateLimitBucket {
	segments := pathSegments(r.URL.Path)
	if len(segments) < 2 || segments[0] != "v1" {
		return nil
	}
	buckets := []RequestRateLimitBucket{{Scope: "ai_gateway_compatible_ip", Key: clientIPs.Resolve(r)}}
	if token := bearerCredential(r.Header.Get("Authorization")); token != "" {
		buckets = append(buckets, RequestRateLimitBucket{Scope: "ai_gateway_compatible_token", Key: token})
	}
	return buckets
}

func sensitiveOperationCategory(method, path string) string {
	if method != http.MethodPost && method != http.MethodPatch && method != http.MethodPut && method != http.MethodDelete {
		return ""
	}
	segments := pathSegments(path)
	if len(segments) < 3 || segments[0] != "api" || segments[1] != "v1" {
		return ""
	}
	if len(segments) == 5 && segments[2] == "organizations" && segments[4] == "invitations" && method == http.MethodPost {
		return "invitation_create"
	}
	if len(segments) < 5 || segments[2] != "platform" || segments[3] != "admin" {
		return ""
	}
	rest := segments[4:]
	joined := strings.Join(rest, "/")
	if strings.HasSuffix(joined, "/rotate-key") {
		return "model_secret_rotation"
	}
	if joined == "ai-gateway/access-tokens" && method == http.MethodPost {
		return "access_token_create"
	}
	if joined == "ai-gateway/balance-adjustments" && method == http.MethodPost {
		return "gateway_balance_adjustment"
	}
	if strings.Contains(joined, "reset-password") || strings.HasSuffix(joined, "/disable") || joined == "users" {
		return "account_administration"
	}
	if strings.HasPrefix(joined, "database-maintenance/jobs") {
		return "database_maintenance"
	}
	if rest[0] == "model-providers" || rest[0] == "model-provider-channels" || rest[0] == "models" {
		return "model_configuration"
	}
	if strings.HasPrefix(joined, "ai-gateway/routing-rules") || strings.HasPrefix(joined, "ai-gateway/model-groups") || strings.HasPrefix(joined, "ai-gateway/model-channel-abilities") {
		return "model_configuration"
	}
	if rest[0] == "assistant" {
		return ""
	}
	return "platform_administration"
}

func pathSegments(path string) []string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func bearerCredential(header string) string {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func retryAfterSeconds(value time.Duration) int {
	seconds := int(value / time.Second)
	if value > 0 && seconds < 1 {
		return 1
	}
	return seconds
}

func writeRateLimitError(w http.ResponseWriter, status int, code string, retryAfter int) {
	w.Header().Set("Content-Type", "application/json")
	if retryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}
