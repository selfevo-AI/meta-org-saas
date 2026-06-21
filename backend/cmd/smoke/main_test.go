package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEnsureTenantOrganizationCompletesOnboardingWhenRequired(t *testing.T) {
	const orgID = "11111111-1111-1111-1111-111111111111"
	var sawOnboarding bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/onboarding/organization" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer smoke-token" {
			t.Fatalf("Authorization = %q, want bearer token", got)
		}
		var body responseMap
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode onboarding body: %v", err)
		}
		if body["organization_name"] == "" {
			t.Fatalf("missing organization name in onboarding body")
		}
		if len(asList(body["enabled_modules"])) == 0 {
			t.Fatalf("missing enabled modules in onboarding body")
		}
		sawOnboarding = true
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(responseMap{
			"organization": responseMap{"id": orgID, "name": "Smoke Organization"},
			"profile":      responseMap{"default_organization_id": orgID},
		})
	}))
	defer server.Close()

	c := &client{base: server.URL, token: "smoke-token", http: server.Client()}
	gotOrgID := c.ensureTenantOrganization(responseMap{"onboarding_required": true}, "20260621010203")

	if !sawOnboarding {
		t.Fatalf("expected onboarding request")
	}
	if gotOrgID != orgID {
		t.Fatalf("org id = %q, want %q", gotOrgID, orgID)
	}
	if c.organizationID != orgID {
		t.Fatalf("client organization id = %q, want %q", c.organizationID, orgID)
	}
}

func TestDoSendsTenantHeaderWhenOrganizationSelected(t *testing.T) {
	const orgID = "22222222-2222-2222-2222-222222222222"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Organization-ID"); got != orgID {
			t.Fatalf("X-Organization-ID = %q, want %q", got, orgID)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(responseMap{"ok": true})
	}))
	defer server.Close()

	c := &client{base: server.URL, token: "smoke-token", organizationID: orgID, http: server.Client()}
	got := c.get("/tenant-route")

	if got["ok"] != true {
		t.Fatalf("unexpected response: %#v", got)
	}
}

func TestPostAcceptsCreatedForBusinessCreateEndpoints(t *testing.T) {
	paths := []string{
		"/inventory/partners",
		"/inventory/items",
		"/inventory/warehouses",
		"/inventory/locations",
		"/inventory/movements",
		"/inventory/transfers",
		"/inventory/adjustments",
		"/inventory/counts",
		"/procurement/requisitions",
		"/procurement/orders",
		"/procurement/receipts",
		"/procurement/returns",
		"/sales/quotations",
		"/sales/orders",
		"/sales/shipments",
		"/sales/returns",
		"/costing/currencies",
		"/costing/exchange-rates",
		"/costing/rate-cards",
		"/costing/ledger-entries",
		"/costing/budgets",
		"/finance/adapters",
		"/finance/export-batches",
		"/finance/imports",
		"/finance/settlement-orders",
		"/finance/settlement-orders/66666666-6666-6666-6666-666666666666/post",
		"/finance/receivables",
		"/finance/receipts",
		"/finance/receipts/33333333-3333-3333-3333-333333333333/allocate",
		"/finance/payables",
		"/finance/payments",
		"/finance/payments/44444444-4444-4444-4444-444444444444/allocate",
		"/platform/admin/organizations/55555555-5555-5555-5555-555555555555/schema/change-requests",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != path {
					t.Fatalf("path = %s, want %s", r.URL.Path, path)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_ = json.NewEncoder(w).Encode(responseMap{"id": "ok"})
			}))
			defer server.Close()

			c := &client{base: server.URL, token: "smoke-token", http: server.Client()}
			c.post(path, responseMap{})
		})
	}
}
