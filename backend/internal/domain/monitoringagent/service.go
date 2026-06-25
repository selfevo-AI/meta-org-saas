package monitoringagent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrValidation = errors.New("validation error")
)

type Repository interface {
	CreateRun(ctx context.Context, run *MonitoringAgentRun) error
	CompleteRun(ctx context.Context, run *MonitoringAgentRun) error
	CollectFindings(ctx context.Context, window ScanWindow) ([]OperationalFinding, error)
	HasOpenSignal(ctx context.Context, fingerprint string) (bool, error)
	CreateSignal(ctx context.Context, signal SignalWrite) error
	CreateContextProposal(ctx context.Context, proposal ContextProposalWrite) error
	ListRuns(ctx context.Context, filter ListRunsFilter) ([]MonitoringAgentRun, error)
	GetRun(ctx context.Context, id uuid.UUID) (*MonitoringAgentRun, error)
}

type Service struct {
	repo   Repository
	config ServiceConfig
}

func NewService(repo Repository, config ServiceConfig) *Service {
	if config.LookbackHours <= 0 {
		config.LookbackHours = 24
	}
	if config.MaxSignalsPerRun <= 0 {
		config.MaxSignalsPerRun = 100
	}
	if strings.TrimSpace(config.DailyTime) == "" {
		config.DailyTime = "02:00"
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	return &Service{repo: repo, config: config}
}

func (s *Service) Run(ctx context.Context, input RunInput) (*MonitoringAgentRun, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("%w: monitoring agent repository is required", ErrValidation)
	}
	trigger := strings.TrimSpace(input.TriggerType)
	if trigger == "" {
		trigger = TriggerManual
	}
	if trigger != TriggerManual && trigger != TriggerScheduled {
		return nil, fmt.Errorf("%w: invalid trigger type", ErrValidation)
	}
	lookbackHours := input.LookbackHours
	if lookbackHours <= 0 {
		lookbackHours = s.config.LookbackHours
	}
	endedAt := s.config.Now().UTC()
	startedAt := endedAt.Add(-time.Duration(lookbackHours) * time.Hour)
	run := &MonitoringAgentRun{
		ID:                uuid.New(),
		TriggerType:       trigger,
		OrganizationID:    input.OrganizationID,
		Status:            RunRunning,
		LookbackStartedAt: startedAt,
		LookbackEndedAt:   endedAt,
		StartedAt:         endedAt,
		Summary:           map[string]any{},
	}
	if err := s.repo.CreateRun(ctx, run); err != nil {
		return nil, err
	}

	findings, err := s.repo.CollectFindings(ctx, ScanWindow{
		OrganizationID: input.OrganizationID,
		StartedAt:      startedAt,
		EndedAt:        endedAt,
		Limit:          s.config.MaxSignalsPerRun * 4,
	})
	if err != nil {
		s.failRun(ctx, run, err)
		return run, err
	}

	sortFindings(findings)
	seen := map[string]bool{}
	byCategory := map[string]int{}
	contextProposalsCreated := 0
	capped := false
	for _, finding := range findings {
		if finding.Category == "" {
			continue
		}
		byCategory[finding.Category]++
		fingerprint := Fingerprint(finding)
		if seen[fingerprint] {
			run.DuplicatesSuppressed++
			continue
		}
		seen[fingerprint] = true
		exists, err := s.repo.HasOpenSignal(ctx, fingerprint)
		if err != nil {
			s.failRun(ctx, run, err)
			return run, err
		}
		if exists {
			run.DuplicatesSuppressed++
			continue
		}
		if run.SignalsCreated >= s.config.MaxSignalsPerRun {
			capped = true
			break
		}
		if err := s.repo.CreateSignal(ctx, signalFromFinding(finding, fingerprint)); err != nil {
			s.failRun(ctx, run, err)
			return run, err
		}
		if proposal, ok := contextProposalFromFinding(finding); ok {
			if err := s.repo.CreateContextProposal(ctx, proposal); err != nil {
				s.failRun(ctx, run, err)
				return run, err
			}
			contextProposalsCreated++
		}
		run.SignalsCreated++
	}

	run.Status = RunCompleted
	run.Summary = map[string]any{
		"total_findings":            len(findings),
		"by_category":               byCategory,
		"capped":                    capped,
		"lookback_hours":            lookbackHours,
		"max_signals_per_run":       s.config.MaxSignalsPerRun,
		"changes_applied":           0,
		"approval_bypasses":         0,
		"context_proposals_created": contextProposalsCreated,
		"proposal_generation_mode":  "pending_context_proposals_and_signal_payloads",
	}
	completedAt := s.config.Now().UTC()
	run.CompletedAt = &completedAt
	if err := s.repo.CompleteRun(ctx, run); err != nil {
		return run, err
	}
	return run, nil
}

func (s *Service) ListRuns(ctx context.Context, filter ListRunsFilter) ([]MonitoringAgentRun, error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	return s.repo.ListRuns(ctx, filter)
}

