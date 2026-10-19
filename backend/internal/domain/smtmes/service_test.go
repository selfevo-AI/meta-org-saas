package smtmes

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

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
