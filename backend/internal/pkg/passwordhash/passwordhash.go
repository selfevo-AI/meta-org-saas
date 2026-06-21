package passwordhash

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

var ErrEmptyPassword = errors.New("password is required")

func GenerateBcryptHash(password string, cost int) (string, error) {
	if password == "" {
		return "", ErrEmptyPassword
	}
	if cost == 0 {
		cost = bcrypt.DefaultCost
	}
	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		return "", fmt.Errorf("bcrypt cost must be between %d and %d", bcrypt.MinCost, bcrypt.MaxCost)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", fmt.Errorf("generate bcrypt hash: %w", err)
	}
	return string(hash), nil
}
