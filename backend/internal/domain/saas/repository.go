package saas

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/tenantdb"
)

type Repository struct {
	db                     *pgxpool.Pool
	tenantDatabaseDefaults tenantdb.Defaults
}

type RepositoryOption func(*Repository)

type membershipRecord struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	AuthorityTier  string
}

func NewRepository(db *pgxpool.Pool, opts ...RepositoryOption) *Repository {
	r := &Repository{
		db: db,
		tenantDatabaseDefaults: tenantdb.Defaults{
			DeploymentMode: tenantdb.DeploymentModeSharedSchema,
			ClusterKey:     "local-primary",
			Region:         "local",
		},
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

func WithTenantDatabaseDefaults(defaults tenantdb.Defaults) RepositoryOption {
	return func(r *Repository) {
		r.tenantDatabaseDefaults = defaults
	}
}

func (r *Repository) BootstrapPlatformAdmin(ctx context.Context, email string, passwordHash string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin platform admin bootstrap: %w", err)
	}
	defer tx.Rollback(ctx)

	var userID uuid.UUID
	err = tx.QueryRow(ctx, `SELECT id FROM users WHERE lower(email) = lower($1)`, email).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		name := strings.Split(email, "@")[0]
		err = tx.QueryRow(ctx, `
			INSERT INTO users (name, email, password_hash, onboarding_status)
			VALUES ($1, lower($2), $3, 'complete')
			RETURNING id
		`, name, email, passwordHash).Scan(&userID)
	}
	if err != nil {
		return fmt.Errorf("upsert platform admin user: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE users
		SET account_status = 'active',
		    password_hash = $2,
		    onboarding_status = CASE WHEN default_organization_id IS NULL THEN 'complete' ELSE onboarding_status END,
		    updated_at = NOW()
		WHERE id = $1
	`, userID, passwordHash); err != nil {
		return fmt.Errorf("activate platform admin user: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO platform_admins (user_id, role)
		VALUES ($1, 'system_owner')
		ON CONFLICT (user_id) DO UPDATE SET role = 'system_owner', updated_at = NOW()
	`, userID); err != nil {
		return fmt.Errorf("upsert platform admin role: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit platform admin bootstrap: %w", err)
	}
	return nil
}

func (r *Repository) GetUserProfile(ctx context.Context, userID uuid.UUID) (*UserProfile, error) {
	profile := &UserProfile{}
	var defaultOrg pgtype.UUID
	err := r.db.QueryRow(ctx, `
		SELECT id, name, email, account_status, onboarding_status, default_organization_id
		FROM users
		WHERE id = $1
	`, userID).Scan(&profile.ID, &profile.Name, &profile.Email, &profile.AccountStatus, &profile.OnboardingStatus, &defaultOrg)
	if err != nil {
		return nil, fmt.Errorf("get user profile: %w", err)
	}
	profile.DefaultOrganizationID = uuidPointer(defaultOrg)
	profile.OnboardingRequired = profile.OnboardingStatus != OnboardingComplete && profile.PlatformRole == ""

	role, err := r.GetPlatformRole(ctx, userID)
	if err == nil {
		profile.PlatformRole = role
	}
	profile.OnboardingRequired = profile.OnboardingStatus != OnboardingComplete && profile.PlatformRole == ""

	orgs, err := r.ListOrganizationsForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	profile.Organizations = orgs
	if profile.DefaultOrganizationID != nil {
		profile.EnabledModules, _ = r.ListEnabledModules(ctx, *profile.DefaultOrganizationID)
	}
	return profile, nil
}

func (r *Repository) GetUserIDByEmail(ctx context.Context, email string) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.db.QueryRow(ctx, `SELECT id FROM users WHERE lower(email) = lower($1)`, email).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("get user by email: %w", err)
	}
	return id, nil
}

