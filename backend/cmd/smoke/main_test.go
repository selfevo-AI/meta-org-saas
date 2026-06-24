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
		"/erp/MCRD",
		"/erp/MITM",
		"/erp/MWHS",
		"/erp/MPOR",
		"/erp/MPOR/1001/POR1",
		"/erp/MRDR",
		"/erp/MRDR/2001/RDR1",
		"/erp/MINV",
		"/erp/MINV/3001/INV1",
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

func TestRequireActionContractAcceptsExecutionMetadata(t *testing.T) {
	requireActionContract(responseMap{
		"execution_id":          "exec-1",
		"idempotency_key":       "idem-1",
		"preconditions_checked": []any{responseMap{"key": "MREQ.Status", "status": "passed"}},
	}, "action")
}

func TestRequireGeneratedProvenanceAcceptsGeneratedRecord(t *testing.T) {
	requireGeneratedProvenance(responseMap{
		"generated_records": []any{
			responseMap{
				"table_code": "MPRJ",
				"key":        "PRJ-1",
				"data": responseMap{
					"provenance": responseMap{"source_table_code": "MREQ"},
				},
			},
		},
	}, "MPRJ")
}
