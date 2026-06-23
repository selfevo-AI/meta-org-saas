package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/passwordhash"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	password := flag.String("password", "", "Plaintext password to hash. Prefer BCRYPT_PASSWORD or -stdin to avoid shell history.")
	cost := flag.Int("cost", bcrypt.DefaultCost, "Bcrypt cost. Use 0 for bcrypt default cost.")
	fromStdin := flag.Bool("stdin", false, "Read plaintext password from stdin.")
	flag.Parse()

	value, err := resolvePassword(*password, *fromStdin, os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	hash, err := passwordhash.GenerateBcryptHash(value, *cost)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(hash)
}

func resolvePassword(flagValue string, fromStdin bool, stdin io.Reader) (string, error) {
	if fromStdin {
		raw, err := io.ReadAll(stdin)
		if err != nil {
			return "", fmt.Errorf("read password from stdin: %w", err)
		}
		return strings.TrimRight(string(raw), "\r\n"), nil
	}
	if flagValue != "" {
		return flagValue, nil
	}
	if envValue := os.Getenv("BCRYPT_PASSWORD"); envValue != "" {
		return envValue, nil
	}
	return "", passwordhash.ErrEmptyPassword
}
