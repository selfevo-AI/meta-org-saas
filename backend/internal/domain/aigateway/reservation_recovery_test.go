package aigateway

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeReservationRecoveryRepository struct {
	reservations  []StaleBalanceReservation
	outcomes      map[uuid.UUID]string
	errors        map[uuid.UUID]error
	released      []uuid.UUID
	claimWorkerID string
	claimCutoff   time.Time
	claimLease    time.Duration
	claimLimit    int
}

func (f *fakeReservationRecoveryRepository) ClaimStaleBalanceReservations(_ context.Context, workerID string, cutoff time.Time, lease time.Duration, limit int) ([]StaleBalanceReservation, error) {
	f.claimWorkerID = workerID
	f.claimCutoff = cutoff
	f.claimLease = lease
	f.claimLimit = limit
	return f.reservations, nil
}

func (f *fakeReservationRecoveryRepository) RecoverStaleBalanceReservation(_ context.Context, _ string, reservation StaleBalanceReservation) (string, error) {
	return f.outcomes[reservation.ID], f.errors[reservation.ID]
}

func (f *fakeReservationRecoveryRepository) ReleaseBalanceReservationRecoveryLease(_ context.Context, _ string, reservationID uuid.UUID) error {
	f.released = append(f.released, reservationID)
	return nil
}

func TestReservationRecoveryWorkerUsesBoundedDefaults(t *testing.T) {
	worker := NewReservationRecoveryWorker(nil, ReservationRecoveryWorkerConfig{WorkerID: "recovery-test", BatchSize: 5001})
	if worker.config.StaleAfter != 30*time.Minute || worker.config.PollInterval != 5*time.Minute || worker.config.LeaseDuration != time.Minute || worker.config.BatchSize != 100 {
		t.Fatalf("worker defaults = stale:%s poll:%s lease:%s batch:%d", worker.config.StaleAfter, worker.config.PollInterval, worker.config.LeaseDuration, worker.config.BatchSize)
	}
	if worker.Stats().WorkerID != "recovery-test" {
		t.Fatalf("worker ID = %q", worker.Stats().WorkerID)
	}
}

func TestReservationRecoveryWorkerRecordsOutcomesAndReleasesFailedLease(t *testing.T) {
	now := time.Date(2026, 7, 13, 8, 0, 0, 0, time.UTC)
	refundedID, settledID, abandonedID, failedID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	repo := &fakeReservationRecoveryRepository{
		reservations: []StaleBalanceReservation{{ID: refundedID}, {ID: settledID}, {ID: abandonedID}, {ID: failedID}},
		outcomes:     map[uuid.UUID]string{refundedID: "refunded", settledID: "settled", abandonedID: "abandoned"},
		errors:       map[uuid.UUID]error{failedID: errors.New("database unavailable")},
	}
	worker := NewReservationRecoveryWorker(repo, ReservationRecoveryWorkerConfig{
		Enabled: true, WorkerID: "recovery-test", StaleAfter: 20 * time.Minute,
		PollInterval: time.Minute, LeaseDuration: 30 * time.Second, BatchSize: 25,
	})
	worker.now = func() time.Time { return now }

	counts, err := worker.RunOnce(context.Background())
	if err == nil || !errors.Is(err, repo.errors[failedID]) {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if counts.Claimed != 4 || counts.Refunded != 1 || counts.Settled != 1 || counts.Abandoned != 1 || counts.Failed != 1 {
		t.Fatalf("counts = %#v", counts)
	}
	if repo.claimWorkerID != "recovery-test" || !repo.claimCutoff.Equal(now.Add(-20*time.Minute)) || repo.claimLease != 30*time.Second || repo.claimLimit != 25 {
		t.Fatalf("claim parameters = worker:%s cutoff:%s lease:%s limit:%d", repo.claimWorkerID, repo.claimCutoff, repo.claimLease, repo.claimLimit)
	}
	if len(repo.released) != 1 || repo.released[0] != failedID {
		t.Fatalf("released leases = %#v", repo.released)
	}
	stats := worker.Stats()
	if stats.RunsTotal != 1 || stats.FailuresTotal != 1 || stats.RecoveredTotal != 3 || stats.LastError == "" {
		t.Fatalf("stats = %#v", stats)
	}
}
