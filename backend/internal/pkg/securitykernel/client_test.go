package securitykernel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
