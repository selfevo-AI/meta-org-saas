package identity

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateUser(ctx context.Context, input CreateUserInput) (*User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := &User{}
	err = r.db.QueryRow(ctx,
		`INSERT INTO users (name, email, password_hash) VALUES ($1, $2, $3)
		 RETURNING id, name, email, password_hash, COALESCE(avatar_url, ''), created_at, updated_at`,
		strings.TrimSpace(input.Name), strings.ToLower(strings.TrimSpace(input.Email)), string(hash),
	).Scan(&user.ID, &user.Name, &user.Email, &user.PasswordHash, &user.AvatarURL, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	user := &User{}
	err := r.db.QueryRow(ctx,
		`SELECT id, name, email, password_hash, COALESCE(avatar_url, ''), created_at, updated_at
		 FROM users WHERE lower(email) = lower($1)`, strings.TrimSpace(email),
	).Scan(&user.ID, &user.Name, &user.Email, &user.PasswordHash, &user.AvatarURL, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return user, nil
}

func (r *Repository) GetUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	user := &User{}
	err := r.db.QueryRow(ctx,
		`SELECT id, name, email, password_hash, COALESCE(avatar_url, ''), created_at, updated_at
		 FROM users WHERE id = $1`, id,
	).Scan(&user.ID, &user.Name, &user.Email, &user.PasswordHash, &user.AvatarURL, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return user, nil
}

func (r *Repository) UpdateUserPassword(ctx context.Context, id uuid.UUID, passwordHash string) error {
	tag, err := r.db.Exec(ctx, `UPDATE users SET password_hash = $2, updated_at = NOW() WHERE id = $1`, id, passwordHash)
	if err != nil {
		return fmt.Errorf("update user password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (r *Repository) CreateAgent(ctx context.Context, input CreateAgentInput) (*AIAgent, string, error) {
	apiKey := uuid.New().String()
	keyHash, err := bcrypt.GenerateFromPassword([]byte(apiKey), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", fmt.Errorf("hash api key: %w", err)
	}

	capJSON, err := json.Marshal(input.Capabilities)
	if err != nil {
		return nil, "", fmt.Errorf("marshal capabilities: %w", err)
	}
	metaJSON, err := json.Marshal(input.Metadata)
	if err != nil {
		return nil, "", fmt.Errorf("marshal metadata: %w", err)
	}

	agent := &AIAgent{}
	err = r.db.QueryRow(ctx,
		`INSERT INTO ai_agents (name, model_type, api_key_hash, capabilities, permission_level, agent_origin, provider, service_class, vendor, contract_ref, risk_level, metadata)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		 RETURNING id, name, model_type, api_key_hash, capabilities, permission_level, agent_origin, COALESCE(provider, ''), service_class, COALESCE(vendor, ''), COALESCE(contract_ref, ''), risk_level, metadata, is_active, created_at, updated_at`,
		input.Name, input.ModelType, string(keyHash), capJSON, input.PermissionLevel, input.AgentOrigin, input.Provider, input.ServiceClass, input.Vendor, input.ContractRef, input.RiskLevel, metaJSON,
	).Scan(&agent.ID, &agent.Name, &agent.ModelType, &agent.APIKeyHash, &capJSON, &agent.PermissionLevel, &agent.AgentOrigin, &agent.Provider, &agent.ServiceClass, &agent.Vendor, &agent.ContractRef, &agent.RiskLevel, &metaJSON, &agent.IsActive, &agent.CreatedAt, &agent.UpdatedAt)
	if err != nil {
		return nil, "", fmt.Errorf("create agent: %w", err)
	}
	if err := hydrateAgentJSON(agent, capJSON, metaJSON); err != nil {
		return nil, "", fmt.Errorf("unmarshal metadata: %w", err)
	}
	return agent, apiKey, nil
}

func (r *Repository) GetAgentByID(ctx context.Context, id uuid.UUID) (*AIAgent, error) {
	agent := &AIAgent{}
	var capJSON, metaJSON []byte
	err := r.db.QueryRow(ctx,
		`SELECT id, name, model_type, api_key_hash, capabilities, permission_level, agent_origin, COALESCE(provider, ''), service_class, COALESCE(vendor, ''), COALESCE(contract_ref, ''), risk_level, metadata, is_active, created_at, updated_at
		 FROM ai_agents WHERE id = $1`, id,
	).Scan(&agent.ID, &agent.Name, &agent.ModelType, &agent.APIKeyHash, &capJSON, &agent.PermissionLevel, &agent.AgentOrigin, &agent.Provider, &agent.ServiceClass, &agent.Vendor, &agent.ContractRef, &agent.RiskLevel, &metaJSON, &agent.IsActive, &agent.CreatedAt, &agent.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get agent by id: %w", err)
	}
	if err := hydrateAgentJSON(agent, capJSON, metaJSON); err != nil {
		return nil, err
	}
	return agent, nil
}

func (r *Repository) ListAgents(ctx context.Context, limit int) ([]AIAgent, error) {
	if limit <= 0 {
		limit = 50
	} else if limit > 100 {
		limit = 100
	}
	rows, err := r.db.Query(ctx,
		`SELECT id, name, model_type, api_key_hash, capabilities, permission_level, agent_origin, COALESCE(provider, ''), service_class, COALESCE(vendor, ''), COALESCE(contract_ref, ''), risk_level, metadata, is_active, created_at, updated_at
		 FROM ai_agents ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	defer rows.Close()

	var agents []AIAgent
	for rows.Next() {
		var agent AIAgent
		var capJSON, metaJSON []byte
		if err := rows.Scan(&agent.ID, &agent.Name, &agent.ModelType, &agent.APIKeyHash, &capJSON, &agent.PermissionLevel, &agent.AgentOrigin, &agent.Provider, &agent.ServiceClass, &agent.Vendor, &agent.ContractRef, &agent.RiskLevel, &metaJSON, &agent.IsActive, &agent.CreatedAt, &agent.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan agent: %w", err)
		}
		if err := hydrateAgentJSON(&agent, capJSON, metaJSON); err != nil {
			return nil, err
		}
		agents = append(agents, agent)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list agents iteration: %w", err)
	}
	return agents, nil
}

func (r *Repository) ListAgentsByOrganization(ctx context.Context, orgID uuid.UUID, limit int) ([]AIAgent, error) {
	if limit <= 0 {
		limit = 50
	} else if limit > 100 {
		limit = 100
	}
	rows, err := r.db.Query(ctx,
		`SELECT DISTINCT a.id, a.name, a.model_type, a.api_key_hash, a.capabilities, a.permission_level,
		        a.agent_origin, COALESCE(a.provider, ''), a.service_class, COALESCE(a.vendor, ''),
		        COALESCE(a.contract_ref, ''), a.risk_level, a.metadata, a.is_active, a.created_at, a.updated_at
		 FROM ai_agents a
		 JOIN organization_memberships om ON om.agent_id = a.id
		 WHERE om.organization_id = $1
		   AND om.member_type = 'agent'
		   AND om.status = 'active'
		 ORDER BY a.created_at DESC
		 LIMIT $2`, orgID, limit)
	if err != nil {
		return nil, fmt.Errorf("list organization agents: %w", err)
	}
	defer rows.Close()

	var agents []AIAgent
	for rows.Next() {
		var agent AIAgent
		var capJSON, metaJSON []byte
		if err := rows.Scan(&agent.ID, &agent.Name, &agent.ModelType, &agent.APIKeyHash, &capJSON, &agent.PermissionLevel, &agent.AgentOrigin, &agent.Provider, &agent.ServiceClass, &agent.Vendor, &agent.ContractRef, &agent.RiskLevel, &metaJSON, &agent.IsActive, &agent.CreatedAt, &agent.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan organization agent: %w", err)
		}
		if err := hydrateAgentJSON(&agent, capJSON, metaJSON); err != nil {
			return nil, err
		}
		agents = append(agents, agent)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list organization agents iteration: %w", err)
	}
	return agents, nil
}

func (r *Repository) AttachAgentToOrganization(ctx context.Context, agentID uuid.UUID, orgID uuid.UUID) error {
	var departmentID uuid.UUID
	if err := r.db.QueryRow(ctx, `
		SELECT id
		FROM departments
		WHERE organization_id = $1 AND status = 'active'
		ORDER BY parent_id NULLS FIRST, sort_order, created_at
		LIMIT 1
	`, orgID).Scan(&departmentID); err != nil {
		return fmt.Errorf("get organization department for agent: %w", err)
	}
	if _, err := r.db.Exec(ctx, `
		INSERT INTO organization_memberships (
			organization_id, department_id, member_type, agent_id, title, authority_tier, status, metadata
		)
		VALUES ($1, $2, 'agent', $3, 'Agent', 'executor', 'active', '{"source":"agent_registration"}'::jsonb)
		ON CONFLICT (department_id, agent_id) WHERE member_type = 'agent'
		DO UPDATE SET status = 'active',
		              authority_tier = 'executor',
		              updated_at = NOW()
	`, orgID, departmentID, agentID); err != nil {
		return fmt.Errorf("attach agent to organization: %w", err)
	}
	return nil
}

func (r *Repository) ListRoles(ctx context.Context) ([]Role, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, name, role_type, COALESCE(description, ''), permissions FROM roles ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	defer rows.Close()

	var roles []Role
	for rows.Next() {
		var role Role
		var permJSON []byte
		if err := rows.Scan(&role.ID, &role.Name, &role.RoleType, &role.Description, &permJSON); err != nil {
			return nil, fmt.Errorf("scan role: %w", err)
		}
		if err := json.Unmarshal(permJSON, &role.Permissions); err != nil {
			return nil, fmt.Errorf("unmarshal role permissions: %w", err)
		}
		roles = append(roles, role)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list roles iteration: %w", err)
	}
	return roles, nil
}

func hydrateAgentJSON(agent *AIAgent, capJSON, metaJSON []byte) error {
	if err := json.Unmarshal(capJSON, &agent.Capabilities); err != nil {
		return fmt.Errorf("unmarshal capabilities: %w", err)
	}
	if err := json.Unmarshal(metaJSON, &agent.Metadata); err != nil {
		return fmt.Errorf("unmarshal metadata: %w", err)
	}
	return nil
}
