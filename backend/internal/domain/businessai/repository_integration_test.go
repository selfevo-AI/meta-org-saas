package businessai

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRepositoryPersistsAuthoritativeContextHash(t *testing.T) {
	if os.Getenv("RUN_BUSINESS_AI_DB_TEST") != "1" {
		t.Skip("set RUN_BUSINESS_AI_DB_TEST=1 to run Business AI database verification")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	url := os.Getenv("BUSINESS_AI_TEST_DATABASE_URL")
	if url == "" {
		url = "postgres://postgres:postgres@localhost:5432/meta_org_saas?sslmode=disable"
	}
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	orgID, projectID := uuid.New(), uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO organizations(id, name) VALUES ($1, $2)`, orgID, "business-ai-context-"+orgID.String()); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM business_stage_ai_runs WHERE organization_id = $1`, orgID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM organizations WHERE id = $1`, orgID)
	}()

	repo := NewRepository(pool)
	run, err := repo.CreateRun(ctx, AnalyzeInput{
		OrganizationID: orgID, ProjectID: projectID, Stage: StagePlan,
		ProviderType: "openai", Model: "test-model", Context: map[string]any{"project_overview": map[string]any{"status": "active"}},
		ContextHash: "sha256-context-v1",
	})
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if run.ContextHash != "sha256-context-v1" {
		t.Fatalf("created context hash = %q", run.ContextHash)
	}
	loaded, err := repo.GetRun(ctx, orgID, projectID, run.ID)
	if err != nil {
		t.Fatalf("GetRun() error = %v", err)
	}
	if loaded.ContextHash != "sha256-context-v1" {
		t.Fatalf("loaded context hash = %q", loaded.ContextHash)
	}
}
