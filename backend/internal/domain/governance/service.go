package governance

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/selfevo-AI/meta-org/backend/internal/pkg/middleware"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrValidation = errors.New("validation error")
	ErrForbidden  = errors.New("forbidden")
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreatePermission(ctx context.Context, p *Permission) (*Permission, error) {
	if p.Name == "" {
		return nil, fmt.Errorf("%w: name is required", ErrValidation)
	}
	if p.Level < 1 || p.Level > 4 {
		return nil, fmt.Errorf("%w: level must be between 1 and 4", ErrValidation)
	}
	if p.Behavior != "auto" && p.Behavior != "notify" && p.Behavior != "approve" && p.Behavior != "deny" {
		p.Behavior = "notify"
	}
	return s.repo.CreatePermission(ctx, p)
}

func (s *Service) ListPermissions(ctx context.Context) ([]Permission, error) {
	return s.repo.ListPermissions(ctx)
}

func (s *Service) CreatePrinciple(ctx context.Context, input CreatePrincipleInput) (*Principle, error) {
	if input.Name == "" || input.Description == "" {
		return nil, fmt.Errorf("%w: name and description are required", ErrValidation)
	}
	return s.repo.CreatePrinciple(ctx, input)
}

func (s *Service) ListPrinciples(ctx context.Context) ([]Principle, error) {
	return s.repo.ListPrinciples(ctx)
}

func (s *Service) GetPrinciple(ctx context.Context, id uuid.UUID) (*Principle, error) {
	return s.repo.GetPrinciple(ctx, id)
}

func (s *Service) CreateControlRule(ctx context.Context, input CreateControlRuleInput) (*ControlRule, error) {
	if input.Action == "" {
		return nil, fmt.Errorf("%w: action is required", ErrValidation)
	}
	return s.repo.CreateControlRule(ctx, input)
}

func (s *Service) ListControlRules(ctx context.Context) ([]ControlRule, error) {
	return s.repo.ListControlRules(ctx)
}

func (s *Service) CheckPermission(ctx context.Context, input PermissionCheckInput) (*PermissionCheckResult, error) {
	decision, err := s.DecideAccess(ctx, AccessDecisionInput{
		ActorID:       input.UserID,
		ActorType:     "internal_human",
		Action:        input.Action,
		Resource:      input.Resource,
		ResourceID:    input.ResourceID,
		RequiredLevel: "L1",
		RiskLevel:     "low",
	})
	if err != nil {
		return nil, err
	}
	return &PermissionCheckResult{
		Allowed:  decision.Allowed,
		Level:    permissionLevelWeight(decision.RequiredLevel),
		Behavior: decision.Behavior,
		Reason:   decision.Reason,
	}, nil
}

func (s *Service) DecideAccess(ctx context.Context, input AccessDecisionInput) (*AccessDecision, error) {
	if input.ActorID == uuid.Nil {
		return nil, fmt.Errorf("%w: actor_id is required", ErrValidation)
	}
	if input.ActorType == "" || input.Action == "" || input.Resource == "" {
		return nil, fmt.Errorf("%w: actor_type, action, and resource are required", ErrValidation)
	}
	if input.RequiredLevel == "" {
		input.RequiredLevel = "L1"
	}
	if input.RiskLevel == "" {
		input.RiskLevel = "low"
	}
	if input.Context == nil {
		input.Context = map[string]any{}
	}
	if err := applyTenantAccessDecisionScope(ctx, &input); err != nil {
		return nil, err
	}

	behavior := "notify"
	matchedRules := []string{"audit-first-default"}
	if p, err := s.repo.GetPermissionByLevel(ctx, permissionLevelWeight(input.RequiredLevel)); err == nil && p.Behavior != "" {
		behavior = p.Behavior
		matchedRules = append(matchedRules, "permission-level:"+p.Name)
	}

	decision := behavior
	reason := "audit allowed"
	switch behavior {
	case "auto":
		decision = "allow"
		reason = "permission behavior auto"
	case "notify":
		decision = "notify"
		reason = "permission behavior notify"
	case "approve":
		decision = "approve"
		reason = "permission behavior requires approval"
	case "deny":
		decision = "deny"
		reason = "permission behavior denies access"
	}

	riskWeight := riskLevelWeight(input.RiskLevel)
	requiredWeight := permissionLevelWeight(input.RequiredLevel)
	if decision == "allow" || decision == "notify" {
		if riskWeight >= 3 || requiredWeight >= 4 {
			decision = "approve"
			behavior = "approve"
			reason = "high risk or L4 action requires human approval"
			matchedRules = append(matchedRules, "high-risk-approval")
		}
		if input.WeightSnapshot != nil && *input.WeightSnapshot < 0.35 && riskWeight >= 2 {
			decision = "approve"
			behavior = "approve"
			reason = "actor context weight below approval threshold"
			matchedRules = append(matchedRules, "low-weight-approval")
		}
		if input.WeightSnapshot != nil && *input.WeightSnapshot < 0.20 {
			decision = "deny"
			behavior = "deny"
			reason = "actor context weight below deny threshold"
			matchedRules = append(matchedRules, "low-weight-deny")
		}
	}
	if input.ActorType == "external_agent" && riskWeight >= 4 {
		decision = "approve"
		behavior = "approve"
		reason = "external service agent on critical risk requires human approval"
		matchedRules = append(matchedRules, "external-agent-critical-approval")
	}

	allowed := decision == "allow" || decision == "notify"
	return s.repo.CreateAccessDecision(ctx, input, decision, behavior, reason, allowed, matchedRules)
}

