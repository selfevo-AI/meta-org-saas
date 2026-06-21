package passwordhash

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestGenerateBcryptHashCreatesVerifiableHash(t *testing.T) {
	password := "example-password-for-hash-test"

	hash, err := GenerateBcryptHash(password, bcrypt.MinCost)

	if err != nil {
		t.Fatalf("GenerateBcryptHash() error = %v", err)
	}
	if !strings.HasPrefix(hash, "$2") {
		t.Fatalf("GenerateBcryptHash() = %q, want bcrypt prefix", hash)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		t.Fatalf("generated hash did not verify password: %v", err)
	}
}

func TestGenerateBcryptHashRejectsEmptyPassword(t *testing.T) {
	_, err := GenerateBcryptHash("", bcrypt.MinCost)

	if err == nil {
		t.Fatal("GenerateBcryptHash() succeeded, want error")
	}
}

func TestGenerateBcryptHashRejectsInvalidCost(t *testing.T) {
	_, err := GenerateBcryptHash("example-password-for-hash-test", bcrypt.MaxCost+1)

	if err == nil {
		t.Fatal("GenerateBcryptHash() succeeded, want invalid cost error")
	}
}
