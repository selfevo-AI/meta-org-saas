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
		INSERT INTO "MCRD"("CardCode", "Payload")
		VALUES
			('SUP-DEMO', jsonb_build_object('CardCode', 'SUP-DEMO', 'CardType', 'S', 'CardName', 'Demo Supplier', 'Email', 'supplier@local.test', 'organization_id', $1::uuid, 'sample_key', $2::text)),
			('CUS-DEMO', jsonb_build_object('CardCode', 'CUS-DEMO', 'CardType', 'C', 'CardName', 'Demo Customer', 'Email', 'customer@local.test', 'organization_id', $1::uuid, 'sample_key', $2::text))
		ON CONFLICT ("CardCode") DO UPDATE SET
			"Payload" = "MCRD"."Payload" || EXCLUDED."Payload",
			"UpdatedAt" = NOW();

		INSERT INTO "MITM"("ItemCode", "Payload")
		VALUES ('ITM-DEMO', jsonb_build_object('ItemCode', 'ITM-DEMO', 'ItemName', 'Demo Assembly Kit', 'ItemType', 'material', 'InvntryUom', 'EA', 'organization_id', $1::uuid, 'sample_key', $2::text))
		ON CONFLICT ("ItemCode") DO UPDATE SET
			"Payload" = "MITM"."Payload" || EXCLUDED."Payload",
			"UpdatedAt" = NOW();

		INSERT INTO "MWHS"("WhsCode", "Payload")
		VALUES ('WHS-DEMO', jsonb_build_object('WhsCode', 'WHS-DEMO', 'WhsName', 'Demo Warehouse', 'organization_id', $1::uuid, 'department_id', $3::uuid, 'sample_key', $2::text))
		ON CONFLICT ("WhsCode") DO UPDATE SET
			"Payload" = "MWHS"."Payload" || EXCLUDED."Payload",
			"UpdatedAt" = NOW();

		INSERT INTO "MITW"("ItemCode", "Payload")
		VALUES ('ITM-DEMO|WHS-DEMO', jsonb_build_object('ItemCode', 'ITM-DEMO|WHS-DEMO', 'BaseItemCode', 'ITM-DEMO', 'WhsCode', 'WHS-DEMO', 'OnHand', 120, 'AvgPrice', 25, 'StockValue', 3000, 'Currency', 'CNY', 'organization_id', $1::uuid, 'sample_key', $2::text))
		ON CONFLICT ("ItemCode") DO UPDATE SET
			"Payload" = "MITW"."Payload" || EXCLUDED."Payload",
			"UpdatedAt" = NOW();
	`, input.OrganizationID, input.SampleKey, departmentID); err != nil {
		return fmt.Errorf("bootstrap sample ERP code-table inventory data: %w", err)
	}
	return nil
}
