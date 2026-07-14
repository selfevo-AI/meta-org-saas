package tenantprojection

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/panicguard"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/tenantdb"
)

type PlatformRepository interface {
	ListProvisionedTargets(context.Context, int) ([]tenantdb.Target, error)
	ApplyProjection(context.Context, []OutboxEvent, OperationalSnapshot, []ActivityItem, []WorkflowEntity, time.Time) (ProjectionFreshness, error)
}

type TenantPool interface {
	AcquireTarget(context.Context, tenantdb.Target) (tenantdb.DB, func(), error)
}

type MigrationRecorder interface {
	RecordTenantDatabaseMigrationResult(context.Context, tenantdb.Target, tenantdb.MigrationResult) error
	UpdateTenantDatabaseTarget(context.Context, tenantdb.Target) error
}

type WorkerConfig struct {
	WorkerID      string
	AdminURL      string
	PollInterval  time.Duration
	LeaseDuration time.Duration
	RetryDelay    time.Duration
	BatchSize     int
	TargetLimit   int
	ActivityLimit int
	MaxAttempts   int
}

type Worker struct {
	repo     PlatformRepository
	pool     TenantPool
	migrator tenantdb.TenantMigrator
	recorder MigrationRecorder
	config   WorkerConfig
	now      func() time.Time
	migrated sync.Map

	statsMu sync.Mutex
	stats   WorkerStats
}

func NewWorker(repo PlatformRepository, pool TenantPool, migrator tenantdb.TenantMigrator, recorder MigrationRecorder, config WorkerConfig) *Worker {
	if strings.TrimSpace(config.WorkerID) == "" {
		config.WorkerID = "tenant-projection-" + uuid.NewString()
	}
	if config.PollInterval <= 0 {
		config.PollInterval = 2 * time.Second
	}
	if config.LeaseDuration <= 0 {
		config.LeaseDuration = time.Minute
	}
	if config.RetryDelay <= 0 {
		config.RetryDelay = 5 * time.Second
	}
	if config.BatchSize <= 0 || config.BatchSize > 1000 {
		config.BatchSize = 100
	}
	if config.TargetLimit <= 0 || config.TargetLimit > 1000 {
		config.TargetLimit = 100
	}
	if config.ActivityLimit <= 0 || config.ActivityLimit > 200 {
		config.ActivityLimit = 50
	}
	if config.MaxAttempts <= 0 {
		config.MaxAttempts = 20
	}
	return &Worker{
		repo: repo, pool: pool, migrator: migrator, recorder: recorder, config: config, now: time.Now,
		stats: WorkerStats{WorkerID: config.WorkerID},
	}
}

