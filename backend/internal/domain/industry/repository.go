package industry

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetPlatformRole(ctx context.Context, actorID uuid.UUID) (string, error) {
	var role string
	err := r.db.QueryRow(ctx, `SELECT role FROM platform_admins WHERE user_id = $1`, actorID).Scan(&role)
	if err != nil {
		return "", fmt.Errorf("get platform role: %w", err)
	}
	return role, nil
}

func (r *Repository) GetOrganizationAuthority(ctx context.Context, actorID uuid.UUID, orgID uuid.UUID) (string, error) {
	var authority string
	err := r.db.QueryRow(ctx, `
		SELECT authority_tier
		FROM organization_memberships
		WHERE organization_id = $1 AND user_id = $2 AND member_type = 'internal' AND status = 'active'
		ORDER BY joined_at ASC
		LIMIT 1
	`, orgID, actorID).Scan(&authority)
	if err != nil {
		return "", fmt.Errorf("get organization authority: %w", err)
	}
	return authority, nil
}

func (r *Repository) ListIndustries(ctx context.Context, limit int) ([]Industry, error) {
	rows, err := r.db.Query(ctx, `
		SELECT industry_key, name, description, status, metadata, created_by, created_at, updated_at
		FROM platform.industries
		ORDER BY status = 'active' DESC, name
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list industries: %w", err)
	}
	defer rows.Close()
	items := []Industry{}
	for rows.Next() {
		var item Industry
		if err := scanIndustry(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) GetIndustry(ctx context.Context, key string) (*Industry, error) {
	item := &Industry{}
	err := scanIndustry(r.db.QueryRow(ctx, `
		SELECT industry_key, name, description, status, metadata, created_by, created_at, updated_at
		FROM platform.industries
		WHERE industry_key = $1
	`, key), item)
	if err != nil {
		return nil, fmt.Errorf("get industry: %w", err)
	}
	return item, nil
}

func (r *Repository) CreateIndustry(ctx context.Context, input CreateIndustryInput, actorID uuid.UUID) (*Industry, error) {
	item := &Industry{}
	err := scanIndustry(r.db.QueryRow(ctx, `
		INSERT INTO platform.industries(industry_key, name, description, status, metadata, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (industry_key) DO UPDATE SET
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			status = EXCLUDED.status,
			metadata = EXCLUDED.metadata,
			updated_at = NOW()
		RETURNING industry_key, name, description, status, metadata, created_by, created_at, updated_at
	`, input.IndustryKey, input.Name, input.Description, input.Status, jsonBytes(input.Metadata), actorID), item)
	if err != nil {
		return nil, fmt.Errorf("create industry: %w", err)
	}
	return item, nil
}

func (r *Repository) ListPackages(ctx context.Context, industryKey string, limit int) ([]Package, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, industry_key, package_key, version, name, description, status, metadata, created_by, created_at, updated_at
		FROM platform.custom_packages
		WHERE $1 = '' OR industry_key = $1
		ORDER BY industry_key, version DESC, updated_at DESC
		LIMIT $2
	`, industryKey, limit)
	if err != nil {
		return nil, fmt.Errorf("list industry packages: %w", err)
	}
	defer rows.Close()
	items := []Package{}
	for rows.Next() {
		var item Package
		if err := scanPackage(rows, &item); err != nil {
			return nil, err
		}
		assets, err := r.listPackageAssets(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		item.Assets = assets
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) GetPackage(ctx context.Context, packageID uuid.UUID) (*Package, error) {
	return r.GetPackageByID(ctx, packageID)
}

func (r *Repository) GetPackageByID(ctx context.Context, packageID uuid.UUID) (*Package, error) {
	item := &Package{}
	err := scanPackage(r.db.QueryRow(ctx, `
		SELECT id, industry_key, package_key, version, name, description, status, metadata, created_by, created_at, updated_at
		FROM platform.custom_packages
		WHERE id = $1
	`, packageID), item)
	if err != nil {
		return nil, fmt.Errorf("get industry package: %w", err)
	}
	assets, err := r.listPackageAssets(ctx, packageID)
	if err != nil {
		return nil, err
	}
	item.Assets = assets
	return item, nil
}

func (r *Repository) CreatePackage(ctx context.Context, input CreatePackageInput, actorID uuid.UUID) (*Package, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin create industry package: %w", err)
	}
	defer tx.Rollback(ctx)

	item := &Package{}
	err = scanPackage(tx.QueryRow(ctx, `
		INSERT INTO platform.custom_packages(industry_key, package_key, version, name, description, status, metadata, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, industry_key, package_key, version, name, description, status, metadata, created_by, created_at, updated_at
	`, input.IndustryKey, input.PackageKey, input.Version, input.Name, input.Description, input.Status, jsonBytes(input.Metadata), actorID), item)
	if err != nil {
		return nil, fmt.Errorf("create industry package: %w", err)
	}
	for _, asset := range input.Assets {
		if _, err := tx.Exec(ctx, `
			INSERT INTO platform.custom_package_assets(package_id, asset_key, asset_type, payload, metadata)
			VALUES ($1, $2, $3, $4, $5)
		`, item.ID, asset.AssetKey, asset.AssetType, jsonBytes(asset.Payload), jsonBytes(asset.Metadata)); err != nil {
			return nil, fmt.Errorf("create package asset %s: %w", asset.AssetKey, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit create industry package: %w", err)
	}
	item.Assets = input.Assets
	return item, nil
}

func (r *Repository) ActivatePackage(ctx context.Context, packageID uuid.UUID, actorID uuid.UUID) (*Package, error) {
	item := &Package{}
	err := scanPackage(r.db.QueryRow(ctx, `
		UPDATE platform.custom_packages
		SET status = 'active',
		    metadata = metadata || jsonb_build_object('activated_by', $2::text),
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id, industry_key, package_key, version, name, description, status, metadata, created_by, created_at, updated_at
	`, packageID, actorID), item)
	if err != nil {
		return nil, fmt.Errorf("activate industry package: %w", err)
	}
	assets, err := r.listPackageAssets(ctx, packageID)
	if err != nil {
		return nil, err
	}
	item.Assets = assets
	return item, nil
}

func (r *Repository) UpsertAdoption(ctx context.Context, input ApplyPackageInput, pkg Package) (*OrganizationAdoption, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin upsert organization industry adoption: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, asset := range pkg.Assets {
		if asset.AssetType != AssetTypeModule {
			continue
		}
		moduleKey := stringFromPayload(asset.Payload, "module_key")
		if moduleKey == "" {
			continue
		}
		displayName := stringFromPayload(asset.Payload, "display_name")
		if displayName == "" {
			displayName = moduleKey
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO saas_modules(module_key, display_name, category, enabled_default, license_scope, metadata)
			VALUES ($1, $2, COALESCE(NULLIF($3, ''), 'business'), FALSE, 'commercial', $4)
			ON CONFLICT (module_key) DO UPDATE SET
				display_name = COALESCE(NULLIF(EXCLUDED.display_name, ''), saas_modules.display_name),
				metadata = saas_modules.metadata || EXCLUDED.metadata,
				updated_at = NOW()
		`, moduleKey, displayName, stringFromPayload(asset.Payload, "category"), jsonBytes(map[string]any{"industry_key": pkg.IndustryKey, "package_id": pkg.ID.String()})); err != nil {
			return nil, fmt.Errorf("upsert industry module %s: %w", moduleKey, err)
		}
	}

	if _, err := tx.Exec(ctx, `
		UPDATE organization_module_entitlements
		SET status = 'disabled', source = 'manual', updated_at = NOW()
		WHERE organization_id = $1
	`, input.OrganizationID); err != nil {
		return nil, fmt.Errorf("disable organization modules for industry adoption: %w", err)
	}
	for _, moduleKey := range input.ModuleKeys {
		if _, err := tx.Exec(ctx, `
			INSERT INTO organization_module_entitlements(organization_id, module_key, status, source, metadata)
			VALUES ($1, $2, 'enabled', 'manual', $3)
			ON CONFLICT (organization_id, module_key) DO UPDATE SET
				status = 'enabled',
				source = 'manual',
				metadata = organization_module_entitlements.metadata || EXCLUDED.metadata,
				updated_at = NOW()
		`, input.OrganizationID, moduleKey, jsonBytes(map[string]any{"industry_key": pkg.IndustryKey, "package_id": input.PackageID.String()})); err != nil {
			return nil, fmt.Errorf("enable organization industry module %s: %w", moduleKey, err)
		}
	}

	item := &OrganizationAdoption{}
	err = scanAdoption(tx.QueryRow(ctx, `
		INSERT INTO platform.organization_industry_adoptions(
			organization_id, industry_key, package_id, is_primary, enabled_modules, status, metadata
		)
		VALUES ($1, $2, $3, TRUE, $4, 'active', $5)
		ON CONFLICT (organization_id) DO UPDATE SET
			industry_key = EXCLUDED.industry_key,
			package_id = EXCLUDED.package_id,
			is_primary = TRUE,
			enabled_modules = EXCLUDED.enabled_modules,
			status = 'active',
			metadata = platform.organization_industry_adoptions.metadata || EXCLUDED.metadata,
			updated_at = NOW()
		RETURNING organization_id, industry_key, package_id, is_primary, enabled_modules, status, metadata, created_at, updated_at
	`, input.OrganizationID, pkg.IndustryKey, input.PackageID, jsonBytes(input.ModuleKeys), jsonBytes(input.Metadata)), item)
	if err != nil {
		return nil, fmt.Errorf("upsert organization industry adoption: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit upsert organization industry adoption: %w", err)
	}
	return item, nil
}

func (r *Repository) ListOrganizationExtensionModules(ctx context.Context, orgID uuid.UUID, industryKey string) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		SELECT asset->'payload'->>'module_key'
		FROM platform.organization_industry_extensions e
		CROSS JOIN LATERAL jsonb_array_elements(e.assets) AS asset
		WHERE e.organization_id = $1
		  AND e.industry_key = $2
		  AND e.status = 'active'
		  AND asset->>'asset_type' = 'module'
	`, orgID, industryKey)
	if err != nil {
		return nil, fmt.Errorf("list organization extension modules: %w", err)
	}
	defer rows.Close()
	modules := []string{}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		if key != "" {
			modules = append(modules, key)
		}
	}
	return modules, rows.Err()
}

func (r *Repository) GetAdoption(ctx context.Context, orgID uuid.UUID) (*OrganizationAdoption, error) {
	item := &OrganizationAdoption{}
	err := scanAdoption(r.db.QueryRow(ctx, `
		SELECT organization_id, industry_key, package_id, is_primary, enabled_modules, status, metadata, created_at, updated_at
		FROM platform.organization_industry_adoptions
		WHERE organization_id = $1
	`, orgID), item)
	if err != nil {
		return nil, fmt.Errorf("get organization industry adoption: %w", err)
	}
	return item, nil
}

func (r *Repository) CreateExtension(ctx context.Context, input CreateExtensionInput, actorID uuid.UUID) (*Extension, error) {
	item := &Extension{}
	err := scanExtension(r.db.QueryRow(ctx, `
		INSERT INTO platform.organization_industry_extensions(
			organization_id, industry_key, package_id, extension_key, name, description, status, assets, metadata, created_by
		)
		VALUES ($1, $2, $3, $4, $5, $6, 'active', $7, $8, $9)
		RETURNING id, organization_id, industry_key, package_id, extension_key, name, description, status, assets, metadata, created_by, created_at, updated_at
	`, input.OrganizationID, input.IndustryKey, nullableUUID(input.PackageID), input.ExtensionKey, input.Name, input.Description, jsonBytes(input.Assets), jsonBytes(input.Metadata), actorID), item)
	if err != nil {
		return nil, fmt.Errorf("create organization industry extension: %w", err)
	}
	return item, nil
}

func (r *Repository) ListExtensions(ctx context.Context, orgID uuid.UUID, limit int) ([]Extension, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, organization_id, industry_key, package_id, extension_key, name, description, status, assets, metadata, created_by, created_at, updated_at
		FROM platform.organization_industry_extensions
		WHERE organization_id = $1
		ORDER BY updated_at DESC
		LIMIT $2
	`, orgID, limit)
	if err != nil {
		return nil, fmt.Errorf("list organization industry extensions: %w", err)
	}
	defer rows.Close()
	items := []Extension{}
	for rows.Next() {
		var item Extension
		if err := scanExtension(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) GetExtension(ctx context.Context, extensionID uuid.UUID) (*Extension, error) {
	item := &Extension{}
	err := scanExtension(r.db.QueryRow(ctx, `
		SELECT id, organization_id, industry_key, package_id, extension_key, name, description, status, assets, metadata, created_by, created_at, updated_at
		FROM platform.organization_industry_extensions
		WHERE id = $1
	`, extensionID), item)
	if err != nil {
		return nil, fmt.Errorf("get organization industry extension: %w", err)
	}
	return item, nil
}

func (r *Repository) CreatePublicationRequest(ctx context.Context, extension Extension, actorID uuid.UUID, reason string, metadata map[string]any) (*PublicationRequest, error) {
	item := &PublicationRequest{}
	err := scanPublicationRequest(r.db.QueryRow(ctx, `
		INSERT INTO platform.custom_package_publication_requests(
			extension_id, source_organization_id, industry_key, status, reason, requested_by, metadata
		)
		VALUES ($1, $2, $3, 'pending', $4, $5, $6)
		RETURNING id, extension_id, source_organization_id, industry_key, status, reason, review_reason,
			requested_by, reviewed_by, metadata, created_at, updated_at, reviewed_at
	`, extension.ID, extension.OrganizationID, extension.IndustryKey, reason, actorID, jsonBytes(metadata)), item)
	if err != nil {
		return nil, fmt.Errorf("create publication request: %w", err)
	}
	return item, nil
}

func (r *Repository) GetPublicationRequest(ctx context.Context, requestID uuid.UUID) (*PublicationRequest, error) {
	item := &PublicationRequest{}
	err := scanPublicationRequest(r.db.QueryRow(ctx, `
		SELECT id, extension_id, source_organization_id, industry_key, status, reason, review_reason,
			requested_by, reviewed_by, metadata, created_at, updated_at, reviewed_at
		FROM platform.custom_package_publication_requests
		WHERE id = $1
	`, requestID), item)
	if err != nil {
		return nil, fmt.Errorf("get publication request: %w", err)
	}
	return item, nil
}

func (r *Repository) UpdatePublicationRequestMetadata(ctx context.Context, requestID uuid.UUID, metadata map[string]any) error {
	_, err := r.db.Exec(ctx, `
		UPDATE platform.custom_package_publication_requests
		SET metadata = $2::jsonb, updated_at = NOW()
		WHERE id = $1
	`, requestID, jsonBytes(metadata))
	if err != nil {
		return fmt.Errorf("update publication request metadata: %w", err)
	}
	return nil
}

func (r *Repository) ListPublicationRequests(ctx context.Context, limit int) ([]PublicationRequest, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, extension_id, source_organization_id, industry_key, status, reason, review_reason,
			requested_by, reviewed_by, metadata, created_at, updated_at, reviewed_at
		FROM platform.custom_package_publication_requests
		ORDER BY created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list publication requests: %w", err)
	}
	defer rows.Close()
	items := []PublicationRequest{}
	for rows.Next() {
		var item PublicationRequest
		if err := scanPublicationRequest(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) ReviewPublicationRequest(ctx context.Context, requestID uuid.UUID, actorID uuid.UUID, status string, reason string) (*PublicationRequest, error) {
	item := &PublicationRequest{}
	err := scanPublicationRequest(r.db.QueryRow(ctx, `
		UPDATE platform.custom_package_publication_requests
		SET status = $2,
		    review_reason = $3,
		    reviewed_by = $4,
		    reviewed_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id, extension_id, source_organization_id, industry_key, status, reason, review_reason,
			requested_by, reviewed_by, metadata, created_at, updated_at, reviewed_at
	`, requestID, status, reason, actorID), item)
	if err != nil {
		return nil, fmt.Errorf("review publication request: %w", err)
	}
	return item, nil
}

func (r *Repository) ListKnowledgeSources(ctx context.Context, industryKey string, orgID uuid.UUID, limit int) ([]KnowledgeSource, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, industry_key, organization_id, source_key, name, source_type, adapter_key, reference_uri,
			sync_status, permission, retrieval_config, metadata, created_at, updated_at
		FROM platform.knowledge_sources
		WHERE ($1 = '' OR industry_key = $1)
		  AND ($2::uuid IS NULL OR organization_id IS NULL OR organization_id = $2)
		ORDER BY organization_id NULLS FIRST, name
		LIMIT $3
	`, industryKey, nullableUUID(orgID), limit)
	if err != nil {
		return nil, fmt.Errorf("list knowledge sources: %w", err)
	}
	defer rows.Close()
	items := []KnowledgeSource{}
	for rows.Next() {
		var item KnowledgeSource
		if err := scanKnowledgeSource(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) listPackageAssets(ctx context.Context, packageID uuid.UUID) ([]PackageAsset, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, package_id, asset_key, asset_type, payload, metadata, created_at, updated_at
		FROM platform.custom_package_assets
		WHERE package_id = $1
		ORDER BY asset_type, asset_key
	`, packageID)
	if err != nil {
		return nil, fmt.Errorf("list package assets: %w", err)
	}
	defer rows.Close()
	items := []PackageAsset{}
	for rows.Next() {
		var item PackageAsset
		if err := scanPackageAsset(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanIndustry(row pgx.Row, item *Industry) error {
	var metadata []byte
	var createdBy pgtype.UUID
	if err := row.Scan(&item.IndustryKey, &item.Name, &item.Description, &item.Status, &metadata, &createdBy, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return err
	}
	item.Metadata = mapFromJSON(metadata)
	item.CreatedBy = uuidPtr(createdBy)
	return nil
}

func scanPackage(row pgx.Row, item *Package) error {
	var metadata []byte
	var createdBy pgtype.UUID
	if err := row.Scan(&item.ID, &item.IndustryKey, &item.PackageKey, &item.Version, &item.Name, &item.Description, &item.Status, &metadata, &createdBy, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return err
	}
	item.Metadata = mapFromJSON(metadata)
	item.CreatedBy = uuidPtr(createdBy)
	return nil
}

func scanPackageAsset(row pgx.Row, item *PackageAsset) error {
	var payload, metadata []byte
	if err := row.Scan(&item.ID, &item.PackageID, &item.AssetKey, &item.AssetType, &payload, &metadata, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return err
	}
	item.Payload = mapFromJSON(payload)
	item.Metadata = mapFromJSON(metadata)
	return nil
}

func scanAdoption(row pgx.Row, item *OrganizationAdoption) error {
	var modules, metadata []byte
	if err := row.Scan(&item.OrganizationID, &item.IndustryKey, &item.PackageID, &item.Primary, &modules, &item.Status, &metadata, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return err
	}
	_ = json.Unmarshal(modules, &item.EnabledModules)
	item.Metadata = mapFromJSON(metadata)
	return nil
}

func scanExtension(row pgx.Row, item *Extension) error {
	var assets, metadata []byte
	var packageID pgtype.UUID
	var createdBy pgtype.UUID
	if err := row.Scan(&item.ID, &item.OrganizationID, &item.IndustryKey, &packageID, &item.ExtensionKey, &item.Name, &item.Description, &item.Status, &assets, &metadata, &createdBy, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return err
	}
	if packageID.Valid {
		item.PackageID = uuid.UUID(packageID.Bytes)
	}
	_ = json.Unmarshal(assets, &item.Assets)
	item.Metadata = mapFromJSON(metadata)
	item.CreatedBy = uuidPtr(createdBy)
	return nil
}

func scanPublicationRequest(row pgx.Row, item *PublicationRequest) error {
	var metadata []byte
	var requestedBy, reviewedBy pgtype.UUID
	var reviewedAt pgtype.Timestamptz
	if err := row.Scan(&item.ID, &item.ExtensionID, &item.SourceOrganizationID, &item.IndustryKey, &item.Status, &item.Reason, &item.ReviewReason, &requestedBy, &reviewedBy, &metadata, &item.CreatedAt, &item.UpdatedAt, &reviewedAt); err != nil {
		return err
	}
	item.RequestedBy = uuidPtr(requestedBy)
	item.ReviewedBy = uuidPtr(reviewedBy)
	if reviewedAt.Valid {
		t := reviewedAt.Time
		item.ReviewedAt = &t
	}
	item.Metadata = mapFromJSON(metadata)
	return nil
}

func scanKnowledgeSource(row pgx.Row, item *KnowledgeSource) error {
	var organizationID pgtype.UUID
	var permission, retrieval, metadata []byte
	if err := row.Scan(&item.ID, &item.IndustryKey, &organizationID, &item.SourceKey, &item.Name, &item.SourceType, &item.AdapterKey, &item.ReferenceURI, &item.SyncStatus, &permission, &retrieval, &metadata, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return err
	}
	item.OrganizationID = uuidPtr(organizationID)
	item.Permission = mapFromJSON(permission)
	item.RetrievalConfig = mapFromJSON(retrieval)
	item.Metadata = mapFromJSON(metadata)
	return nil
}

func jsonBytes(value any) []byte {
	if value == nil {
		return []byte("{}")
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Map:
		if reflected.IsNil() {
			return []byte("{}")
		}
	case reflect.Slice:
		if reflected.IsNil() {
			return []byte("[]")
		}
	}
	data, err := json.Marshal(value)
	if err != nil {
		return []byte("{}")
	}
	return data
}

func mapFromJSON(data []byte) map[string]any {
	out := map[string]any{}
	if len(data) == 0 {
		return out
	}
	_ = json.Unmarshal(data, &out)
	return out
}

func uuidPtr(value pgtype.UUID) *uuid.UUID {
	if !value.Valid {
		return nil
	}
	id := uuid.UUID(value.Bytes)
	return &id
}

func nullableUUID(value uuid.UUID) any {
	if value == uuid.Nil {
		return nil
	}
	return value
}
