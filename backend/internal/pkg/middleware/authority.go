package middleware

import (
	"encoding/json"
	"net/http"
)

// Membership authority tiers, least to most privileged. They mirror the CHECK
// constraint on organization_memberships.authority_tier.
const (
	AuthorityExecutor            = "executor"
	AuthorityReviewer            = "reviewer"
	AuthorityOrganizationAdmin   = "organization_admin"
	AuthorityOrganizationCreator = "organization_creator"
)

var authorityRank = map[string]int{
	AuthorityExecutor:            1,
	AuthorityReviewer:            2,
	AuthorityOrganizationAdmin:   3,
	AuthorityOrganizationCreator: 4,
}

// AuthorityAtLeast reports whether the tenant's membership tier meets the
// required tier. Platform administrators resolving a tenant have no membership
// tier; TenantMiddleware has already enforced their tenant_data permissions, so
// they are admitted here.
func AuthorityAtLeast(tenant *TenantContext, required string) bool {
	if tenant == nil {
		return false
	}
	if tenant.IsPlatformAdmin {
		return true
	}
	return authorityRank[tenant.AuthorityTier] >= authorityRank[required]
}

// RequireAuthority gates a tenant-scoped route behind a minimum membership
// authority tier. It must run after AuthMiddleware and TenantMiddleware.
//
// Agents are never admitted: governance decisions (approvals, voids, external
// integrations) stay with humans, and agent-driven changes flow through the
// approved tool-runtime path instead of these HTTP routes.
func RequireAuthority(required string) func(http.Handler) http.Handler {
	if _, ok := authorityRank[required]; !ok {
		panic("middleware.RequireAuthority: unknown authority tier " + required)
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := UserFromContext(r.Context())
			if !ok {
				writeAuthorityError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			tenant, ok := TenantFromContext(r.Context())
			if !ok {
				writeAuthorityError(w, http.StatusForbidden, "organization_required")
				return
			}
			if user.Type != "human" && !tenant.IsPlatformAdmin {
				writeAuthorityError(w, http.StatusForbidden, "authority_human_required")
				return
			}
			if !AuthorityAtLeast(tenant, required) {
				writeAuthorityError(w, http.StatusForbidden, "authority_insufficient")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// writeAuthorityError carries the reason as the stable error code so clients
// can distinguish an authority gate from other 403 responses.
func writeAuthorityError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code, "code": code})
}
