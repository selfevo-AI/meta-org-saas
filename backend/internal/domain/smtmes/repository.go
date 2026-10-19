package smtmes

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/tenantdb"
)

// Repository: platform database for the edge registry, tenant database for the
// integration inbox and sync log.
type Repository struct {
	platform *pgxpool.Pool
	tenant   tenantdb.DB
}

func NewRepository(platform *pgxpool.Pool, tenant tenantdb.DB) *Repository {
	return &Repository{platform: platform, tenant: tenant}
}

// ---------------------------------------------------------------------------
// Edge registry (platform database).
// ---------------------------------------------------------------------------

func (r *Repository) CreateEdgeInstance(ctx context.Context, record EdgeInstanceRecord) (*EdgeInstanceRecord, error) {
	row := r.platform.QueryRow(ctx, `
		INSERT INTO platform.smt_edge_instances(
			organization_id, name, base_url, api_key_cipher, webhook_secret_cipher,
			subscribed_events, status, auto_push
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING instance_key`,
		record.OrganizationID, record.Name, record.BaseURL, record.APIKeyCipher,
		record.WebhookSecretCipher, record.SubscribedEvents, record.Status, record.AutoPush)
	var key uuid.UUID
	if err := row.Scan(&key); err != nil {
		return nil, fmt.Errorf("create SMT edge instance: %w", err)
	}
	return r.GetEdgeInstance(ctx, record.OrganizationID, key)
}

