package monitoringagent

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestHandlerRunAndListRuns(t *testing.T) {
	orgID := uuid.New()
	repo := &memoryRepository{
		findings: []OperationalFinding{
			{
				Category:       SignalContextBuildFailure,
				OrganizationID: &orgID,
				EntityType:     "context_package",
				EntityID:       uuid.NewString(),
				Reason:         "missing finance rule",
				Severity:       SeverityHigh,
			},
		},
	}
	handler := NewHandler(NewService(repo, ServiceConfig{MaxSignalsPerRun: 100}))
	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	body := `{"organization_id":"` + orgID.String() + `","lookback_hours":12}`
	request := httptest.NewRequest(http.MethodPost, "/monitoring-agent/runs", strings.NewReader(body))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusCreated, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"status":"completed"`) {
		t.Fatalf("response missing completed status: %s", response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"signals_created":1`) {
		t.Fatalf("response missing signal count: %s", response.Body.String())
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/monitoring-agent/runs?limit=5", nil)
	listResponse := httptest.NewRecorder()
	router.ServeHTTP(listResponse, listRequest)

	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d; body=%s", listResponse.Code, http.StatusOK, listResponse.Body.String())
	}
	if !strings.Contains(listResponse.Body.String(), `"runs"`) || !strings.Contains(listResponse.Body.String(), `"trigger_type":"manual"`) {
		t.Fatalf("list response missing runs: %s", listResponse.Body.String())
	}

	statusRequest := httptest.NewRequest(http.MethodGet, "/monitoring-agent/status", nil)
	statusResponse := httptest.NewRecorder()
	router.ServeHTTP(statusResponse, statusRequest)

	if statusResponse.Code != http.StatusOK {
		t.Fatalf("status response = %d, want %d; body=%s", statusResponse.Code, http.StatusOK, statusResponse.Body.String())
	}
	if !strings.Contains(statusResponse.Body.String(), `"scheduler_enabled":false`) {
		t.Fatalf("status response missing scheduler state: %s", statusResponse.Body.String())
	}
}

func TestHandlerRejectsInvalidRunBody(t *testing.T) {
	handler := NewHandler(NewService(&memoryRepository{}, ServiceConfig{MaxSignalsPerRun: 100}))
	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	request := httptest.NewRequest(http.MethodPost, "/monitoring-agent/runs", strings.NewReader(`{"organization_id":"bad"}`))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}
