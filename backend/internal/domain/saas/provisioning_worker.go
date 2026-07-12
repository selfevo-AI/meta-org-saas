package saas

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/tenantdb"
)

type TenantDatabaseProvisioningJob struct {
	ID             uuid.UUID
	LeaseOwner     string
	Target         tenantdb.Target
	BootstrapInput tenantdb.TenantBootstrapInput
	AttemptCount   int
	MaxAttempts    int
}

type TenantDatabaseProvisioningWorkerConfig struct {
	WorkerID       string
	PollInterval   time.Duration
	LeaseDuration  time.Duration
	RetryBaseDelay time.Duration
	RetryMaxDelay  time.Duration
}

type tenantDatabaseProvisioningJobRepository interface {
	ClaimTenantDatabaseProvisioningJob(context.Context, string, time.Duration) (*TenantDatabaseProvisioningJob, error)
	CompleteTenantDatabaseProvisioningJob(context.Context, TenantDatabaseProvisioningJob, tenantdb.Target, tenantdb.MigrationResult) error
	FailTenantDatabaseProvisioningJob(context.Context, TenantDatabaseProvisioningJob, tenantdb.Target, tenantdb.MigrationResult, string, time.Time, bool) error
}

type tenantDatabaseProvisioner interface {
	Provision(context.Context, tenantdb.Target) (tenantdb.ProvisionResult, error)
}

type tenantDatabaseBootstrapper interface {
	BootstrapTenant(context.Context, tenantdb.Target, tenantdb.TenantBootstrapInput) error
}

type TenantDatabaseProvisioningWorker struct {
	repo         tenantDatabaseProvisioningJobRepository
	provisioner  tenantDatabaseProvisioner
	bootstrapper tenantDatabaseBootstrapper
	config       TenantDatabaseProvisioningWorkerConfig
	now          func() time.Time
}

func NewTenantDatabaseProvisioningWorker(
	repo tenantDatabaseProvisioningJobRepository,
	provisioner tenantDatabaseProvisioner,
	bootstrapper tenantDatabaseBootstrapper,
	config TenantDatabaseProvisioningWorkerConfig,
) *TenantDatabaseProvisioningWorker {
	if strings.TrimSpace(config.WorkerID) == "" {
		config.WorkerID = "tenant-provisioner-" + uuid.NewString()
	}
	if config.PollInterval <= 0 {
		config.PollInterval = 2 * time.Second
	}
	if config.LeaseDuration <= 0 {
		config.LeaseDuration = 30 * time.Minute
	}
	if config.RetryBaseDelay <= 0 {
		config.RetryBaseDelay = 5 * time.Second
	}
	if config.RetryMaxDelay <= 0 {
		config.RetryMaxDelay = 15 * time.Minute
	}
	return &TenantDatabaseProvisioningWorker{
		repo: repo, provisioner: provisioner, bootstrapper: bootstrapper, config: config, now: time.Now,
	}
}

