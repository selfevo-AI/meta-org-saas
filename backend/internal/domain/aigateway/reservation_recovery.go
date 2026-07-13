package aigateway

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type StaleBalanceReservation struct {
	ID           uuid.UUID
	InvocationID *uuid.UUID
}

type ReservationRecoveryCounts struct {
	Claimed   int `json:"claimed"`
	Refunded  int `json:"refunded"`
	Settled   int `json:"settled"`
	Abandoned int `json:"abandoned"`
	Failed    int `json:"failed"`
}

func (c *ReservationRecoveryCounts) Add(other ReservationRecoveryCounts) {
	c.Claimed += other.Claimed
	c.Refunded += other.Refunded
	c.Settled += other.Settled
	c.Abandoned += other.Abandoned
	c.Failed += other.Failed
}

type ReservationRecoveryRepository interface {
	ClaimStaleBalanceReservations(context.Context, string, time.Time, time.Duration, int) ([]StaleBalanceReservation, error)
	RecoverStaleBalanceReservation(context.Context, string, StaleBalanceReservation) (string, error)
	ReleaseBalanceReservationRecoveryLease(context.Context, string, uuid.UUID) error
}

type ReservationRecoveryWorkerConfig struct {
	Enabled       bool
	WorkerID      string
	StaleAfter    time.Duration
	PollInterval  time.Duration
	LeaseDuration time.Duration
	BatchSize     int
}

type ReservationRecoveryWorkerStats struct {
	WorkerID       string                    `json:"worker_id"`
	Running        bool                      `json:"running"`
	LastStartedAt  time.Time                 `json:"last_started_at"`
	LastFinishedAt time.Time                 `json:"last_finished_at"`
	LastCounts     ReservationRecoveryCounts `json:"last_counts"`
	RunsTotal      uint64                    `json:"runs_total"`
	FailuresTotal  uint64                    `json:"failures_total"`
	RecoveredTotal uint64                    `json:"recovered_total"`
	LastError      string                    `json:"last_error,omitempty"`
}

type ReservationRecoveryWorker struct {
	repo   ReservationRecoveryRepository
	config ReservationRecoveryWorkerConfig
	now    func() time.Time
	mu     sync.Mutex
	stats  ReservationRecoveryWorkerStats
}

func NewReservationRecoveryWorker(repo ReservationRecoveryRepository, config ReservationRecoveryWorkerConfig) *ReservationRecoveryWorker {
	if config.WorkerID == "" {
		config.WorkerID = "ai-reservation-" + uuid.NewString()
	}
	if config.StaleAfter <= 0 {
		config.StaleAfter = 30 * time.Minute
	}
	if config.PollInterval <= 0 {
		config.PollInterval = 5 * time.Minute
	}
	if config.LeaseDuration <= 0 {
		config.LeaseDuration = time.Minute
	}
	if config.BatchSize <= 0 || config.BatchSize > 1000 {
		config.BatchSize = 100
	}
	return &ReservationRecoveryWorker{
		repo: repo, config: config, now: time.Now,
		stats: ReservationRecoveryWorkerStats{WorkerID: config.WorkerID},
	}
}

func (w *ReservationRecoveryWorker) Start(ctx context.Context) {
	if w == nil || w.repo == nil || !w.config.Enabled {
		return
	}
	w.setRunning(true)
	go func() {
		defer w.setRunning(false)
		if _, err := w.RunOnce(ctx); err != nil {
			log.Printf("ai gateway reservation recovery failed: %v", err)
		}
		ticker := time.NewTicker(w.config.PollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := w.RunOnce(ctx); err != nil {
					log.Printf("ai gateway reservation recovery failed: %v", err)
				}
			}
		}
	}()
}