func (r *Repository) CreateUserWithPasswordHash(ctx context.Context, name string, email string, passwordHash string) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.db.QueryRow(ctx, `
		INSERT INTO users (name, email, password_hash)
		VALUES ($1, lower($2), $3)
		RETURNING id
	`, name, email, passwordHash).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create invited user: %w", err)
	}
	return id, nil
}

func (r *Repository) ListOrganizationsForUser(ctx context.Context, userID uuid.UUID) ([]OrganizationAccount, error) {
	rows, err := r.db.Query(ctx, `
		SELECT o.id, o.name, COALESCE(o.description, ''), COALESCE(o.status, 'active'), om.id, om.authority_tier
		FROM organization_memberships om
		JOIN organizations o ON o.id = om.organization_id
		WHERE om.user_id = $1 AND om.status = 'active' AND COALESCE(o.status, 'active') = 'active'
		ORDER BY om.joined_at ASC, o.name
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list user organizations: %w", err)
	}
	defer rows.Close()

	items := []OrganizationAccount{}
	for rows.Next() {
		var item OrganizationAccount
		var membershipID uuid.UUID
		if err := rows.Scan(&item.ID, &item.Name, &item.Description, &item.Status, &membershipID, &item.AuthorityTier); err != nil {
			return nil, fmt.Errorf("scan user organization: %w", err)
		}
		item.MembershipID = &membershipID
		item.IsOwner = item.AuthorityTier == AuthorityOwner
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list user organizations iteration: %w", err)
	}
	return items, nil
}

func (r *Repository) ListOrganizationsForPlatform(ctx context.Context, limit int) ([]OrganizationAccount, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, name, COALESCE(description, ''), COALESCE(status, 'active'), closed_at, closed_by, COALESCE(closed_reason, '')
		FROM organizations
		ORDER BY created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list platform organizations: %w", err)
	}
	defer rows.Close()
	items := []OrganizationAccount{}
	for rows.Next() {
		var item OrganizationAccount
		if err := scanOrganizationAccount(rows, &item); err != nil {
			return nil, fmt.Errorf("scan platform organization: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list platform organizations iteration: %w", err)
	}
	return items, nil
}

func (r *Repository) GetOrganizationAccount(ctx context.Context, orgID uuid.UUID) (*OrganizationAccount, error) {
	item := &OrganizationAccount{}
	err := scanOrganizationAccount(r.db.QueryRow(ctx, `
		SELECT id, name, COALESCE(description, ''), COALESCE(status, 'active'), closed_at, closed_by, COALESCE(closed_reason, '')
		FROM organizations
		WHERE id = $1
	`, orgID), item)
	if err != nil {
		return nil, fmt.Errorf("get organization account: %w", err)
	}
	return item, nil
}

func (r *Repository) CloseOrganization(ctx context.Context, orgID uuid.UUID, actorID uuid.UUID, reason string) (*OrganizationAccount, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin close organization: %w", err)
	}
	defer tx.Rollback(ctx)

	item := &OrganizationAccount{}
	err = scanOrganizationAccount(tx.QueryRow(ctx, `
		UPDATE organizations
		SET status = 'closed',
		    closed_at = COALESCE(closed_at, NOW()),
		    closed_by = $2,
		    closed_reason = $3,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id, name, COALESCE(description, ''), COALESCE(status, 'active'), closed_at, closed_by, COALESCE(closed_reason, '')
	`, orgID, actorID, reason), item)
	if err != nil {
		return nil, fmt.Errorf("close organization: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE platform.organization_schema_targets
		SET status = 'archived',
		    metadata = metadata || jsonb_build_object('closed_by', $2::text, 'closed_reason', $3),
		    updated_at = NOW()
		WHERE organization_id = $1
	`, orgID, actorID, reason); err != nil {
		return nil, fmt.Errorf("archive organization schema target: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE platform.tenant_database_targets
		SET status = 'archived',
		    metadata = metadata || jsonb_build_object('closed_by', $2::text, 'closed_reason', $3),
		    updated_at = NOW()
		WHERE organization_id = $1
	`, orgID, actorID, reason); err != nil {
		return nil, fmt.Errorf("archive tenant database target: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit close organization: %w", err)
	}
	return item, nil
}

func scanOrganizationAccount(row interface{ Scan(dest ...any) error }, item *OrganizationAccount) error {
	var closedAt pgtype.Timestamptz
	var closedBy pgtype.UUID
	if err := row.Scan(
		&item.ID,
		&item.Name,
		&item.Description,
		&item.Status,
		&closedAt,
		&closedBy,
		&item.ClosedReason,
	); err != nil {
		return err
	}
	if closedAt.Valid {
		value := closedAt.Time
		item.ClosedAt = &value
	}
	if closedBy.Valid {
		value, err := uuid.FromBytes(closedBy.Bytes[:])
		if err == nil {
			item.ClosedBy = &value
		}
	}
	return nil
}

func (r *Repository) GetPlatformRole(ctx context.Context, userID uuid.UUID) (string, error) {
	var role string
	err := r.db.QueryRow(ctx, `SELECT role FROM platform_admins WHERE user_id = $1`, userID).Scan(&role)
	if err != nil {
		return "", fmt.Errorf("get platform role: %w", err)
	}
	return role, nil
}

func (r *Repository) ListModules(ctx context.Context) ([]Module, error) {
	rows, err := r.db.Query(ctx, `
		SELECT module_key, display_name, category, enabled_default, license_scope, metadata
		FROM saas_modules
		ORDER BY category, display_name
	`)
	if err != nil {
		return nil, fmt.Errorf("list modules: %w", err)
	}
	defer rows.Close()

	items := []Module{}
	for rows.Next() {
		var item Module
		var metadataJSON []byte
		if err := rows.Scan(&item.ModuleKey, &item.DisplayName, &item.Category, &item.EnabledDefault, &item.LicenseScope, &metadataJSON); err != nil {
			return nil, fmt.Errorf("scan module: %w", err)
		}
		item.Metadata = unmarshalMap(metadataJSON)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list modules iteration: %w", err)
	}
	return items, nil
}

func (r *Repository) ListDefaultModuleKeys(ctx context.Context) ([]string, error) {
	rows, err := r.db.Query(ctx, `SELECT module_key FROM saas_modules WHERE enabled_default ORDER BY module_key`)
	if err != nil {
		return nil, fmt.Errorf("list default modules: %w", err)
	}
	defer rows.Close()
	keys := []string{}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("scan default module: %w", err)
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list default modules iteration: %w", err)
	}
	return keys, nil
}

func (r *Repository) CompleteOnboarding(ctx context.Context, userID uuid.UUID, input OnboardingOrganizationInput, enabledModules []string) (*OrganizationAccount, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin onboarding: %w", err)
	}
	defer tx.Rollback(ctx)

	var status string
	if err := tx.QueryRow(ctx, `SELECT onboarding_status FROM users WHERE id = $1 FOR UPDATE`, userID).Scan(&status); err != nil {
		return nil, fmt.Errorf("lock onboarding user: %w", err)
	}
	if status == OnboardingComplete {
		return nil, fmt.Errorf("%w: onboarding is already complete", ErrConflict)
	}

	var org OrganizationAccount
	var createdBy pgtype.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO organizations (name, description, created_by)
		VALUES ($1, $2, $3)
		RETURNING id, name, COALESCE(description, ''), created_by
	`, input.OrganizationName, input.Description, userID).Scan(&org.ID, &org.Name, &org.Description, &createdBy)
	if err != nil {
		return nil, fmt.Errorf("create onboarding organization: %w", err)
	}

	var deptID uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO departments (organization_id, name, code, description, metadata)
		VALUES ($1, 'Default Department', 'DEFAULT', 'Created by SaaS onboarding', '{"system_created":true}'::jsonb)
		RETURNING id
	`, org.ID).Scan(&deptID); err != nil {
		return nil, fmt.Errorf("create onboarding department: %w", err)
	}

	var membershipID uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO organization_memberships (
			organization_id, department_id, member_type, user_id, title, authority_tier, status, metadata
		)
		VALUES ($1, $2, 'internal', $3, 'Owner', 'organization_creator', 'active', '{"source":"onboarding"}'::jsonb)
		RETURNING id
	`, org.ID, deptID, userID).Scan(&membershipID); err != nil {
		return nil, fmt.Errorf("create owner membership: %w", err)
	}

	var planID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT id FROM saas_plans WHERE code = 'foundation'`).Scan(&planID); err != nil {
		return nil, fmt.Errorf("get foundation plan: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO organization_subscriptions (
			organization_id, plan_id, status, trial_ends_at, current_period_start, current_period_end, metadata
		)
		VALUES ($1, $2, 'trialing', NOW() + INTERVAL '30 days', NOW(), NOW() + INTERVAL '30 days', '{"source":"onboarding"}'::jsonb)
		ON CONFLICT (organization_id) DO UPDATE SET
			plan_id = EXCLUDED.plan_id,
			status = EXCLUDED.status,
			trial_ends_at = EXCLUDED.trial_ends_at,
			current_period_start = EXCLUDED.current_period_start,
			current_period_end = EXCLUDED.current_period_end,
			updated_at = NOW()
	`, org.ID, planID); err != nil {
		return nil, fmt.Errorf("create organization subscription: %w", err)
	}

	for _, moduleKey := range enabledModules {
		if _, err := tx.Exec(ctx, `
			INSERT INTO organization_module_entitlements (organization_id, module_key, status, source)
			VALUES ($1, $2, 'enabled', 'trial')
			ON CONFLICT (organization_id, module_key) DO UPDATE SET status = 'enabled', source = 'trial', updated_at = NOW()
		`, org.ID, moduleKey); err != nil {
			return nil, fmt.Errorf("create organization entitlement: %w", err)
		}
	}

	if _, err := tx.Exec(ctx, `
		UPDATE users
		SET onboarding_status = 'complete',
		    default_organization_id = $2,
		    updated_at = NOW()
		WHERE id = $1
	`, userID, org.ID); err != nil {
		return nil, fmt.Errorf("complete onboarding user: %w", err)
	}

	if _, err := tx.Exec(ctx, `SELECT platform.provision_runtime_organization($1)`, org.ID); err != nil {
		return nil, fmt.Errorf("provision runtime organization schema: %w", err)
	}
	if err := r.upsertTenantDatabaseTarget(ctx, tx, r.tenantDatabaseDefaults.TargetForOrganization(org.ID)); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit onboarding: %w", err)
	}
	org.MembershipID = &membershipID
	org.AuthorityTier = AuthorityOwner
	org.IsOwner = true
	return &org, nil
}