func (w *TenantDatabaseProvisioningWorker) Start(ctx context.Context) {
	if w == nil || w.repo == nil || w.provisioner == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(w.config.PollInterval)
		defer ticker.Stop()
		for {
			processed, err := w.RunOnce(ctx)
			if err != nil {
				log.Printf("tenant database provisioning job failed: %v", err)
			}
			if processed {
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

func (w *TenantDatabaseProvisioningWorker) RunOnce(ctx context.Context) (bool, error) {
	if w == nil || w.repo == nil || w.provisioner == nil {
		return false, nil
	}
	job, err := w.repo.ClaimTenantDatabaseProvisioningJob(ctx, w.config.WorkerID, w.config.LeaseDuration)
	if err != nil || job == nil {
		return false, err
	}

	jobCtx, cancel := context.WithTimeout(ctx, w.config.LeaseDuration)
	result, runErr := w.provisioner.Provision(jobCtx, job.Target)
	updated := result.Target
	if updated.OrganizationID == uuid.Nil {
		updated = job.Target
	}
	if runErr == nil && w.bootstrapper != nil {
		job.BootstrapInput.OrganizationID = job.Target.OrganizationID
		if err := w.bootstrapper.BootstrapTenant(jobCtx, updated, job.BootstrapInput); err != nil {
			runErr = fmt.Errorf("bootstrap tenant database: %w", err)
		}
	}
	cancel()

	if runErr == nil {
		updated.Status = tenantdb.TargetStatusProvisioned
		if err := w.repo.CompleteTenantDatabaseProvisioningJob(ctx, *job, updated, result.Migration); err != nil {
			return true, err
		}
		return true, nil
	}

	terminal := job.AttemptCount >= job.MaxAttempts
	if terminal {
		updated.Status = tenantdb.TargetStatusFailed
	} else {
		updated.Status = tenantdb.TargetStatusProvisioning
	}
	updated.Metadata = mergeTenantDatabaseMetadata(updated.Metadata, map[string]any{
		"last_provisioning_error":   runErr.Error(),
		"last_provisioning_attempt": job.AttemptCount,
	})
	nextAttempt := w.now().Add(w.retryDelay(job.AttemptCount))
	if err := w.repo.FailTenantDatabaseProvisioningJob(ctx, *job, updated, result.Migration, runErr.Error(), nextAttempt, terminal); err != nil {
		return true, errors.Join(runErr, err)
	}
	return true, runErr
}

func (w *TenantDatabaseProvisioningWorker) retryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	multiplier := math.Pow(2, float64(attempt-1))
	delay := time.Duration(float64(w.config.RetryBaseDelay) * multiplier)
	if delay > w.config.RetryMaxDelay || delay < 0 {
		return w.config.RetryMaxDelay
	}
	return delay
}

func mergeTenantDatabaseMetadata(base map[string]any, extra map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range base {
		out[key] = value
	}
	for key, value := range extra {
		out[key] = value
	}
	return out
}

func (r *Repository) ClaimTenantDatabaseProvisioningJob(ctx context.Context, workerID string, lease time.Duration) (*TenantDatabaseProvisioningJob, error) {
	if lease <= 0 {
		lease = 30 * time.Minute
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tenant provisioning job claim: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		WITH exhausted AS (
		    UPDATE platform.tenant_database_provisioning_jobs
		    SET status = 'failed',
		        last_error = CASE WHEN last_error = '' THEN 'worker lease expired after final attempt' ELSE last_error END,
		        lease_owner = '', lease_expires_at = NULL, completed_at = NOW(), updated_at = NOW()
		    WHERE status = 'running' AND lease_expires_at <= NOW() AND attempt_count >= max_attempts
		    RETURNING tenant_database_id, last_error
		)
		UPDATE platform.tenant_database_targets t
		SET status = CASE WHEN t.status = 'archived' THEN 'archived' ELSE 'failed' END,
		    metadata = t.metadata || jsonb_build_object('last_provisioning_error', e.last_error),
		    updated_at = NOW()
		FROM exhausted e
		WHERE t.id = e.tenant_database_id
	`); err != nil {
		return nil, fmt.Errorf("expire exhausted tenant provisioning jobs: %w", err)
	}

	job := &TenantDatabaseProvisioningJob{LeaseOwner: workerID}
	var bootstrapJSON []byte
	var metadataJSON []byte
	err = tx.QueryRow(ctx, `
		WITH candidate AS (
		    SELECT id
		    FROM platform.tenant_database_provisioning_jobs
		    WHERE attempt_count < max_attempts
		      AND (
		        (status IN ('pending', 'retry_scheduled') AND available_at <= NOW())
		        OR (status = 'running' AND lease_expires_at <= NOW())
		      )
		    ORDER BY available_at, created_at
		    FOR UPDATE SKIP LOCKED
		    LIMIT 1
		)
		UPDATE platform.tenant_database_provisioning_jobs j
		SET status = 'running', attempt_count = j.attempt_count + 1,
		    lease_owner = $1, lease_expires_at = NOW() + make_interval(secs => $2),
		    started_at = COALESCE(j.started_at, NOW()), last_error = '', updated_at = NOW()
		FROM candidate c, platform.tenant_database_targets t
		WHERE j.id = c.id AND t.organization_id = j.organization_id
		RETURNING j.id, j.attempt_count, j.max_attempts, j.bootstrap_payload,
		          t.organization_id, t.deployment_mode, t.cluster_key, t.region,
		          t.database_name, t.schema_name, t.connection_secret_ref,
		          t.migration_version, t.status, t.metadata
	`, workerID, int64(lease/time.Second)).Scan(
		&job.ID, &job.AttemptCount, &job.MaxAttempts, &bootstrapJSON,
		&job.Target.OrganizationID, &job.Target.DeploymentMode, &job.Target.ClusterKey, &job.Target.Region,
		&job.Target.DatabaseName, &job.Target.SchemaName, &job.Target.ConnectionSecretRef,
		&job.Target.MigrationVersion, &job.Target.Status, &metadataJSON,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit empty tenant provisioning job claim: %w", err)
		}
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("claim tenant provisioning job: %w", err)
	}
	if err := json.Unmarshal(bootstrapJSON, &job.BootstrapInput); err != nil {
		return nil, fmt.Errorf("decode tenant provisioning bootstrap payload: %w", err)
	}
	job.Target.Metadata = unmarshalMap(metadataJSON)
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tenant provisioning job claim: %w", err)
	}
	return job, nil
}

func (r *Repository) CompleteTenantDatabaseProvisioningJob(ctx context.Context, job TenantDatabaseProvisioningJob, target tenantdb.Target, migration tenantdb.MigrationResult) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tenant provisioning job completion: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := r.upsertTenantDatabaseTarget(ctx, tx, target); err != nil {
		return err
	}
	if err := r.recordTenantDatabaseMigrationResult(ctx, tx, target, migration); err != nil {
		return err
	}
	command, err := tx.Exec(ctx, `
		UPDATE platform.tenant_database_provisioning_jobs
		SET status = 'succeeded', lease_owner = '', lease_expires_at = NULL,
		    completed_at = NOW(), last_error = '', updated_at = NOW()
		WHERE id = $1 AND status = 'running' AND lease_owner = $2
	`, job.ID, job.LeaseOwner)
	if err != nil {
		return fmt.Errorf("complete tenant provisioning job: %w", err)
	}
	if command.RowsAffected() != 1 {
		return fmt.Errorf("tenant provisioning job %s lease is no longer owned by %s", job.ID, job.LeaseOwner)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tenant provisioning job completion: %w", err)
	}
	return nil
}

func (r *Repository) FailTenantDatabaseProvisioningJob(ctx context.Context, job TenantDatabaseProvisioningJob, target tenantdb.Target, migration tenantdb.MigrationResult, message string, nextAttempt time.Time, terminal bool) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tenant provisioning job failure: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := r.upsertTenantDatabaseTarget(ctx, tx, target); err != nil {
		return err
	}
	if err := r.recordTenantDatabaseMigrationResult(ctx, tx, target, migration); err != nil {
		return err
	}
	status := "retry_scheduled"
	if terminal {
		status = "failed"
	}
	command, err := tx.Exec(ctx, `
		UPDATE platform.tenant_database_provisioning_jobs
		SET status = $3, available_at = $4, lease_owner = '', lease_expires_at = NULL,
		    last_error = $5, completed_at = CASE WHEN $3 = 'failed' THEN NOW() ELSE NULL END,
		    updated_at = NOW()
		WHERE id = $1 AND status = 'running' AND lease_owner = $2
	`, job.ID, job.LeaseOwner, status, nextAttempt, message)
	if err != nil {
		return fmt.Errorf("record tenant provisioning job failure: %w", err)
	}
	if command.RowsAffected() != 1 {
		return fmt.Errorf("tenant provisioning job %s lease is no longer owned by %s", job.ID, job.LeaseOwner)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tenant provisioning job failure: %w", err)
	}
	return nil
}