func (w *ReservationRecoveryWorker) RunOnce(ctx context.Context) (ReservationRecoveryCounts, error) {
	started := w.now().UTC()
	w.beginRun(started)
	reservations, err := w.repo.ClaimStaleBalanceReservations(ctx, w.config.WorkerID, started.Add(-w.config.StaleAfter), w.config.LeaseDuration, w.config.BatchSize)
	if err != nil {
		w.finishRun(ReservationRecoveryCounts{}, err)
		return ReservationRecoveryCounts{}, err
	}
	counts := ReservationRecoveryCounts{Claimed: len(reservations)}
	var runErr error
	for _, reservation := range reservations {
		outcome, recoverErr := w.repo.RecoverStaleBalanceReservation(ctx, w.config.WorkerID, reservation)
		if recoverErr != nil {
			counts.Failed++
			runErr = errors.Join(runErr, recoverErr)
			if releaseErr := w.repo.ReleaseBalanceReservationRecoveryLease(ctx, w.config.WorkerID, reservation.ID); releaseErr != nil {
				runErr = errors.Join(runErr, releaseErr)
			}
			continue
		}
		switch outcome {
		case "refunded":
			counts.Refunded++
		case "abandoned":
			counts.Abandoned++
		case "settled":
			counts.Settled++
		}
	}
	w.finishRun(counts, runErr)
	return counts, runErr
}

