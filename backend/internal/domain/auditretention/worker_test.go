package auditretention

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeRetentionRepository struct {
	batches []RedactionCounts
	err     error
	record  RunRecord
	cutoffs []time.Time
}

func (f *fakeRetentionRepository) RedactBatch(_ context.Context, cutoff time.Time, _ int) (RedactionCounts, error) {
	f.cutoffs = append(f.cutoffs, cutoff)
	if f.err != nil {
		return RedactionCounts{}, f.err
	}
	if len(f.batches) == 0 {
		return RedactionCounts{}, nil
	}
	result := f.batches[0]
	f.batches = f.batches[1:]
	return result, nil
}

func (f *fakeRetentionRepository) RecordRun(_ context.Context, record RunRecord) error {
	f.record = record
	return nil
}

func TestWorkerRedactsUntilBatchIsEmptyAndRecordsAudit(t *testing.T) {
	now := time.Date(2026, 7, 13, 8, 0, 0, 0, time.UTC)
	repo := &fakeRetentionRepository{batches: []RedactionCounts{
		{AIInvocations: 2, AssistantMessages: 3},
		{BusinessAIRuns: 1},
		{},
	}}
	worker := NewWorker(repo, WorkerConfig{Enabled: true, RetentionDays: 30, BatchSize: 100, MaxBatches: 10})
	worker.now = func() time.Time { return now }

	counts, err := worker.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if counts.Total() != 6 || repo.record.Counts.Total() != 6 {
		t.Fatalf("redaction counts = %#v, record = %#v", counts, repo.record.Counts)
	}
	if repo.record.Status != "succeeded" || repo.record.RetentionDays != 30 {
		t.Fatalf("run record = %#v", repo.record)
	}
	wantCutoff := now.AddDate(0, 0, -30)
	if len(repo.cutoffs) != 3 || !repo.cutoffs[0].Equal(wantCutoff) {
		t.Fatalf("cutoffs = %#v, want %s", repo.cutoffs, wantCutoff)
	}
	stats := worker.Stats()
	if stats.RunsTotal != 1 || stats.RowsRedactedTotal != 6 || stats.FailuresTotal != 0 {
		t.Fatalf("worker stats = %#v", stats)
	}
}

func TestWorkerRecordsFailedRun(t *testing.T) {
	repo := &fakeRetentionRepository{err: errors.New("database unavailable")}
	worker := NewWorker(repo, WorkerConfig{Enabled: true})
	worker.now = func() time.Time { return time.Date(2026, 7, 13, 8, 0, 0, 0, time.UTC) }

	_, err := worker.RunOnce(context.Background())
	if err == nil {
		t.Fatal("RunOnce() error = nil, want failure")
	}
	if repo.record.Status != "failed" || repo.record.ErrorMessage == "" {
		t.Fatalf("failed run record = %#v", repo.record)
	}
	if worker.Stats().FailuresTotal != 1 {
		t.Fatalf("worker stats = %#v", worker.Stats())
	}
}