func (r *Repository) EnsureSingleOrgForUser(ctx context.Context, userID uuid.UUID) (*OrganizationAccount, error) {
	orgs, err := r.ListOrganizationsForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(orgs) > 0 {
		_, _ = r.db.Exec(ctx, `
			UPDATE users
			SET onboarding_status = 'complete',
			    default_organization_id = COALESCE(default_organization_id, $2),
			    updated_at = NOW()
			WHERE id = $1
		`, userID, orgs[0].ID)
		return &orgs[0], nil
	}
	defaultModules, err := r.ListDefaultModuleKeys(ctx)
	if err != nil {
		return nil, err
	}
	return r.CompleteOnboarding(ctx, userID, OnboardingOrganizationInput{
		OrganizationName: "Default Organization",
		Description:      "Single organization workspace",
	}, defaultModules)
}

func (r *Repository) GetHumanMembership(ctx context.Context, userID uuid.UUID, orgID uuid.UUID) (*membershipRecord, error) {
	item := &membershipRecord{}
	err := r.db.QueryRow(ctx, `
		SELECT id, organization_id, authority_tier
		FROM organization_memberships
		WHERE organization_id = $1
		  AND user_id = $2
		  AND member_type = 'internal'
		  AND status = 'active'
		ORDER BY joined_at ASC
		LIMIT 1
	`, orgID, userID).Scan(&item.ID, &item.OrganizationID, &item.AuthorityTier)
	if err != nil {
		return nil, fmt.Errorf("get human membership: %w", err)
	}
	return item, nil
}

