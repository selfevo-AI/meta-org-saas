package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func authorityRequest(user *AuthenticatedUser, tenant *TenantContext) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/costing/currencies", nil)
	ctx := req.Context()
	if user != nil {
		ctx = context.WithValue(ctx, UserContextKey, *user)
	}
	if tenant != nil {
		ctx = context.WithValue(ctx, TenantContextKey, tenant)
	}
	return req.WithContext(ctx)
}

func TestRequireAuthority(t *testing.T) {
	human := &AuthenticatedUser{ID: "u1", Type: "human"}
	agent := &AuthenticatedUser{ID: "a1", Type: "ai"}
	cases := []struct {
		name     string
		required string
		user     *AuthenticatedUser
		tenant   *TenantContext
		status   int
		code     string
	}{
		{"no user", AuthorityReviewer, nil, &TenantContext{AuthorityTier: AuthorityReviewer}, http.StatusUnauthorized, "unauthorized"},
		{"no tenant", AuthorityReviewer, human, nil, http.StatusForbidden, "organization_required"},
		{"executor below reviewer", AuthorityReviewer, human, &TenantContext{AuthorityTier: AuthorityExecutor}, http.StatusForbidden, "authority_insufficient"},
		{"reviewer meets reviewer", AuthorityReviewer, human, &TenantContext{AuthorityTier: AuthorityReviewer}, http.StatusOK, ""},
		{"admin exceeds reviewer", AuthorityReviewer, human, &TenantContext{AuthorityTier: AuthorityOrganizationAdmin}, http.StatusOK, ""},
		{"reviewer below admin", AuthorityOrganizationAdmin, human, &TenantContext{AuthorityTier: AuthorityReviewer}, http.StatusForbidden, "authority_insufficient"},
		{"creator meets admin", AuthorityOrganizationAdmin, human, &TenantContext{AuthorityTier: AuthorityOrganizationCreator}, http.StatusOK, ""},
		{"unknown tier is lowest", AuthorityExecutor, human, &TenantContext{AuthorityTier: "observer"}, http.StatusForbidden, "authority_insufficient"},
		{"agent blocked even as admin", AuthorityExecutor, agent, &TenantContext{AuthorityTier: AuthorityOrganizationAdmin}, http.StatusForbidden, "authority_human_required"},
		{"platform admin admitted", AuthorityOrganizationCreator, human, &TenantContext{IsPlatformAdmin: true}, http.StatusOK, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			handler := RequireAuthority(tc.required)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			}))
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, authorityRequest(tc.user, tc.tenant))
			if recorder.Code != tc.status {
				t.Fatalf("status = %d, want %d (body %s)", recorder.Code, tc.status, recorder.Body.String())
			}
			if (tc.status == http.StatusOK) != called {
				t.Fatalf("handler called = %v, want %v", called, tc.status == http.StatusOK)
			}
			if tc.code != "" && !containsJSONError(recorder.Body.String(), tc.code) {
				t.Fatalf("body = %s, want error %q", recorder.Body.String(), tc.code)
			}
		})
	}
}

func TestRequireAuthorityRejectsUnknownTier(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for unknown tier")
		}
	}()
	RequireAuthority("superuser")
}

func containsJSONError(body, code string) bool {
	var payload map[string]string
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return false
	}
	return payload["error"] == code && payload["code"] == code
}
