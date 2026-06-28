package identity

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func TestChangeOwnPasswordRequiresCurrentPasswordAndStoresHash(t *testing.T) {
	userID := uuid.New()
	hash, err := bcrypt.GenerateFromPassword([]byte("OldPass!2026"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash old password: %v", err)
	}
	repo := &passwordChangeRepo{
		user: &User{ID: userID, Name: "Tenant User", Email: "tenant@example.test", PasswordHash: string(hash)},
	}
	service := NewService(repo, "test-secret")

	if err := service.ChangeOwnPassword(context.Background(), userID, "wrong", "NewPass!2026"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("ChangeOwnPassword wrong current error = %v, want ErrInvalidCredentials", err)
	}
	if repo.updatedHash != "" {
		t.Fatal("password hash updated despite wrong current password")
	}

	if err := service.ChangeOwnPassword(context.Background(), userID, "OldPass!2026", "NewPass!2026"); err != nil {
		t.Fatalf("ChangeOwnPassword error = %v", err)
	}
	if repo.updatedHash == "" || repo.updatedHash == "NewPass!2026" {
		t.Fatalf("updatedHash = %q, want non-empty bcrypt hash", repo.updatedHash)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(repo.updatedHash), []byte("NewPass!2026")); err != nil {
		t.Fatalf("updated hash does not verify new password: %v", err)
	}
}

type passwordChangeRepo struct {
	user        *User
	updatedHash string
}

func (r *passwordChangeRepo) CreateUser(context.Context, CreateUserInput) (*User, error) {
	return nil, nil
}

func (r *passwordChangeRepo) GetUserByEmail(context.Context, string) (*User, error) {
	return nil, nil
}

func (r *passwordChangeRepo) GetUserByID(context.Context, uuid.UUID) (*User, error) {
	return r.user, nil
}

func (r *passwordChangeRepo) UpdateUserPassword(_ context.Context, _ uuid.UUID, passwordHash string) error {
	r.updatedHash = passwordHash
	return nil
}

func (r *passwordChangeRepo) CreateAgent(context.Context, CreateAgentInput) (*AIAgent, string, error) {
	return nil, "", nil
}

func (r *passwordChangeRepo) GetAgentByID(context.Context, uuid.UUID) (*AIAgent, error) {
	return nil, nil
}

func (r *passwordChangeRepo) ListAgents(context.Context, int) ([]AIAgent, error) {
	return nil, nil
}

func (r *passwordChangeRepo) ListAgentsByOrganization(context.Context, uuid.UUID, int) ([]AIAgent, error) {
	return nil, nil
}

func (r *passwordChangeRepo) AttachAgentToOrganization(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}

func (r *passwordChangeRepo) ListRoles(context.Context) ([]Role, error) {
	return nil, nil
}
