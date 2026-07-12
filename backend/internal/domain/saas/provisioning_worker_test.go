package saas

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/tenantdb"
)

func TestTenantDatabaseProvisioningWorkerCompletesProvisionAndBootstrap(t *testing.T) {
	orgID := uuid.New()
	target := tenantdb.NewDedicatedDatabaseTarget(orgID, "meta_org_", "local-primary", "local")
	jobRepo := &fakeProvisioningJobRepository{job: &TenantDatabaseProvisioningJob{
		ID: uuid.New(), Target: target, AttemptCount: 1, MaxAttempts: 8,
		BootstrapInput: tenantdb.TenantBootstrapInput{OrganizationID: orgID, OrganizationName: "Acme"},
	}}
	provisioned := target
	provisioned.Status = tenantdb.TargetStatusProvisioned
	provisioner := &workerProvisioner{result: tenantdb.ProvisionResult{
		Target:    provisioned,
		Migration: tenantdb.MigrationResult{Version: "001_tenant_business_baseline"},
	}}
	bootstrapper := &workerBootstrapper{}
	worker := NewTenantDatabaseProvisioningWorker(jobRepo, provisioner, bootstrapper, TenantDatabaseProvisioningWorkerConfig{WorkerID: "worker-a"})

	processed, err := worker.RunOnce(context.Background())

	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if !processed {
		t.Fatal("RunOnce() processed = false, want true")
	}
	if bootstrapper.input.OrganizationID != orgID || bootstrapper.input.OrganizationName != "Acme" {
		t.Fatalf("bootstrap input = %#v", bootstrapper.input)
	}
	if jobRepo.completedTarget.Status != tenantdb.TargetStatusProvisioned {
		t.Fatalf("completed target status = %q", jobRepo.completedTarget.Status)
	}
	if jobRepo.completedMigration.Version != "001_tenant_business_baseline" {
		t.Fatalf("completed migration version = %q", jobRepo.completedMigration.Version)
	}
	if jobRepo.failed {
		t.Fatal("job was recorded as failed")
	}
}

func TestTenantDatabaseProvisioningWorkerSchedulesRetryWithBackoff(t *testing.T) {
	orgID := uuid.New()
	target := tenantdb.NewDedicatedDatabaseTarget(orgID, "meta_org_", "local-primary", "local")
	jobRepo := &fakeProvisioningJobRepository{job: &TenantDatabaseProvisioningJob{
		ID: uuid.New(), Target: target, AttemptCount: 2, MaxAttempts: 8,
	}}
	provisioner := &workerProvisioner{err: errors.New("createdb denied")}
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	worker := NewTenantDatabaseProvisioningWorker(jobRepo, provisioner, nil, TenantDatabaseProvisioningWorkerConfig{
		WorkerID: "worker-a", RetryBaseDelay: 5 * time.Second,
	})
	worker.now = func() time.Time { return now }

	processed, err := worker.RunOnce(context.Background())

	if !processed || err == nil || err.Error() != "createdb denied" {
		t.Fatalf("RunOnce() = processed %v, err %v", processed, err)
	}
	if !jobRepo.failed || jobRepo.terminal {
		t.Fatalf("failed/terminal = %v/%v, want true/false", jobRepo.failed, jobRepo.terminal)
	}
	if jobRepo.failedTarget.Status != tenantdb.TargetStatusProvisioning {
		t.Fatalf("failed target status = %q, want provisioning", jobRepo.failedTarget.Status)
	}
	if want := now.Add(10 * time.Second); !jobRepo.nextAttempt.Equal(want) {
		t.Fatalf("next attempt = %s, want %s", jobRepo.nextAttempt, want)
	}
}

func TestTenantDatabaseProvisioningWorkerMarksFinalBootstrapFailure(t *testing.T) {
	orgID := uuid.New()
	target := tenantdb.NewDedicatedDatabaseTarget(orgID, "meta_org_", "local-primary", "local")
	jobRepo := &fakeProvisioningJobRepository{job: &TenantDatabaseProvisioningJob{
		ID: uuid.New(), Target: target, AttemptCount: 3, MaxAttempts: 3,
	}}
	provisioned := target
	provisioned.Status = tenantdb.TargetStatusProvisioned
	provisioner := &workerProvisioner{result: tenantdb.ProvisionResult{Target: provisioned}}
	bootstrapper := &workerBootstrapper{err: errors.New("seed rejected")}
	worker := NewTenantDatabaseProvisioningWorker(jobRepo, provisioner, bootstrapper, TenantDatabaseProvisioningWorkerConfig{WorkerID: "worker-a"})

	processed, err := worker.RunOnce(context.Background())

	if !processed || err == nil {
		t.Fatalf("RunOnce() = processed %v, err %v", processed, err)
	}
	if !jobRepo.terminal {
		t.Fatal("terminal = false, want true")
	}
	if jobRepo.failedTarget.Status != tenantdb.TargetStatusFailed {
		t.Fatalf("failed target status = %q, want failed", jobRepo.failedTarget.Status)
	}
	if jobRepo.message != "bootstrap tenant database: seed rejected" {
		t.Fatalf("failure message = %q", jobRepo.message)
	}
}

type fakeProvisioningJobRepository struct {
	job                *TenantDatabaseProvisioningJob
	completedTarget    tenantdb.Target
	completedMigration tenantdb.MigrationResult
	failed             bool
	failedTarget       tenantdb.Target
	message            string
	nextAttempt        time.Time
	terminal           bool
}

func (f *fakeProvisioningJobRepository) ClaimTenantDatabaseProvisioningJob(_ context.Context, workerID string, _ time.Duration) (*TenantDatabaseProvisioningJob, error) {
	if f.job == nil {
		return nil, nil
	}
	job := *f.job
	job.LeaseOwner = workerID
	f.job = nil
	return &job, nil
}

func (f *fakeProvisioningJobRepository) CompleteTenantDatabaseProvisioningJob(_ context.Context, _ TenantDatabaseProvisioningJob, target tenantdb.Target, migration tenantdb.MigrationResult) error {
	f.completedTarget = target
	f.completedMigration = migration
	return nil
}

func (f *fakeProvisioningJobRepository) FailTenantDatabaseProvisioningJob(_ context.Context, _ TenantDatabaseProvisioningJob, target tenantdb.Target, _ tenantdb.MigrationResult, message string, nextAttempt time.Time, terminal bool) error {
	f.failed = true
	f.failedTarget = target
	f.message = message
	f.nextAttempt = nextAttempt
	f.terminal = terminal
	return nil
}

type workerProvisioner struct {
	result tenantdb.ProvisionResult
	err    error
}

func (p *workerProvisioner) Provision(context.Context, tenantdb.Target) (tenantdb.ProvisionResult, error) {
	return p.result, p.err
}

type workerBootstrapper struct {
	input tenantdb.TenantBootstrapInput
	err   error
}

func (b *workerBootstrapper) BootstrapTenant(_ context.Context, _ tenantdb.Target, input tenantdb.TenantBootstrapInput) error {
	b.input = input
	return b.err
}
