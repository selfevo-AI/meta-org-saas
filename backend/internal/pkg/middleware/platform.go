package middleware

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/platformauth"
)

type PlatformRoleResolver interface {
	GetPlatformRole(context.Context, uuid.UUID) (string, error)
}

type platformPermissionResolver interface {
	ListPlatformRolePermissions(context.Context, string) ([]string, error)
}

func PlatformPermissionMiddleware(resolver PlatformRoleResolver, permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if resolver == nil {
				writePlatformError(w, http.StatusInternalServerError, "platform resolver is not configured")
				return
			}
			user, ok := UserFromContext(r.Context())
			if !ok || user.Type != "human" {
				writePlatformError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			userID, err := uuid.Parse(user.ID)
			if err != nil {
				writePlatformError(w, http.StatusUnauthorized, "invalid authenticated user")
				return
			}
			role, err := resolver.GetPlatformRole(r.Context(), userID)
			if err != nil || !resolverHasPermission(r.Context(), resolver, role, permission) {
				writePlatformError(w, http.StatusForbidden, "platform_forbidden")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func resolverHasPermission(ctx context.Context, resolver PlatformRoleResolver, role string, permission string) bool {
	normalized := platformauth.NormalizeRole(role)
	if permissionResolver, ok := resolver.(platformPermissionResolver); ok {
		if permissions, err := permissionResolver.ListPlatformRolePermissions(ctx, normalized); err == nil && len(permissions) > 0 {
			for _, item := range permissions {
				if item == permission {
					return true
				}
			}
			return false
		}
	}
	return platformauth.HasPermission(normalized, permission)
}

func writePlatformError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