func (s *Service) ListAccessDecisions(ctx context.Context, limit int) ([]AccessDecision, error) {
	return s.repo.ListAccessDecisions(ctx, limit)
}

func permissionLevelWeight(level string) int {
	switch level {
	case "L1":
		return 1
	case "L2":
		return 2
	case "L3":
		return 3
	case "L4":
		return 4
	default:
		return 1
	}
}

func riskLevelWeight(level string) int {
	switch level {
	case "low":
		return 1
	case "medium":
		return 2
	case "high":
		return 3
	case "critical":
		return 4
	default:
		return 1
	}
}

func validPreferenceKey(key string) bool {
	if key == "" || len(key) > 120 {
		return false
	}
	for _, r := range key {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func (s *Service) ListDataTables(ctx context.Context, category string) ([]DataTable, error) {
	if category == "" {
		category = "canonical"
	}
	if category == "all" {
		category = ""
	}
	return s.repo.ListDataTables(ctx, category)
}

func (s *Service) ListDataFields(ctx context.Context, tableName string) ([]DataField, error) {
	if tableName == "" {
		return nil, fmt.Errorf("%w: table_name is required", ErrValidation)
	}
	return s.repo.ListDataFields(ctx, tableName)
}

func (s *Service) GetUserFieldPreference(ctx context.Context, actorID, tableName string) (*UserFieldPreference, error) {
	if actorID == "" || tableName == "" {
		return nil, fmt.Errorf("%w: actor_id and table_name are required", ErrValidation)
	}
	pref, err := s.repo.GetUserFieldPreference(ctx, actorID, tableName)
	if err == nil {
		return pref, nil
	}
	fields, fieldErr := s.repo.ListDataFields(ctx, tableName)
	if fieldErr != nil {
		return nil, err
	}
	visible := make([]string, 0)
	order := make([]string, 0, len(fields))
	for _, field := range fields {
		order = append(order, field.FieldName)
		if field.IsVisibleDefault {
			visible = append(visible, field.FieldName)
		}
	}
	return &UserFieldPreference{
		ActorID:       actorID,
		TableName:     tableName,
		VisibleFields: visible,
		FieldOrder:    order,
		FieldWidths:   map[string]int{},
	}, nil
}

func (s *Service) UpsertUserFieldPreference(ctx context.Context, actorID, tableName string, input UpsertUserFieldPreferenceInput) (*UserFieldPreference, error) {
	if actorID == "" || tableName == "" {
		return nil, fmt.Errorf("%w: actor_id and table_name are required", ErrValidation)
	}
	if input.VisibleFields == nil {
		input.VisibleFields = []string{}
	}
	if input.FieldOrder == nil {
		input.FieldOrder = []string{}
	}
	if input.FieldWidths == nil {
		input.FieldWidths = map[string]int{}
	}
	return s.repo.UpsertUserFieldPreference(ctx, actorID, tableName, input)
}

func (s *Service) GetUserUIPreference(ctx context.Context, actorID, preferenceKey string) (*UserUIPreference, error) {
	if actorID == "" || !validPreferenceKey(preferenceKey) {
		return nil, fmt.Errorf("%w: actor_id and preference_key are required", ErrValidation)
	}
	pref, err := s.repo.GetUserUIPreference(ctx, actorID, preferenceKey)
	if err == nil {
		return pref, nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return &UserUIPreference{
			ActorID:       actorID,
			PreferenceKey: preferenceKey,
			Value:         map[string]any{},
		}, nil
	}
	return nil, err
}

func (s *Service) UpsertUserUIPreference(ctx context.Context, actorID, preferenceKey string, input UpsertUserUIPreferenceInput) (*UserUIPreference, error) {
	if actorID == "" || !validPreferenceKey(preferenceKey) {
		return nil, fmt.Errorf("%w: actor_id and preference_key are required", ErrValidation)
	}
	if input.Value == nil {
		input.Value = map[string]any{}
	}
	return s.repo.UpsertUserUIPreference(ctx, actorID, preferenceKey, input)
}

func (s *Service) CreateFieldPermissionRule(ctx context.Context, input CreateFieldPermissionRuleInput) (*FieldPermissionRule, error) {
	if input.OrganizationID == nil {
		input.OrganizationID = currentTenantOrganizationID(ctx)
	}
	if input.ScopeType == "" {
		input.ScopeType = "organization"
	}
	if input.ScopeType != "organization" && input.ScopeType != "department" && input.ScopeType != "project" && input.ScopeType != "function" && input.ScopeType != "form" && input.ScopeType != "field" {
		return nil, fmt.Errorf("%w: invalid scope_type", ErrValidation)
	}
	if input.TableName == "" {
		return nil, fmt.Errorf("%w: table_name is required", ErrValidation)
	}
	if input.FieldName == "" {
		input.FieldName = "*"
	}
	if input.ActorType == "" {
		input.ActorType = "*"
	}
	if input.Action == "" {
		input.Action = "read"
	}
	if input.Action != "read" && input.Action != "write" && input.Action != "delete" && input.Action != "admin" {
		return nil, fmt.Errorf("%w: invalid action", ErrValidation)
	}
	if input.Behavior == "" {
		input.Behavior = "allow"
	}
	if input.Behavior != "allow" && input.Behavior != "notify" && input.Behavior != "approve" && input.Behavior != "deny" {
		return nil, fmt.Errorf("%w: invalid behavior", ErrValidation)
	}
	if input.RequiredLevel == "" {
		input.RequiredLevel = "L1"
	}
	if !isPermissionLevel(input.RequiredLevel) {
		return nil, fmt.Errorf("%w: invalid required_level", ErrValidation)
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
	return s.repo.CreateFieldPermissionRule(ctx, input)
}

func (s *Service) ListFieldPermissionRules(ctx context.Context, tableName string) ([]FieldPermissionRule, error) {
	return s.repo.ListFieldPermissionRules(ctx, tableName)
}

func (s *Service) CheckFieldAccess(ctx context.Context, input FieldAccessCheckInput) (*FieldAccessCheckResult, error) {
	if input.OrganizationID == nil {
		input.OrganizationID = currentTenantOrganizationID(ctx)
	}
	if input.ScopeType == "" {
		input.ScopeType = "organization"
	}
	if input.ActorID == "" || input.ActorType == "" || input.TableName == "" {
		return nil, fmt.Errorf("%w: actor_id, actor_type, and table_name are required", ErrValidation)
	}
	if input.Action == "" {
		input.Action = "read"
	}
	return s.repo.CheckFieldAccess(ctx, input)
}

func isPermissionLevel(level string) bool {
	return level == "L1" || level == "L2" || level == "L3" || level == "L4"
}

func applyTenantAccessDecisionScope(ctx context.Context, input *AccessDecisionInput) error {
	orgID := currentTenantOrganizationID(ctx)
	if orgID == nil {
		return nil
	}
	if input.OrganizationID != nil && *input.OrganizationID != *orgID {
		return fmt.Errorf("%w: access decision organization must match current organization", ErrForbidden)
	}
	id := *orgID
	input.OrganizationID = &id
	return nil
}

func currentTenantOrganizationID(ctx context.Context) *uuid.UUID {
	tenant, ok := middleware.TenantFromContext(ctx)
	if !ok || tenant.OrganizationID == nil {
		return nil
	}
	id := *tenant.OrganizationID
	return &id
}
