package monitoringagent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestServiceRunCreatesSignalsAndSuppressesUnacknowledgedDuplicates(t *testing.T) {
	orgID := uuid.New()
	sourceID := uuid.New()
	repo := &memoryRepository{
		findings: []OperationalFinding{
			{
				Category:       SignalAIFailure,
				OrganizationID: &orgID,
				EntityType:     "ai_invocation",
				EntityID:       sourceID.String(),
				Reason:         "provider timeout",
				Severity:       SeverityHigh,
				OccurredAt:     time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC),
				Data:           map[string]any{"status": "failed"},
			},
			{
				Category:       SignalAIFailure,
				OrganizationID: &orgID,
				EntityType:     "ai_invocation",
				EntityID:       sourceID.String(),
				Reason:         "provider timeout",
				Severity:       SeverityHigh,
				OccurredAt:     time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC),
				Data:           map[string]any{"status": "failed"},
			},
		},
		existingFingerprints: map[string]bool{
			Fingerprint(OperationalFinding{
				Category:       SignalAIFailure,
				OrganizationID: &orgID,
				EntityType:     "ai_invocation",
				EntityID:       sourceID.String(),
				Reason:         "provider timeout",
			}): true,
		},
	}
	service := NewService(repo, ServiceConfig{MaxSignalsPerRun: 100})

	run, err := service.Run(context.Background(), RunInput{
		TriggerType:    TriggerManual,
		OrganizationID: &orgID,
		LookbackHours:  24,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if run.Status != RunCompleted {
		t.Fatalf("status = %q, want %q", run.Status, RunCompleted)
	}
	if run.SignalsCreated != 0 {
		t.Fatalf("signals created = %d, want 0", run.SignalsCreated)
	}
	if run.DuplicatesSuppressed != 2 {
		t.Fatalf("duplicates suppressed = %d, want 2", run.DuplicatesSuppressed)
	}
	if len(repo.createdSignals) != 0 {
		t.Fatalf("created signals = %d, want 0", len(repo.createdSignals))
	}
}

func TestServiceRunCapsCreatedSignalsAndRecordsSummary(t *testing.T) {
	repo := &memoryRepository{}
	for i := 0; i < 3; i++ {
		repo.findings = append(repo.findings, OperationalFinding{
			Category:   SignalToolFailure,
			EntityType: "tool_execution",
			EntityID:   uuid.NewString(),
			Reason:     "tool failed",
			Severity:   SeverityMedium,
			OccurredAt: time.Date(2026, 6, 25, 11, i, 0, 0, time.UTC),
		})
	}
	service := NewService(repo, ServiceConfig{MaxSignalsPerRun: 2})

	run, err := service.Run(context.Background(), RunInput{TriggerType: TriggerManual, LookbackHours: 24})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if run.SignalsCreated != 2 {
		t.Fatalf("signals created = %d, want 2", run.SignalsCreated)
	}
	if run.DuplicatesSuppressed != 0 {
		t.Fatalf("duplicates suppressed = %d, want 0", run.DuplicatesSuppressed)
	}
	if got := run.Summary["capped"]; got != true {
		t.Fatalf("summary capped = %v, want true", got)
	}
	if len(repo.createdSignals) != 2 {
		t.Fatalf("created signal count = %d, want 2", len(repo.createdSignals))
	}
}

func TestServiceRunFailsWithoutApplyingChanges(t *testing.T) {
	repo := &memoryRepository{collectErr: errors.New("warehouse offline")}
	service := NewService(repo, ServiceConfig{MaxSignalsPerRun: 100})

	run, err := service.Run(context.Background(), RunInput{TriggerType: TriggerManual, LookbackHours: 24})
	if err == nil {
		t.Fatal("Run returned nil error, want collection error")
	}
	if run == nil {
		t.Fatal("run is nil, want failed run record")
	}
	if run.Status != RunFailed {
		t.Fatalf("status = %q, want %q", run.Status, RunFailed)
	}
	if len(repo.createdSignals) != 0 {
		t.Fatalf("created signals = %d, want 0", len(repo.createdSignals))
	}
	if repo.appliedChanges != 0 {
		t.Fatalf("applied changes = %d, want 0", repo.appliedChanges)
	}
}

func TestServiceRunCreatesPendingContextProposalForRuleGap(t *testing.T) {
	dictionaryID := uuid.New()
	repo := &memoryRepository{
		findings: []OperationalFinding{
			{
				Category:   SignalContextRuleGap,
				EntityType: "context_package",
				EntityID:   uuid.NewString(),
				Reason:     "finance validation omitted",
				Severity:   SeverityMedium,
				ProposalPayload: map[string]any{
					"dictionary_version_id": dictionaryID.String(),
					"omissions":             []any{"finance.amount"},
				},
			},
		},
	}
	service := NewService(repo, ServiceConfig{MaxSignalsPerRun: 100})

	run, err := service.Run(context.Background(), RunInput{TriggerType: TriggerManual, LookbackHours: 24})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if run.SignalsCreated != 1 {
		t.Fatalf("signals created = %d, want 1", run.SignalsCreated)
	}
	if len(repo.createdProposals) != 1 {
		t.Fatalf("created proposals = %d, want 1", len(repo.createdProposals))
	}
	if repo.createdProposals[0].Status != "pending" {
		t.Fatalf("proposal status = %q, want pending", repo.createdProposals[0].Status)
	}
	if repo.appliedChanges != 0 {
		t.Fatalf("applied changes = %d, want 0", repo.appliedChanges)
	}
}

type memoryRepository struct {
	runs                 []MonitoringAgentRun
	findings             []OperationalFinding
	existingFingerprints map[string]bool
	createdSignals       []SignalWrite
	createdProposals     []ContextProposalWrite
	collectErr           error
	appliedChanges       int
}

func (r *memoryRepository) CreateRun(_ context.Context, run *MonitoringAgentRun) error {
	if run.ID == uuid.Nil {
		run.ID = uuid.New()
	}
	r.runs = append(r.runs, *run)
	return nil
}

func (r *memoryRepository) CompleteRun(_ context.Context, run *MonitoringAgentRun) error {
	if len(r.runs) == 0 {
		return errors.New("missing run")
	}
	r.runs[len(r.runs)-1] = *run
	return nil
}

func (r *memoryRepository) CollectFindings(_ context.Context, window ScanWindow) ([]OperationalFinding, error) {
	if r.collectErr != nil {
		return nil, r.collectErr
	}
	return r.findings, nil
}

func (r *memoryRepository) HasOpenSignal(_ context.Context, fingerprint string) (bool, error) {
	return r.existingFingerprints != nil && r.existingFingerprints[fingerprint], nil
}

func (r *memoryRepository) CreateSignal(_ context.Context, signal SignalWrite) error {
	r.createdSignals = append(r.createdSignals, signal)
	return nil
}

func (r *memoryRepository) CreateContextProposal(_ context.Context, proposal ContextProposalWrite) error {
	r.createdProposals = append(r.createdProposals, proposal)
	return nil
}

func (r *memoryRepository) ListRuns(_ context.Context, _ ListRunsFilter) ([]MonitoringAgentRun, error) {
	return r.runs, nil
}

func (r *memoryRepository) GetRun(_ context.Context, id uuid.UUID) (*MonitoringAgentRun, error) {
	for _, run := range r.runs {
		if run.ID == id {
			copy := run
			return &copy, nil
		}
	}
	return nil, ErrNotFound
}