func (r *Repository) GetAgentMembership(ctx context.Context, agentID uuid.UUID, orgID uuid.UUID) (*membershipRecord, error) {
	item := &membershipRecord{}
	err := r.db.QueryRow(ctx, `
		SELECT id, organization_id, authority_tier
		FROM organization_memberships
		WHERE organization_id = $1
		  AND agent_id = $2
		  AND member_type = 'agent'
		  AND status = 'active'
		ORDER BY joined_at ASC
		LIMIT 1
	`, orgID, agentID).Scan(&item.ID, &item.OrganizationID, &item.AuthorityTier)
	if err != nil {
		return nil, fmt.Errorf("get agent membership: %w", err)
	}
	return item, nil
}

func (r *Repository) ListEnabledModules(ctx context.Context, orgID uuid.UUID) (map[string]bool, error) {
	rows, err := r.db.Query(ctx, `
		SELECT module_key
		FROM organization_module_entitlements
		WHERE organization_id = $1 AND status = 'enabled'
	`, orgID)
	if err != nil {
		return nil, fmt.Errorf("list enabled modules: %w", err)
	}
	defer rows.Close()
	enabled := map[string]bool{}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("scan enabled module: %w", err)
		}
		enabled[key] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list enabled modules iteration: %w", err)
	}
	return enabled, nil
}

