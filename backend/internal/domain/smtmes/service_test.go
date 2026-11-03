package smtmes

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/middleware"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/safeurl"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/secretbox"
)

func TestVerifySignatureAcceptsValidHMACAndRejectsTampering(t *testing.T) {
	box, err := secretbox.New("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("secretbox.New: %v", err)
	}
	secret := "smt-webhook-secret"
	cipher, err := box.Encrypt(secret)
	if err != nil {
		t.Fatalf("encrypt secret: %v", err)
	}
	service := NewService(nil, nil, box)
	instance := &ResolvedInstance{}
	instance.WebhookSecretCipher = cipher

	body := []byte(`{"id":7,"type":"loading.error","ref":"WO-1","data":{}}`)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if err := service.VerifySignature(instance, body, signature); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
	if err := service.VerifySignature(instance, body, "sha256=00"); err == nil {
		t.Fatal("tampered signature accepted")
	}
	if err := service.VerifySignature(instance, append(body, ' '), signature); err == nil {
		t.Fatal("tampered body accepted")
	}
}

func TestMaskSecretHidesValue(t *testing.T) {
	masked := maskSecret("abcdef123456")
	if masked == "abcdef123456" || masked == "" {
		t.Fatalf("masked = %q", masked)
	}
}

func TestCreateAndUpdateEdgeRejectInternalBaseURLs(t *testing.T) {
	box, err := secretbox.New("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("secretbox.New: %v", err)
	}
	orgID := uuid.New()
	ctx := context.WithValue(context.Background(), middleware.TenantContextKey, &middleware.TenantContext{
		Mode: "saas", OrganizationID: &orgID, AuthorityTier: middleware.AuthorityOrganizationAdmin,
	})
	// A nil repository is safe: validation must fail before any persistence.
	service := NewService(nil, nil, box).WithOutboundPolicy(safeurl.Policy{})

	for _, baseURL := range []string{
		"http://127.0.0.1:8090", "http://localhost/api", "http://10.0.0.5", "http://169.254.169.254/latest",
		"http://[::1]:8080", "http://metadata.google.internal", "ftp://edge.example.com", "http://user:pw@edge.example.com",
	} {
		_, err := service.CreateEdge(ctx, CreateEdgeInstanceInput{Name: "edge", BaseURL: baseURL, APIKey: "k"})
		if !errors.Is(err, ErrValidation) {
			t.Errorf("CreateEdge(%q) error = %v, want ErrValidation", baseURL, err)
		}
		_, err = service.UpdateEdge(ctx, uuid.New(), UpdateEdgeInstanceInput{BaseURL: baseURL})
		if !errors.Is(err, ErrValidation) {
			t.Errorf("UpdateEdge(%q) error = %v, want ErrValidation", baseURL, err)
		}
	}

	// Development policy admits loopback so local edge simulators keep working.
	permissive := NewService(nil, nil, box).WithOutboundPolicy(safeurl.Policy{AllowPrivate: true})
	if err := permissive.validateBaseURL(ctx, "http://127.0.0.1:8090"); err != nil {
		t.Fatalf("permissive policy rejected loopback: %v", err)
	}
}