func (w *ReservationRecoveryWorker) Stats() ReservationRecoveryWorkerStats {
	if w == nil {
		return ReservationRecoveryWorkerStats{}
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.stats
}

func (w *ReservationRecoveryWorker) setRunning(running bool) {
	w.mu.Lock()
	w.stats.Running = running
	w.mu.Unlock()
}

func (w *ReservationRecoveryWorker) beginRun(started time.Time) {
	w.mu.Lock()
	w.stats.LastStartedAt = started
	w.mu.Unlock()
}

func (w *ReservationRecoveryWorker) finishRun(counts ReservationRecoveryCounts, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.stats.LastFinishedAt = w.now().UTC()
	w.stats.LastCounts = counts
	w.stats.RunsTotal++
	w.stats.RecoveredTotal += uint64(counts.Refunded + counts.Settled + counts.Abandoned)
	w.stats.LastError = ""
	if err != nil {
		w.stats.FailuresTotal++
		w.stats.LastError = err.Error()
	}
}

func (r *PostgresRepository) AttachBalanceReservation(ctx context.Context, reservationID uuid.UUID, invocationID uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE ai_gateway_balance_transactions
		SET invocation_id = $2
		WHERE id = $1 AND transaction_type = 'reserve'
	`, reservationID, invocationID)
	if err != nil {
		return fmt.Errorf("attach ai gateway balance reservation: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("%w: balance reservation not found", ErrNotFound)
	}
	return nil
}

func (r *PostgresRepository) ClaimStaleBalanceReservations(ctx context.Context, workerID string, cutoff time.Time, lease time.Duration, limit int) ([]StaleBalanceReservation, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	rows, err := r.db.Query(ctx, `
		WITH candidates AS (
			SELECT reserve.id
			FROM ai_gateway_balance_transactions reserve
			WHERE reserve.transaction_type = 'reserve'
			  AND reserve.created_at < $2
			  AND (reserve.reconcile_lease_expires_at IS NULL OR reserve.reconcile_lease_expires_at < NOW())
			  AND NOT EXISTS (
				SELECT 1 FROM ai_gateway_balance_transactions finished
				WHERE finished.reservation_id = reserve.id
				  AND finished.transaction_type IN ('settle', 'refund')
			  )
			ORDER BY reserve.created_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT $4
		)
		UPDATE ai_gateway_balance_transactions reserve
		SET reconcile_lease_owner = $1,
			reconcile_lease_expires_at = NOW() + ($3 * INTERVAL '1 second')
		FROM candidates
		WHERE reserve.id = candidates.id
		RETURNING reserve.id, reserve.invocation_id
	`, workerID, cutoff, int64(lease/time.Second), limit)
	if err != nil {
		return nil, fmt.Errorf("claim stale ai gateway reservations: %w", err)
	}
	defer rows.Close()
	reservations := make([]StaleBalanceReservation, 0, limit)
	for rows.Next() {
		var reservation StaleBalanceReservation
		var invocationID pgtype.UUID
		if err := rows.Scan(&reservation.ID, &invocationID); err != nil {
			return nil, fmt.Errorf("scan stale ai gateway reservation: %w", err)
		}
		reservation.InvocationID = uuidPointer(invocationID)
		reservations = append(reservations, reservation)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate stale ai gateway reservations: %w", err)
	}
	return reservations, nil
}

func (r *PostgresRepository) RecoverStaleBalanceReservation(ctx context.Context, workerID string, reservation StaleBalanceReservation) (string, error) {
	var leaseOwner string
	err := r.db.QueryRow(ctx, `
		SELECT reconcile_lease_owner
		FROM ai_gateway_balance_transactions
		WHERE id = $1 AND transaction_type = 'reserve'
	`, reservation.ID).Scan(&leaseOwner)
	if err != nil {
		return "", fmt.Errorf("verify ai gateway reservation recovery lease: %w", err)
	}
	if leaseOwner != workerID {
		return "", fmt.Errorf("%w: AI gateway reservation recovery lease is not owned by this worker", ErrUnavailable)
	}
	if reservation.InvocationID == nil {
		if err := r.RefundAccessTokenBalance(ctx, reservation.ID, "stale_unattached_reservation"); err != nil {
			return "", err
		}
		return "refunded", nil
	}
	var status string
	var cost float64
	var channelID pgtype.UUID
	err = r.db.QueryRow(ctx, `
		SELECT invocation.status,
			COALESCE(
				NULLIF(invocation.cost_amount, 0),
				(SELECT SUM(ledger.actual_amount) FROM ai_usage_ledger ledger WHERE ledger.invocation_id = invocation.id),
				0
			)::float8,
			invocation.channel_id
		FROM ai_invocations invocation
		WHERE invocation.id = $1
	`, *reservation.InvocationID).Scan(&status, &cost, &channelID)
	if errors.Is(err, pgx.ErrNoRows) {
		if err := r.RefundAccessTokenBalance(ctx, reservation.ID, "stale_missing_invocation"); err != nil {
			return "", err
		}
		return "refunded", nil
	}
	if err != nil {
		return "", fmt.Errorf("load stale ai gateway invocation: %w", err)
	}
	outcome := "settled"
	if status == StatusStarted || status == StatusStreaming {
		var releasedChannel pgtype.UUID
		err := r.db.QueryRow(ctx, `
			UPDATE ai_invocations
			SET status = $2,
				error_type = 'recovered_after_restart',
				error_message = 'stale AI invocation recovered after backend interruption',
				completed_at = NOW()
			WHERE id = $1 AND status IN ('started', 'streaming')
			RETURNING channel_id
		`, *reservation.InvocationID, StatusCancelled).Scan(&releasedChannel)
		if err == nil {
			if id := uuidPointer(releasedChannel); id != nil {
				if err := r.ReleaseChannel(ctx, id, cost); err != nil {
					return "", err
				}
			}
			outcome = "abandoned"
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("cancel stale ai gateway invocation: %w", err)
		}
	}
	if err := r.SettleAccessTokenBalance(ctx, SettleBalanceInput{
		ReservationID: reservation.ID,
		ActualAmount:  cost,
		Currency:      "",
		Reason:        "stale_reservation_recovery",
	}); err != nil {
		return "", err
	}
	return outcome, nil
}

func (r *PostgresRepository) ReleaseBalanceReservationRecoveryLease(ctx context.Context, workerID string, reservationID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		UPDATE ai_gateway_balance_transactions
		SET reconcile_lease_owner = '', reconcile_lease_expires_at = NULL
		WHERE id = $1 AND reconcile_lease_owner = $2 AND transaction_type = 'reserve'
	`, reservationID, workerID)
	if err != nil {
		return fmt.Errorf("release ai gateway reservation recovery lease: %w", err)
	}
	return nil
}