func (r *Repository) GetTenantDatabaseTarget(ctx context.Context, orgID uuid.UUID) (*tenantdb.Target, error) {
	return scanTenantDatabaseTarget(r.db.QueryRow(ctx, `
		SELECT organization_id, deployment_mode, cluster_key, region, database_name, schema_name,
		       connection_secret_ref, migration_version, status, metadata
		FROM platform.tenant_database_targets
		WHERE organization_id = $1
	`, orgID))
}

func (r *Repository) UpdateTenantDatabaseTarget(ctx context.Context, target tenantdb.Target) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tenant database target update: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := r.upsertTenantDatabaseTarget(ctx, tx, target); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tenant database target update: %w", err)
	}
	return nil
}

func (r *Repository) UpdateOrganizationModules(ctx context.Context, orgID uuid.UUID, enabledModules []string) (map[string]bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin update organization modules: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		UPDATE organization_module_entitlements
		SET status = 'disabled', source = 'manual', updated_at = NOW()
		WHERE organization_id = $1
	`, orgID); err != nil {
		return nil, fmt.Errorf("disable organization modules: %w", err)
	}

	for _, key := range enabledModules {
		if _, err := tx.Exec(ctx, `
			INSERT INTO organization_module_entitlements (organization_id, module_key, status, source)
			VALUES ($1, $2, 'enabled', 'manual')
			ON CONFLICT (organization_id, module_key)
			DO UPDATE SET status = 'enabled', source = 'manual', updated_at = NOW()
		`, orgID, key); err != nil {
			return nil, fmt.Errorf("enable organization module %s: %w", key, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit update organization modules: %w", err)
	}
	return r.ListEnabledModules(ctx, orgID)
}

func (r *Repository) GetSubscription(ctx context.Context, orgID uuid.UUID) (*OrganizationSubscription, error) {
	item := &OrganizationSubscription{}
	var metadataJSON []byte
	err := r.db.QueryRow(ctx, `
		SELECT s.id, s.organization_id, s.plan_id, COALESCE(p.code, ''), COALESCE(p.name, ''),
		       s.status, s.trial_ends_at, s.current_period_start, s.current_period_end,
		       s.metadata, s.created_at, s.updated_at
		FROM organization_subscriptions s
		LEFT JOIN saas_plans p ON p.id = s.plan_id
		WHERE s.organization_id = $1
	`, orgID).Scan(
		&item.ID, &item.OrganizationID, &item.PlanID, &item.PlanCode, &item.PlanName,
		&item.Status, &item.TrialEndsAt, &item.CurrentPeriodStart, &item.CurrentPeriodEnd,
		&metadataJSON, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get subscription: %w", err)
	}
	item.Metadata = unmarshalMap(metadataJSON)
	return item, nil
}

func (r *Repository) CreateInvitation(ctx context.Context, orgID uuid.UUID, invitedBy uuid.UUID, input CreateInvitationInput, token string) (*Invitation, error) {
	tokenHash := hashToken(token)
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
	metadataJSON := mustJSON(input.Metadata)
	item := &Invitation{Token: token}
	err := scanInvitation(r.db.QueryRow(ctx, `
		INSERT INTO organization_invitations (
			organization_id, email, name, role_id, authority_tier, token_hash, invited_by, expires_at, metadata
		)
		VALUES ($1, lower($2), $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, organization_id, email, name, role_id, authority_tier, status,
		          invited_by, accepted_by, expires_at, metadata, accepted_at, created_at, updated_at
	`, orgID, input.Email, input.Name, input.RoleID, input.AuthorityTier, tokenHash, invitedBy, time.Now().AddDate(0, 0, input.ExpiresInDays), metadataJSON), item)
	if err != nil {
		return nil, fmt.Errorf("create invitation: %w", err)
	}
	item.Token = token
	return item, nil
}

func (r *Repository) ListInvitations(ctx context.Context, orgID uuid.UUID, limit int) ([]Invitation, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, organization_id, email, name, role_id, authority_tier, status,
		       invited_by, accepted_by, expires_at, metadata, accepted_at, created_at, updated_at
		FROM organization_invitations
		WHERE organization_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, orgID, limit)
	if err != nil {
		return nil, fmt.Errorf("list invitations: %w", err)
	}
	defer rows.Close()
	items := []Invitation{}
	for rows.Next() {
		var item Invitation
		if err := scanInvitation(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list invitations iteration: %w", err)
	}
	return items, nil
}

func (r *Repository) AcceptInvitationWithNewUser(ctx context.Context, token string, input AcceptInvitationInput, passwordHash string) (*OrganizationAccount, uuid.UUID, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, uuid.Nil, fmt.Errorf("begin accept invitation: %w", err)
	}
	defer tx.Rollback(ctx)

	inv, err := r.lockInvitationByToken(ctx, tx, token)
	if err != nil {
		return nil, uuid.Nil, err
	}
	if err := validateInvitationForAcceptance(inv); err != nil {
		return nil, uuid.Nil, err
	}
	if !strings.EqualFold(inv.Email, input.Email) {
		return nil, uuid.Nil, fmt.Errorf("%w: invitation email does not match", ErrValidation)
	}
	var existingUserID uuid.UUID
	err = tx.QueryRow(ctx, `SELECT id FROM users WHERE lower(email) = lower($1)`, input.Email).Scan(&existingUserID)
	if err == nil {
		return nil, uuid.Nil, fmt.Errorf("%w: email already exists; login and ask the owner to add this account", ErrConflict)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, uuid.Nil, fmt.Errorf("check invited user: %w", err)
	}
	var userID uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO users (name, email, password_hash)
		VALUES ($1, lower($2), $3)
		RETURNING id
	`, input.Name, input.Email, passwordHash).Scan(&userID); err != nil {
		return nil, uuid.Nil, fmt.Errorf("create invited user: %w", err)
	}
	org, err := r.acceptInvitationForUser(ctx, tx, inv, userID)
	if err != nil {
		return nil, uuid.Nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, uuid.Nil, fmt.Errorf("commit accept invitation: %w", err)
	}
	return org, userID, nil
}

