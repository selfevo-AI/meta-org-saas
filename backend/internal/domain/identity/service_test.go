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

	if _, _, err := service.ChangeOwnPassword(context.Background(), userID, "wrong", "NewPass!2026"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("ChangeOwnPassword wrong current error = %v, want ErrInvalidCredentials", err)
	}
	if repo.updatedHash != "" {
		t.Fatal("password hash updated despite wrong current password")
	}

	token, expiresAt, err := service.ChangeOwnPassword(context.Background(), userID, "OldPass!2026", "NewPass!2026")
	if err != nil {
		t.Fatalf("ChangeOwnPassword error = %v", err)
	}
	if token == "" || expiresAt == 0 {
		t.Fatalf("ChangeOwnPassword replacement token = (%q, %d), want a fresh session token", token, expiresAt)
	}
	if repo.user.SessionVersion == 0 {
		t.Fatal("session version not bumped by password change")
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

// loginRepo serves the stored user by email so AuthenticateUser can be driven
// without a database.
type loginRepo struct{ passwordChangeRepo }

func (r *loginRepo) GetUserByEmail(context.Context, string) (*User, error) {
	return r.user, nil
}

func TestAuthenticateUserRejectsDisabledAccountOnlyAfterPasswordCheck(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("Secret!2026"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	repo := &loginRepo{passwordChangeRepo{user: &User{
		ID: uuid.New(), Name: "Disabled", Email: "disabled@example.test", PasswordHash: string(hash), AccountStatus: AccountStatusDisabled,
	}}}
	service := NewService(repo, "test-secret")

	if _, err := service.AuthenticateUser(context.Background(), "disabled@example.test", "wrong"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password error = %v, want ErrInvalidCredentials (must not reveal disabled state)", err)
	}
	if _, err := service.AuthenticateUser(context.Background(), "disabled@example.test", "Secret!2026"); !errors.Is(err, ErrAccountDisabled) {
		t.Fatalf("disabled account error = %v, want ErrAccountDisabled", err)
	}

	repo.user.AccountStatus = AccountStatusActive
	resp, err := service.AuthenticateUser(context.Background(), "disabled@example.test", "Secret!2026")
	if err != nil || resp == nil || resp.Token == "" {
		t.Fatalf("active account login = (%v, %v), want token", resp, err)
	}
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

func (r *passwordChangeRepo) GetSessionVersion(context.Context, uuid.UUID) (int64, error) {
	return r.user.SessionVersion, nil
}

func (r *passwordChangeRepo) UpdateUserPassword(_ context.Context, _ uuid.UUID, passwordHash string) (int64, error) {
	r.updatedHash = passwordHash
	r.user.SessionVersion++
	return r.user.SessionVersion, nil
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
