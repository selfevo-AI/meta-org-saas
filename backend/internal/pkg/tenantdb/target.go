package tenantdb

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"
)

const (
	DeploymentModeDedicatedDatabase = "dedicated_database"
	DeploymentModeSharedSchema      = "shared_schema"

	TargetStatusProvisioning = "provisioning"
	TargetStatusProvisioned  = "provisioned"
	TargetStatusFailed       = "failed"
	TargetStatusArchived     = "archived"
)

type Target struct {
	OrganizationID      uuid.UUID
	DeploymentMode      string
	ClusterKey          string
	Region              string
	DatabaseName        string
	SchemaName          string
	ConnectionSecretRef string
	MigrationVersion    string
	Status              string
	Metadata            map[string]any
}

type Defaults struct {
	DeploymentMode     string
	DatabaseNamePrefix string
	ClusterKey         string
	Region             string
}

type DataSourceResolver struct {
	adminURL string
}

func NewDataSourceResolver(adminURL string) DataSourceResolver {
	return DataSourceResolver{adminURL: strings.TrimSpace(adminURL)}
}

func (r DataSourceResolver) URLForTarget(target Target) (string, error) {
	switch target.DeploymentMode {
	case DeploymentModeSharedSchema:
		if r.adminURL == "" {
			return "", fmt.Errorf("tenant database admin url is required")
		}
		return r.adminURL, nil
	default:
		return DatabaseURLForName(r.adminURL, target.DatabaseName)
	}
}

func (d Defaults) TargetForOrganization(id uuid.UUID) Target {
	switch strings.ToLower(strings.TrimSpace(d.DeploymentMode)) {
	case DeploymentModeSharedSchema:
		return Target{
			OrganizationID: id,
			DeploymentMode: DeploymentModeSharedSchema,
			ClusterKey:     firstNonEmpty(d.ClusterKey, "local-primary"),
			Region:         firstNonEmpty(d.Region, "local"),
			SchemaName:     SchemaNameForOrganization(id),
			Status:         TargetStatusProvisioned,
			Metadata: map[string]any{
				"source": "tenant_database_defaults",
			},
		}
	default:
		return NewDedicatedDatabaseTarget(id, d.DatabaseNamePrefix, d.ClusterKey, d.Region)
	}
}

func DatabaseNameForOrganization(prefix string, id uuid.UUID) string {
	hexID := strings.ReplaceAll(strings.ToLower(id.String()), "-", "")
	if len(hexID) > 4 {
		hexID = hexID[:4]
	}
	return normalizeDatabaseNamePrefix(prefix) + hexID
}

func NewDedicatedDatabaseTarget(id uuid.UUID, prefix string, clusterKey string, region string) Target {
	clusterKey = strings.TrimSpace(clusterKey)
	if clusterKey == "" {
		clusterKey = "local-primary"
	}
	region = strings.TrimSpace(region)
	if region == "" {
		region = "local"
	}
	return Target{
		OrganizationID: id,
		DeploymentMode: DeploymentModeDedicatedDatabase,
		ClusterKey:     clusterKey,
		Region:         region,
		DatabaseName:   DatabaseNameForOrganization(prefix, id),
		SchemaName:     "public",
		Status:         TargetStatusProvisioning,
		Metadata: map[string]any{
			"source": "tenant_database_target",
		},
	}
}

func DatabaseURLForName(adminURL string, databaseName string) (string, error) {
	databaseName = strings.TrimSpace(databaseName)
	if _, err := QuoteIdentifier(databaseName); err != nil {
		return "", fmt.Errorf("invalid database name: %w", err)
	}
	parsed, err := url.Parse(strings.TrimSpace(adminURL))
	if err != nil {
		return "", fmt.Errorf("parse tenant database admin url: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("tenant database admin url must include scheme and host")
	}
	parsed.Path = "/" + databaseName
	return parsed.String(), nil
}

func normalizeDatabaseNamePrefix(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "meta_org_"
	}
	var b strings.Builder
	lastUnderscore := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastUnderscore = false
		default:
			if !lastUnderscore {
				b.WriteByte('_')
				lastUnderscore = true
			}
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "meta_org_"
	}
	if out[0] >= '0' && out[0] <= '9' {
		out = "tenant_" + out
	}
	return out + "_"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