func (w *Worker) Start(ctx context.Context) {
	if w == nil || w.repo == nil || w.pool == nil || w.migrator == nil {
		return
	}
	w.setRunning(true)
	go func() {
		defer w.setRunning(false)
		ticker := time.NewTicker(w.config.PollInterval)
		defer ticker.Stop()
		for {
			processed, err := w.runOnceGuarded(ctx)
			if err != nil {
				log.Printf("tenant projection worker failed: %v", err)
			}
			if processed > 0 {
				continue
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

// runOnceGuarded keeps a panic in one projection cycle from killing the
// worker goroutine (and with it the whole process).
func (w *Worker) runOnceGuarded(ctx context.Context) (processed int, err error) {
	defer panicguard.RecoverAs("tenant projection worker", &err)
	return w.RunOnce(ctx)
}

func (w *Worker) RunOnce(ctx context.Context) (int, error) {
	if w == nil || w.repo == nil || w.pool == nil || w.migrator == nil {
		return 0, nil
	}
	targets, err := w.repo.ListProvisionedTargets(ctx, w.config.TargetLimit)
	if err != nil {
		w.recordFailure(err)
		return 0, err
	}
	w.statsMu.Lock()
	w.stats.TargetsDiscovered += uint64(len(targets))
	w.statsMu.Unlock()

	processed := 0
	errs := make([]error, 0)
	for _, target := range targets {
		count, err := w.processTarget(ctx, target)
		processed += count
		if err != nil {
			errs = append(errs, fmt.Errorf("tenant %s: %w", target.OrganizationID, err))
		}
	}
	if len(errs) > 0 {
		joined := errors.Join(errs...)
		w.recordFailure(joined)
		return processed, joined
	}
	return processed, nil
}

func (w *Worker) processTarget(ctx context.Context, target tenantdb.Target) (int, error) {
	if err := w.ensureMigrated(ctx, &target); err != nil {
		return 0, err
	}
	db, release, err := w.pool.AcquireTarget(ctx, target)
	if err != nil {
		return 0, err
	}
	defer release()

	events, err := ClaimEvents(ctx, db, w.config.WorkerID, w.config.LeaseDuration, w.config.BatchSize, w.config.MaxAttempts)
	if err != nil {
		return 0, err
	}
	if len(events) == 0 {
		return 0, nil
	}
	eventIDs := make([]uuid.UUID, len(events))
	for index, event := range events {
		eventIDs[index] = event.EventID
		if event.OrganizationID != target.OrganizationID {
			err := fmt.Errorf("outbox event %s organization %s does not match target %s", event.EventID, event.OrganizationID, target.OrganizationID)
			w.failEvents(ctx, db, eventIDs, err)
			return len(events), err
		}
	}
	w.statsMu.Lock()
	w.stats.EventsClaimed += uint64(len(events))
	w.statsMu.Unlock()

	snapshot, err := LoadOperationalSnapshot(ctx, db, target.OrganizationID)
	if err != nil {
		w.failEvents(ctx, db, eventIDs, err)
		return len(events), err
	}
	activities, err := LoadRecentActivity(ctx, db, target.OrganizationID, w.config.ActivityLimit)
	if err != nil {
		w.failEvents(ctx, db, eventIDs, err)
		return len(events), err
	}
	workflowEntities, err := LoadWorkflowEntities(ctx, db, target.OrganizationID)
	if err != nil {
		w.failEvents(ctx, db, eventIDs, err)
		return len(events), err
	}
	projectedAt := w.now().UTC()
	freshness, err := w.repo.ApplyProjection(ctx, events, snapshot, activities, workflowEntities, projectedAt)
	if err != nil {
		w.failEvents(ctx, db, eventIDs, err)
		return len(events), err
	}
	if err := MarkEventsPublished(ctx, db, w.config.WorkerID, eventIDs, projectedAt); err != nil {
		return len(events), err
	}
	w.statsMu.Lock()
	w.stats.EventsProjected += uint64(len(events))
	w.stats.ProjectionRuns++
	w.stats.LastProjectionAt = projectedAt
	w.stats.LastProjectionLagMs = freshness.LagMilliseconds
	if freshness.LagMilliseconds > w.stats.MaxObservedLagMs {
		w.stats.MaxObservedLagMs = freshness.LagMilliseconds
	}
	w.stats.LastError = ""
	w.statsMu.Unlock()
	return len(events), nil
}

func (w *Worker) ensureMigrated(ctx context.Context, target *tenantdb.Target) error {
	if _, ok := w.migrated.Load(target.DatabaseName); ok {
		return nil
	}
	url, err := tenantdb.DatabaseURLForName(w.config.AdminURL, target.DatabaseName)
	if err != nil {
		return err
	}
	result, err := w.migrator.Migrate(ctx, *target, url)
	if err != nil {
		return fmt.Errorf("migrate tenant projection infrastructure: %w", err)
	}
	if strings.TrimSpace(result.Version) != "" {
		target.MigrationVersion = result.Version
	}
	target.Metadata = mergeMetadata(target.Metadata, map[string]any{
		"last_projection_migration_at": w.now().UTC(),
		"projection_migration_version": target.MigrationVersion,
	})
	if w.recorder != nil {
		if err := w.recorder.RecordTenantDatabaseMigrationResult(ctx, *target, result); err != nil {
			return fmt.Errorf("record tenant projection migration: %w", err)
		}
		if err := w.recorder.UpdateTenantDatabaseTarget(ctx, *target); err != nil {
			return fmt.Errorf("update tenant projection migration target: %w", err)
		}
	}
	w.migrated.Store(target.DatabaseName, true)
	w.statsMu.Lock()
	w.stats.TargetsMigrated++
	w.statsMu.Unlock()
	return nil
}

func (w *Worker) failEvents(ctx context.Context, db tenantdb.DB, eventIDs []uuid.UUID, cause error) {
	retryAt := w.now().Add(w.config.RetryDelay)
	markErr := MarkEventsFailed(ctx, db, w.config.WorkerID, eventIDs, retryAt, cause.Error())
	w.statsMu.Lock()
	w.stats.EventsFailed += uint64(len(eventIDs))
	w.stats.ProjectionFailures++
	w.stats.LastError = cause.Error()
	w.statsMu.Unlock()
	if markErr != nil {
		log.Printf("tenant projection event failure update failed: %v", markErr)
	}
}

func (w *Worker) recordFailure(err error) {
	w.statsMu.Lock()
	w.stats.ProjectionFailures++
	w.stats.LastError = err.Error()
	w.statsMu.Unlock()
}

func (w *Worker) setRunning(running bool) {
	w.statsMu.Lock()
	w.stats.Running = running
	w.statsMu.Unlock()
}

func (w *Worker) Stats() WorkerStats {
	if w == nil {
		return WorkerStats{}
	}
	w.statsMu.Lock()
	defer w.statsMu.Unlock()
	return w.stats
}

func mergeMetadata(base map[string]any, extra map[string]any) map[string]any {
	merged := map[string]any{}
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range extra {
		merged[key] = value
	}
	return merged
}