func (r *Repository) AcceptInvitation(ctx context.Context, token string, userID uuid.UUID) (*OrganizationAccount, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin accept invitation: %w", err)
	}
	defer tx.Rollback(ctx)

	inv, err := r.lockInvitationByToken(ctx, tx, token)
	if err != nil {
		return nil, err
	}
	if err := validateInvitationForAcceptance(inv); err != nil {
		return nil, err
	}
	org, err := r.acceptInvitationForUser(ctx, tx, inv, userID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit accept invitation: %w", err)
	}
	return org, nil
}

func (r *Repository) lockInvitationByToken(ctx context.Context, tx pgx.Tx, token string) (*Invitation, error) {
	var inv Invitation
	var metadataJSON []byte
	err := tx.QueryRow(ctx, `
		SELECT id, organization_id, email, name, role_id, authority_tier, status,
		       invited_by, accepted_by, expires_at, metadata, accepted_at, created_at, updated_at
		FROM organization_invitations
		WHERE token_hash = $1
		FOR UPDATE
	`, hashToken(token)).Scan(
		&inv.ID, &inv.OrganizationID, &inv.Email, &inv.Name, &inv.RoleID, &inv.AuthorityTier,
		&inv.Status, &inv.InvitedBy, &inv.AcceptedBy, &inv.ExpiresAt, &metadataJSON,
		&inv.AcceptedAt, &inv.CreatedAt, &inv.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get invitation: %w", err)
	}
	inv.Metadata = unmarshalMap(metadataJSON)
	return &inv, nil
}

