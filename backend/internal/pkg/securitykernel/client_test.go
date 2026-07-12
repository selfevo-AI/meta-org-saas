package securitykernel

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestClientAuthorizeSignsRequestAndAllowsDecision(t *testing.T) {
	secret := "test-secret"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/authorize" {
			t.Fatalf("path = %s, want /v1/authorize", r.URL.Path)
		}
		if r.Header.Get("X-Security-Timestamp") == "" {
			t.Fatalf("missing X-Security-Timestamp")
		}
		if r.Header.Get("X-Security-Nonce") == "" {
			t.Fatalf("missing X-Security-Nonce")
		}
		if r.Header.Get("X-Security-Signature") == "" {
			t.Fatalf("missing X-Security-Signature")
		}
		_ = json.NewEncoder(w).Encode(Decision{Allowed: true, Reason: "allowed", DecisionType: "allow"})
	}))
	defer server.Close()

	client := NewClient(Config{URL: server.URL, SharedSecret: secret, EnforcementMode: "blocking"})
	decision, err := client.Authorize(context.Background(), Request{
		Actor:    Actor{ActorID: uuid.New(), ActorType: "human", AuthorityTier: "organization_creator"},
		Resource: Resource{ModuleKey: "assistant", ResourceType: "skill", Action: "activate"},
	})

	if err != nil {
		t.Fatalf("Authorize returned error: %v", err)
	}
	if !decision.Allowed {
		t.Fatalf("decision allowed = false, reason = %s", decision.Reason)
	}
}

func TestClientAuthorizeDeniesWhenKernelDenies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(Decision{Allowed: false, Reason: "license_denied", DecisionType: "deny"})
	}))
	defer server.Close()

	client := NewClient(Config{URL: server.URL, SharedSecret: "test-secret", EnforcementMode: "blocking"})
	decision, err := client.Authorize(context.Background(), Request{
		Actor:    Actor{ActorID: uuid.New(), ActorType: "human"},
		Resource: Resource{ModuleKey: "ai_gateway", ResourceType: "model", Action: "use"},
	})

	if err == nil {
		t.Fatalf("Authorize error = nil, want denial error")
	}
	if decision.Allowed {
		t.Fatalf("decision allowed = true, want false")
	}
}

func TestClientAuthorizeFailsClosedWhenKernelUnavailable(t *testing.T) {
	client := NewClient(Config{URL: "http://127.0.0.1:1", SharedSecret: "test-secret", EnforcementMode: "blocking"})

	decision, err := client.Authorize(context.Background(), Request{
		Actor:    Actor{ActorID: uuid.New(), ActorType: "human"},
		Resource: Resource{ModuleKey: "assistant", ResourceType: "skill", Action: "create"},
	})

	if err == nil {
		t.Fatalf("Authorize error = nil, want unavailable error")
	}
	if decision.Allowed {
		t.Fatalf("decision allowed = true, want fail-closed false")
	}
}

func TestNoopClientAllowsWhenKernelNotConfigured(t *testing.T) {
	client := NewNoopClient()

	decision, err := client.Authorize(context.Background(), Request{
		Actor:    Actor{ActorID: uuid.New(), ActorType: "human"},
		Resource: Resource{ModuleKey: "assistant", ResourceType: "skill", Action: "create"},
	})

	if err != nil {
		t.Fatalf("Authorize returned error: %v", err)
	}
	if !decision.Allowed {
		t.Fatalf("decision allowed = false, want true")
	}
}

func TestClientFailsClosedWhenRequiredKernelURLMissing(t *testing.T) {
	client := NewClient(Config{Required: true, EnforcementMode: "blocking"})

	decision, err := client.Authorize(context.Background(), Request{})
	if err == nil || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Authorize error = %v, want ErrUnavailable", err)
	}
	if decision.Allowed || decision.Reason != "security_kernel_url_required" {
		t.Fatalf("decision = %#v", decision)
	}
}

func TestClientFailsClosedWhenConfiguredSecretMissing(t *testing.T) {
	client := NewClient(Config{URL: "http://security-kernel:8090", EnforcementMode: "blocking"})

	decision, err := client.Authorize(context.Background(), Request{})
	if err == nil || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Authorize error = %v, want ErrUnavailable", err)
	}
	if decision.Allowed || decision.Reason != "security_kernel_shared_secret_required" {
		t.Fatalf("decision = %#v", decision)
	}
}

func TestValidateConfigRequiresProductionSecuritySettings(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{name: "missing url", config: Config{Required: true, SharedSecret: strings.Repeat("s", 32), EnforcementMode: "blocking"}},
		{name: "invalid url", config: Config{Required: true, URL: "security-kernel:8090", SharedSecret: strings.Repeat("s", 32), EnforcementMode: "blocking"}},
		{name: "short secret", config: Config{Required: true, URL: "http://security-kernel:8090", SharedSecret: "short", EnforcementMode: "blocking"}},
		{name: "audit mode", config: Config{Required: true, URL: "http://security-kernel:8090", SharedSecret: strings.Repeat("s", 32), EnforcementMode: "audit"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := ValidateConfig(test.config); err == nil {
				t.Fatal("ValidateConfig() error = nil")
			}
		})
	}
	if err := ValidateConfig(Config{
		Required: true, URL: "http://security-kernel:8090", SharedSecret: strings.Repeat("s", 32), EnforcementMode: "blocking",
	}); err != nil {
		t.Fatalf("ValidateConfig() error = %v", err)
	}
}

func TestHTTPClientHealthReflectsKernelReadiness(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	client := NewClient(Config{URL: server.URL, SharedSecret: strings.Repeat("s", 32)}).(HealthChecker)
	if err := client.CheckHealth(context.Background()); err == nil || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("CheckHealth() error = %v, want ErrUnavailable", err)
	}
}
