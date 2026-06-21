package identity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

type duplicateEmailRepo struct{}

func (duplicateEmailRepo) CreateUser(context.Context, CreateUserInput) (*User, error) {
	return nil, fmt.Errorf("create user: %w", &pgconn.PgError{
		Code:           "23505",
		ConstraintName: "users_email_key",
		Message:        `duplicate key value violates unique constraint "users_email_key"`,
	})
}

func (duplicateEmailRepo) GetUserByEmail(context.Context, string) (*User, error) {
	return nil, fmt.Errorf("not implemented")
}

func (duplicateEmailRepo) GetUserByID(context.Context, uuid.UUID) (*User, error) {
	return nil, fmt.Errorf("not implemented")
}

func (duplicateEmailRepo) CreateAgent(context.Context, CreateAgentInput) (*AIAgent, string, error) {
	return nil, "", fmt.Errorf("not implemented")
}

func (duplicateEmailRepo) GetAgentByID(context.Context, uuid.UUID) (*AIAgent, error) {
	return nil, fmt.Errorf("not implemented")
}

func (duplicateEmailRepo) ListAgents(context.Context, int) ([]AIAgent, error) {
	return nil, fmt.Errorf("not implemented")
}

func (duplicateEmailRepo) ListAgentsByOrganization(context.Context, uuid.UUID, int) ([]AIAgent, error) {
	return nil, fmt.Errorf("not implemented")
}

func (duplicateEmailRepo) AttachAgentToOrganization(context.Context, uuid.UUID, uuid.UUID) error {
	return fmt.Errorf("not implemented")
}

func (duplicateEmailRepo) ListRoles(context.Context) ([]Role, error) {
	return nil, fmt.Errorf("not implemented")
}

func TestRegisterDuplicateEmailReturnsConflictWithoutDatabaseDetails(t *testing.T) {
	router := chi.NewRouter()
	NewHandler(NewService(duplicateEmailRepo{}, "test-secret")).RegisterPublicRoutes(router)

	req := httptest.NewRequest(
		http.MethodPost,
		"/auth/register",
		bytes.NewBufferString(`{"name":"Existing","email":"existing@example.com","password":"Secret123!"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusConflict, rec.Body.String())
	}

	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["error"] != "email already registered" {
		t.Fatalf("error = %q, want %q", payload["error"], "email already registered")
	}
	if strings.Contains(payload["error"], "users_email_key") || strings.Contains(payload["error"], "SQLSTATE") {
		t.Fatalf("error leaks database details: %q", payload["error"])
	}
}