func validateInvitationForAcceptance(inv *Invitation) error {
	if inv.Status != "pending" || time.Now().After(inv.ExpiresAt) {
		return fmt.Errorf("%w: invitation is not active", ErrValidation)
	}
	return nil
}

func (r *Repository) acceptInvitationForUser(ctx context.Context, tx pgx.Tx, inv *Invitation, userID uuid.UUID) (*OrganizationAccount, error) {
	var deptID uuid.UUID
	if err := tx.QueryRow(ctx, `
		SELECT id
		FROM departments
		WHERE organization_id = $1 AND status = 'active'
		ORDER BY parent_id NULLS FIRST, sort_order, created_at
		LIMIT 1
	`, inv.OrganizationID).Scan(&deptID); err != nil {
		return nil, fmt.Errorf("get invitation department: %w", err)
	}

	var membershipID uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO organization_memberships (
			organization_id, department_id, member_type, user_id, title, role_id, authority_tier, status, metadata
		)
		VALUES ($1, $2, 'internal', $3, '', $4, $5, 'active', '{"source":"invitation"}'::jsonb)
		ON CONFLICT (department_id, user_id) WHERE member_type = 'internal'
		DO UPDATE SET role_id = EXCLUDED.role_id,
		              authority_tier = EXCLUDED.authority_tier,
		              status = 'active',
		              updated_at = NOW()
		RETURNING id
	`, inv.OrganizationID, deptID, userID, inv.RoleID, inv.AuthorityTier).Scan(&membershipID); err != nil {
		return nil, fmt.Errorf("create invited membership: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE organization_invitations
		SET status = 'accepted', accepted_by = $2, accepted_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`, inv.ID, userID); err != nil {
		return nil, fmt.Errorf("mark invitation accepted: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE users
		SET onboarding_status = 'complete',
		    default_organization_id = COALESCE(default_organization_id, $2),
		    updated_at = NOW()
		WHERE id = $1
	`, userID, inv.OrganizationID); err != nil {
		return nil, fmt.Errorf("update invited user: %w", err)
	}

	org := &OrganizationAccount{}
	if err := tx.QueryRow(ctx, `
		SELECT id, name, COALESCE(description, '')
		FROM organizations
		WHERE id = $1
	`, inv.OrganizationID).Scan(&org.ID, &org.Name, &org.Description); err != nil {
		return nil, fmt.Errorf("get accepted invitation organization: %w", err)
	}
	org.MembershipID = &membershipID
	org.AuthorityTier = inv.AuthorityTier
	org.IsOwner = inv.AuthorityTier == AuthorityOwner
	return org, nil
}

func scanInvitation(scan interface{ Scan(dest ...any) error }, item *Invitation) error {
	var metadataJSON []byte
	if err := scan.Scan(
		&item.ID, &item.OrganizationID, &item.Email, &item.Name, &item.RoleID, &item.AuthorityTier, &item.Status,
		&item.InvitedBy, &item.AcceptedBy, &item.ExpiresAt, &metadataJSON, &item.AcceptedAt, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return fmt.Errorf("scan invitation: %w", err)
	}
	item.Metadata = unmarshalMap(metadataJSON)
	return nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func mustJSON(value any) []byte {
	if value == nil {
		return []byte("{}")
	}
	data, err := json.Marshal(value)
	if err != nil {
		return []byte("{}")
	}
	return data
}

func unmarshalMap(data []byte) map[string]any {
	if len(data) == 0 {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil || out == nil {
		return map[string]any{}
	}
	return out
}

func (r *Repository) upsertTenantDatabaseTarget(ctx context.Context, tx pgx.Tx, target tenantdb.Target) error {
	metadataJSON := mustJSON(target.Metadata)
	if _, err := tx.Exec(ctx, `
		INSERT INTO platform.tenant_database_targets(
		    organization_id, deployment_mode, cluster_key, region, database_name, schema_name,
		    connection_secret_ref, migration_version, status, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::jsonb)
		ON CONFLICT (organization_id) DO UPDATE SET
		    deployment_mode = EXCLUDED.deployment_mode,
		    cluster_key = EXCLUDED.cluster_key,
		    region = EXCLUDED.region,
		    database_name = EXCLUDED.database_name,
		    schema_name = EXCLUDED.schema_name,
		    connection_secret_ref = EXCLUDED.connection_secret_ref,
		    migration_version = EXCLUDED.migration_version,
		    status = CASE
		        WHEN platform.tenant_database_targets.status = 'archived' THEN 'archived'
		        ELSE EXCLUDED.status
		    END,
		    metadata = platform.tenant_database_targets.metadata || EXCLUDED.metadata,
		    updated_at = NOW()
	`, target.OrganizationID, target.DeploymentMode, target.ClusterKey, target.Region, target.DatabaseName, target.SchemaName,
		target.ConnectionSecretRef, target.MigrationVersion, target.Status, metadataJSON); err != nil {
		return fmt.Errorf("upsert tenant database target: %w", err)
	}
	return nil
}

func scanTenantDatabaseTarget(scan interface{ Scan(dest ...any) error }) (*tenantdb.Target, error) {
	var target tenantdb.Target
	var metadataJSON []byte
	if err := scan.Scan(
		&target.OrganizationID,
		&target.DeploymentMode,
		&target.ClusterKey,
		&target.Region,
		&target.DatabaseName,
		&target.SchemaName,
		&target.ConnectionSecretRef,
		&target.MigrationVersion,
		&target.Status,
		&metadataJSON,
	); err != nil {
		return nil, fmt.Errorf("get tenant database target: %w", err)
	}
	target.Metadata = unmarshalMap(metadataJSON)
	return &target, nil
}

func uuidPointer(value pgtype.UUID) *uuid.UUID {
	if !value.Valid {
		return nil
	}
	id := uuid.UUID(value.Bytes)
	return &id
}