func (r *Repository) ListEdgeInstances(ctx context.Context, orgID uuid.UUID) ([]EdgeInstanceRecord, error) {
	rows, err := r.platform.Query(ctx, `
		SELECT instance_key, organization_id, name, base_url, api_key_cipher, webhook_secret_cipher,
		       subscribed_events, status, auto_push, last_poll_id, last_sync_at, last_event_at, created_at, updated_at
		FROM platform.smt_edge_instances
		WHERE organization_id = $1
		ORDER BY created_at`, orgID)
	if err != nil {
		return nil, fmt.Errorf("list SMT edge instances: %w", err)
	}
	defer rows.Close()
	items := []EdgeInstanceRecord{}
	for rows.Next() {
		item, err := scanEdgeInstance(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (r *Repository) GetEdgeInstance(ctx context.Context, orgID uuid.UUID, key uuid.UUID) (*EdgeInstanceRecord, error) {
	row := r.platform.QueryRow(ctx, `
		SELECT instance_key, organization_id, name, base_url, api_key_cipher, webhook_secret_cipher,
		       subscribed_events, status, auto_push, last_poll_id, last_sync_at, last_event_at, created_at, updated_at
		FROM platform.smt_edge_instances
		WHERE organization_id = $1 AND instance_key = $2`, orgID, key)
	item, err := scanEdgeInstance(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return item, nil
}

// ResolveInstance resolves a webhook/pull target by instance key alone.
func (r *Repository) ResolveInstance(ctx context.Context, key uuid.UUID) (*ResolvedInstance, error) {
	row := r.platform.QueryRow(ctx, `
		SELECT i.instance_key, i.organization_id, i.name, i.base_url, i.api_key_cipher, i.webhook_secret_cipher,
		       i.subscribed_events, i.status, i.auto_push, i.last_poll_id, i.last_sync_at, i.last_event_at, i.created_at, i.updated_at,
		       COALESCE(t.deployment_mode, ''), COALESCE(t.database_name, ''), COALESCE(t.status, '')
		FROM platform.smt_edge_instances i
		LEFT JOIN platform.tenant_database_targets t ON t.organization_id = i.organization_id
		WHERE i.instance_key = $1`, key)
	var item ResolvedInstance
	err := row.Scan(
		&item.InstanceKey, &item.OrganizationID, &item.Name, &item.BaseURL,
		&item.APIKeyCipher, &item.WebhookSecretCipher, &item.SubscribedEvents, &item.Status,
		&item.AutoPush, &item.LastPollID, &item.LastSyncAt, &item.LastEventAt, &item.CreatedAt, &item.UpdatedAt,
		&item.TenantDatabaseDeploymentMode, &item.TenantDatabaseName, &item.TenantDatabaseStatus,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("resolve SMT edge instance: %w", err)
	}
	item.APIKeyMasked = maskSecret(item.APIKeyCipher)
	item.WebhookPath = "/api/v1/smt/hooks/" + item.InstanceKey.String()
	return &item, nil
}

func (r *Repository) UpdateEdgeInstance(ctx context.Context, orgID uuid.UUID, key uuid.UUID, input UpdateEdgeInstanceInput, apiKeyCipher, webhookSecretCipher *string) (*EdgeInstanceRecord, error) {
	_, err := r.platform.Exec(ctx, `
		UPDATE platform.smt_edge_instances SET
			name = COALESCE(NULLIF($3, ''), name),
			base_url = COALESCE(NULLIF($4, ''), base_url),
			api_key_cipher = COALESCE($5, api_key_cipher),
			webhook_secret_cipher = COALESCE($6, webhook_secret_cipher),
			subscribed_events = COALESCE(NULLIF($7, ''), subscribed_events),
			status = COALESCE(NULLIF($8, ''), status),
			auto_push = COALESCE($9, auto_push),
			updated_at = NOW()
		WHERE organization_id = $1 AND instance_key = $2`,
		orgID, key, input.Name, input.BaseURL, apiKeyCipher, webhookSecretCipher,
		input.SubscribedEvents, input.Status, input.AutoPush)
	if err != nil {
		return nil, fmt.Errorf("update SMT edge instance: %w", err)
	}
	return r.GetEdgeInstance(ctx, orgID, key)
}

func (r *Repository) DeleteEdgeInstance(ctx context.Context, orgID uuid.UUID, key uuid.UUID) error {
	tag, err := r.platform.Exec(ctx, `
		DELETE FROM platform.smt_edge_instances WHERE organization_id = $1 AND instance_key = $2`, orgID, key)
	if err != nil {
		return fmt.Errorf("delete SMT edge instance: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("SMT edge instance not found")
	}
	return nil
}

func (r *Repository) TouchEdgeInstanceSync(ctx context.Context, key uuid.UUID, pollID int64) error {
	_, err := r.platform.Exec(ctx, `
		UPDATE platform.smt_edge_instances
		SET last_sync_at = NOW(), last_poll_id = GREATEST(last_poll_id, $2), updated_at = NOW()
		WHERE instance_key = $1`, key, pollID)
	return err
}

func (r *Repository) TouchEdgeInstanceEvent(ctx context.Context, key uuid.UUID) error {
	_, err := r.platform.Exec(ctx, `
		UPDATE platform.smt_edge_instances
		SET last_event_at = NOW(), updated_at = NOW()
		WHERE instance_key = $1`, key)
	return err
}

// ListEnabledModules returns the org's enabled module keys.
func (r *Repository) ListEnabledModules(ctx context.Context, orgID uuid.UUID) (map[string]bool, error) {
	rows, err := r.platform.Query(ctx, `
		SELECT module_key FROM public.organization_module_entitlements
		WHERE organization_id = $1 AND status = 'enabled'`, orgID)
	if err != nil {
		return nil, fmt.Errorf("list enabled modules: %w", err)
	}
	defer rows.Close()
	modules := map[string]bool{}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		modules[key] = true
	}
	return modules, rows.Err()
}

// ---------------------------------------------------------------------------
// Integration inbox + sync log (tenant database, routed by request context).
// ---------------------------------------------------------------------------

// InsertEdgeEvent deduplicates an inbound event; inserted is false on replay.
// Failed rows are re-claimed so retried webhook deliveries can be processed.
func (r *Repository) InsertEdgeEvent(ctx context.Context, instanceKey uuid.UUID, eventID int64, eventType, ref string, payload []byte) (int64, bool, error) {
	row := r.tenant.QueryRow(ctx, `
		INSERT INTO smt_edge_events(instance_key, event_id, event_type, ref, payload)
		VALUES ($1, $2, $3, $4, $5::jsonb)
		ON CONFLICT (instance_key, event_id) DO UPDATE
			SET status = 'pending', error = ''
			WHERE smt_edge_events.status = 'failed'
		RETURNING id`, instanceKey, eventID, eventType, ref, string(payload))
	var id int64
	if err := row.Scan(&id); err != nil {
		if err == pgx.ErrNoRows {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("insert SMT edge event: %w", err)
	}
	return id, true, nil
}

func (r *Repository) MarkEdgeEvent(ctx context.Context, id int64, status, message string) error {
	_, err := r.tenant.Exec(ctx, `
		UPDATE smt_edge_events SET status = $2, error = $3, processed_at = NOW() WHERE id = $1`,
		id, status, message)
	return err
}

func (r *Repository) ListEdgeEvents(ctx context.Context, instanceKey uuid.UUID, status string, limit int) ([]EdgeEventRecord, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	sql := `
		SELECT id, instance_key, event_id, event_type, ref, status, error, received_at, processed_at
		FROM smt_edge_events
		WHERE ($1::uuid IS NULL OR instance_key = $1::uuid)
		  AND ($2 = '' OR status = $2)
		ORDER BY id DESC LIMIT $3`
	var key any
	if instanceKey != uuid.Nil {
		key = instanceKey
	}
	rows, err := r.tenant.Query(ctx, sql, key, status, limit)
	if err != nil {
		return nil, fmt.Errorf("list SMT edge events: %w", err)
	}
	defer rows.Close()
	items := []EdgeEventRecord{}
	for rows.Next() {
		var item EdgeEventRecord
		if err := rows.Scan(&item.ID, &item.InstanceKey, &item.EventID, &item.EventType, &item.Ref, &item.Status, &item.Error, &item.ReceivedAt, &item.ProcessedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) InsertSyncLog(ctx context.Context, instanceKey uuid.UUID, scope, status, message string, payload map[string]any) error {
	if payload == nil {
		payload = map[string]any{}
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = r.tenant.Exec(ctx, `
		INSERT INTO smt_edge_sync_log(instance_key, scope, status, message, payload)
		VALUES ($1, $2, $3, $4, $5::jsonb)`, instanceKey, scope, status, message, string(data))
	return err
}

func (r *Repository) ListSyncLog(ctx context.Context, instanceKey uuid.UUID, limit int) ([]SyncLogRecord, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var key any
	if instanceKey != uuid.Nil {
		key = instanceKey
	}
	rows, err := r.tenant.Query(ctx, `
		SELECT id, instance_key, scope, status, message, payload, created_at
		FROM smt_edge_sync_log
		WHERE ($1::uuid IS NULL OR instance_key = $1::uuid)
		ORDER BY id DESC LIMIT $2`, key, limit)
	if err != nil {
		return nil, fmt.Errorf("list SMT sync log: %w", err)
	}
	defer rows.Close()
	items := []SyncLogRecord{}
	for rows.Next() {
		var item SyncLogRecord
		var payload []byte
		if err := rows.Scan(&item.ID, &item.InstanceKey, &item.Scope, &item.Status, &item.Message, &payload, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.Payload = map[string]any{}
		_ = json.Unmarshal(payload, &item.Payload)
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanEdgeInstance(row interface{ Scan(...any) error }) (*EdgeInstanceRecord, error) {
	var item EdgeInstanceRecord
	if err := row.Scan(
		&item.InstanceKey, &item.OrganizationID, &item.Name, &item.BaseURL,
		&item.APIKeyCipher, &item.WebhookSecretCipher, &item.SubscribedEvents, &item.Status,
		&item.AutoPush, &item.LastPollID, &item.LastSyncAt, &item.LastEventAt, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	item.APIKeyMasked = maskSecret(item.APIKeyCipher)
	item.WebhookPath = "/api/v1/smt/hooks/" + item.InstanceKey.String()
	return &item, nil
}

func maskSecret(cipher string) string {
	if cipher == "" {
		return ""
	}
	digest := cipher
	if len(digest) > 6 {
		digest = digest[:6]
	}
	return "****" + digest
}
