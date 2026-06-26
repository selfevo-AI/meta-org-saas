package tenantdb

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TenantBootstrapInput struct {
	OrganizationID               uuid.UUID
	OrganizationName             string
	Description                  string
	OwnerUserID                  uuid.UUID
	OwnerName                    string
	OwnerEmail                   string
	EnabledModules               []string
	SampleKey                    string
	IncludeBusinessClosureSample bool
}

type TenantBootstrapper struct {
	AdminURL string
}

func (b TenantBootstrapper) BootstrapTenant(ctx context.Context, target Target, input TenantBootstrapInput) error {
	if target.DeploymentMode != DeploymentModeDedicatedDatabase {
		return nil
	}
	targetURL, err := DatabaseURLForName(b.AdminURL, target.DatabaseName)
	if err != nil {
		return err
	}
	pool, err := pgxpool.New(ctx, targetURL)
	if err != nil {
		return fmt.Errorf("connect tenant database for bootstrap: %w", err)
	}
	defer pool.Close()
	return BootstrapTenantData(ctx, pool, input)
}

func BootstrapTenantData(ctx context.Context, db DB, input TenantBootstrapInput) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tenant bootstrap: %w", err)
	}
	defer tx.Rollback(ctx)

	if input.OwnerUserID != uuid.Nil {
		if _, err := tx.Exec(ctx, `
			INSERT INTO users(id, name, email, password_hash, account_status, onboarding_status)
			VALUES ($1, $2, lower($3), '', 'active', 'complete')
			ON CONFLICT (id) DO UPDATE SET
			    name = EXCLUDED.name,
			    email = EXCLUDED.email,
			    account_status = 'active',
			    onboarding_status = 'complete',
			    updated_at = NOW()
		`, input.OwnerUserID, input.OwnerName, input.OwnerEmail); err != nil {
			return fmt.Errorf("bootstrap tenant owner projection: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO organizations(id, name, description, status, created_by, metadata)
		VALUES ($1, $2, $3, 'active', NULLIF($4::uuid, '00000000-0000-0000-0000-000000000000'::uuid), jsonb_build_object('sample_key', $5::text))
		ON CONFLICT (id) DO UPDATE SET
		    name = EXCLUDED.name,
		    description = EXCLUDED.description,
		    status = 'active',
		    metadata = organizations.metadata || EXCLUDED.metadata,
		    updated_at = NOW()
	`, input.OrganizationID, input.OrganizationName, input.Description, input.OwnerUserID, input.SampleKey); err != nil {
		return fmt.Errorf("bootstrap tenant organization projection: %w", err)
	}

	var departmentID uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO departments(organization_id, name, code, description, metadata)
		VALUES ($1, 'Default Department', 'DEFAULT', 'Tenant default department', jsonb_build_object('system_created', true, 'sample_key', $2::text))
		ON CONFLICT (organization_id, code) WHERE code IS NOT NULL AND code <> ''
		DO UPDATE SET name = EXCLUDED.name, description = EXCLUDED.description, metadata = departments.metadata || EXCLUDED.metadata, updated_at = NOW()
		RETURNING id
	`, input.OrganizationID, input.SampleKey).Scan(&departmentID); err != nil {
		return fmt.Errorf("bootstrap tenant default department: %w", err)
	}

	if input.OwnerUserID != uuid.Nil {
		if _, err := tx.Exec(ctx, `
			INSERT INTO organization_memberships(organization_id, department_id, member_type, user_id, title, authority_tier, status, metadata)
			VALUES ($1, $2, 'internal', $3, 'Owner', 'organization_creator', 'active', jsonb_build_object('sample_key', $4::text))
			ON CONFLICT (department_id, user_id) WHERE user_id IS NOT NULL AND status <> 'archived'
			DO NOTHING
		`, input.OrganizationID, departmentID, input.OwnerUserID, input.SampleKey); err != nil {
			return fmt.Errorf("bootstrap tenant owner membership: %w", err)
		}
	}

	for _, moduleKey := range input.EnabledModules {
		if moduleKey == "" {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO saas_modules(module_key, display_name, category, enabled_default, license_scope, metadata)
			VALUES ($1, $1, 'business', true, 'commercial', jsonb_build_object('tenant_bootstrap', true, 'sample_key', $2::text))
			ON CONFLICT (module_key) DO UPDATE SET enabled_default = true, metadata = saas_modules.metadata || EXCLUDED.metadata, updated_at = NOW()
		`, moduleKey, input.SampleKey); err != nil {
			return fmt.Errorf("bootstrap tenant module snapshot %s: %w", moduleKey, err)
		}
	}

	if input.IncludeBusinessClosureSample {
		if err := bootstrapBusinessClosureSample(ctx, tx, input, departmentID); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tenant bootstrap: %w", err)
	}
	return nil
}

func bootstrapBusinessClosureSample(ctx context.Context, db DB, input TenantBootstrapInput, departmentID uuid.UUID) error {
	if _, err := db.Exec(ctx, `
		INSERT INTO sample_work_orders(organization_id, work_order_no, product_name, quantity, status, metadata)
		VALUES ($1, 'WO-DEMO-001', 'Demo Assembly Kit', 120, 'planned', jsonb_build_object('sample_key', $2::text))
		ON CONFLICT (work_order_no) DO UPDATE SET
		    product_name = EXCLUDED.product_name,
		    quantity = EXCLUDED.quantity,
		    status = EXCLUDED.status,
		    metadata = sample_work_orders.metadata || EXCLUDED.metadata,
		    updated_at = NOW()
	`, input.OrganizationID, input.SampleKey); err != nil {
		return fmt.Errorf("bootstrap sample work order: %w", err)
	}

	if _, err := db.Exec(ctx, `
		WITH supplier AS (
			INSERT INTO business_partners(partner_code, partner_type, name, email, status, organization_id, metadata)
			VALUES ('SUP-DEMO', 'supplier', 'Demo Supplier', 'supplier@local.test', 'active', $1, jsonb_build_object('sample_key', $2::text))
			ON CONFLICT (organization_id, partner_code) WHERE partner_code <> ''
			DO UPDATE SET name = EXCLUDED.name, metadata = business_partners.metadata || EXCLUDED.metadata, updated_at = NOW()
			RETURNING id
		), customer AS (
			INSERT INTO business_partners(partner_code, partner_type, name, email, status, organization_id, metadata)
			VALUES ('CUS-DEMO', 'customer', 'Demo Customer', 'customer@local.test', 'active', $1, jsonb_build_object('sample_key', $2::text))
			ON CONFLICT (organization_id, partner_code) WHERE partner_code <> ''
			DO UPDATE SET name = EXCLUDED.name, metadata = business_partners.metadata || EXCLUDED.metadata, updated_at = NOW()
			RETURNING id
		), item AS (
			INSERT INTO items(item_code, name, item_type, base_uom, status, organization_id, metadata)
			VALUES ('ITM-DEMO', 'Demo Assembly Kit', 'material', 'EA', 'active', $1, jsonb_build_object('sample_key', $2::text))
			ON CONFLICT (organization_id, item_code) WHERE item_code <> ''
			DO UPDATE SET name = EXCLUDED.name, metadata = items.metadata || EXCLUDED.metadata, updated_at = NOW()
			RETURNING id
		), warehouse AS (
			INSERT INTO warehouses(warehouse_code, name, status, organization_id, department_id, metadata)
			VALUES ('WHS-DEMO', 'Demo Warehouse', 'active', $1, $3, jsonb_build_object('sample_key', $2::text))
			ON CONFLICT (organization_id, warehouse_code) WHERE warehouse_code <> ''
			DO UPDATE SET name = EXCLUDED.name, metadata = warehouses.metadata || EXCLUDED.metadata, updated_at = NOW()
			RETURNING id
		)
		INSERT INTO inventory_balances(item_id, warehouse_id, quantity, average_cost, value_amount, currency, organization_id, metadata)
		SELECT item.id, warehouse.id, 120, 25, 3000, 'CNY', $1, jsonb_build_object('sample_key', $2::text)
		FROM item, warehouse
		ON CONFLICT (item_id, warehouse_id, COALESCE(location_id, '00000000-0000-0000-0000-000000000000'::uuid))
		DO UPDATE SET quantity = EXCLUDED.quantity, average_cost = EXCLUDED.average_cost, value_amount = EXCLUDED.value_amount, metadata = inventory_balances.metadata || EXCLUDED.metadata, updated_at = NOW()
	`, input.OrganizationID, input.SampleKey, departmentID); err != nil {
		return fmt.Errorf("bootstrap sample inventory data: %w", err)
	}
	return nil
}