func (s *Service) GetRun(ctx context.Context, id uuid.UUID) (*MonitoringAgentRun, error) {
	if id == uuid.Nil {
		return nil, ErrValidation
	}
	return s.repo.GetRun(ctx, id)
}

func (s *Service) Status(ctx context.Context, organizationID *uuid.UUID) (*MonitoringAgentStatus, error) {
	runs, err := s.ListRuns(ctx, ListRunsFilter{OrganizationID: organizationID, Limit: 1})
	if err != nil {
		return nil, err
	}
	status := &MonitoringAgentStatus{
		SchedulerEnabled: s.config.SchedulerEnabled,
		DailyTime:        s.config.DailyTime,
		LookbackHours:    s.config.LookbackHours,
		MaxSignalsPerRun: s.config.MaxSignalsPerRun,
	}
	if len(runs) > 0 {
		latest := runs[0]
		status.LatestRun = &latest
	}
	return status, nil
}

func (s *Service) failRun(ctx context.Context, run *MonitoringAgentRun, err error) {
	run.Status = RunFailed
	run.ErrorMessage = err.Error()
	run.Summary = map[string]any{
		"changes_applied":   0,
		"approval_bypasses": 0,
	}
	completedAt := s.config.Now().UTC()
	run.CompletedAt = &completedAt
	_ = s.repo.CompleteRun(ctx, run)
}

func Fingerprint(finding OperationalFinding) string {
	orgID := "global"
	if finding.OrganizationID != nil {
		orgID = finding.OrganizationID.String()
	}
	parts := []string{
		strings.ToLower(strings.TrimSpace(finding.Category)),
		orgID,
		strings.ToLower(strings.TrimSpace(finding.EntityType)),
		strings.ToLower(strings.TrimSpace(finding.EntityID)),
		normalizeReason(finding.Reason),
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:])
}

func signalFromFinding(finding OperationalFinding, fingerprint string) SignalWrite {
	data := map[string]any{
		"fingerprint": fingerprint,
		"category":    finding.Category,
		"entity_type": finding.EntityType,
		"entity_id":   finding.EntityID,
		"reason":      finding.Reason,
		"severity":    finding.Severity,
	}
	if finding.OrganizationID != nil {
		data["organization_id"] = finding.OrganizationID.String()
	}
	if !finding.OccurredAt.IsZero() {
		data["occurred_at"] = finding.OccurredAt.UTC().Format(time.RFC3339)
	}
	if len(finding.Data) > 0 {
		data["source_data"] = finding.Data
	}
	if len(finding.ProposalPayload) > 0 {
		data["proposal_payload"] = finding.ProposalPayload
	}
	return SignalWrite{
		SignalType: finding.Category,
		Source:     "monitoring_agent",
		Priority:   priorityForSeverity(finding.Severity),
		Data:       data,
	}
}

func contextProposalFromFinding(finding OperationalFinding) (ContextProposalWrite, bool) {
	if finding.Category != SignalContextRuleGap || len(finding.ProposalPayload) == 0 {
		return ContextProposalWrite{}, false
	}
	rawID, ok := finding.ProposalPayload["dictionary_version_id"].(string)
	if !ok || strings.TrimSpace(rawID) == "" {
		return ContextProposalWrite{}, false
	}
	dictionaryID, err := uuid.Parse(rawID)
	if err != nil {
		return ContextProposalWrite{}, false
	}
	payload := make(map[string]any, len(finding.ProposalPayload)+4)
	for key, value := range finding.ProposalPayload {
		payload[key] = value
	}
	payload["source"] = "monitoring_agent"
	payload["requires_human_review"] = true
	payload["reason"] = finding.Reason
	payload["entity_id"] = finding.EntityID
	return ContextProposalWrite{
		DictionaryVersionID: dictionaryID,
		ProposalType:        "dictionary_change",
		Title:               "Monitoring agent context rule gap",
		Summary:             firstNonEmpty(finding.Reason, "Context package omitted required business context"),
		Payload:             payload,
		Status:              "pending",
	}, true
}

func sortFindings(findings []OperationalFinding) {
	sort.SliceStable(findings, func(i, j int) bool {
		left, right := findings[i], findings[j]
		if priorityForSeverity(left.Severity) != priorityForSeverity(right.Severity) {
			return priorityForSeverity(left.Severity) > priorityForSeverity(right.Severity)
		}
		if !left.OccurredAt.Equal(right.OccurredAt) {
			return left.OccurredAt.After(right.OccurredAt)
		}
		return Fingerprint(left) < Fingerprint(right)
	})
}

func priorityForSeverity(severity string) int {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case SeverityCritical:
		return 90
	case SeverityHigh:
		return 70
	case SeverityMedium:
		return 50
	case SeverityLow:
		return 30
	default:
		return 10
	}
}

func normalizeReason(reason string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(reason))), " ")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
