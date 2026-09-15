package toolruntime

import (
	"context"
	"fmt"

	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/erp"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/ontology"
)

func OntologyTools(service OntologyService) map[string]ToolAdapter {
	if service == nil {
		return map[string]ToolAdapter{}
	}
	return map[string]ToolAdapter{
		"ontology.types.list": func(ctx context.Context, input ExecuteToolInput) (ToolResult, error) {
			types, err := service.Types(ctx)
			return ToolResult{Summary: "Accessible business object types", Data: map[string]any{"object_types": types}}, err
		},
		"ontology.objects.query": func(ctx context.Context, input ExecuteToolInput) (ToolResult, error) {
			result, err := service.Query(ctx, stringArg(input.Arguments, "object_type"), ontology.QueryInput{Filters: mapArg(input.Arguments, "filters"), Search: stringArg(input.Arguments, "search"), Cursor: stringArg(input.Arguments, "cursor"), Limit: 50})
			return ToolResult{Summary: "Business objects queried", Data: map[string]any{"page": result}}, err
		},
		"ontology.objects.get": func(ctx context.Context, input ExecuteToolInput) (ToolResult, error) {
			result, err := service.Get(ctx, stringArg(input.Arguments, "object_type"), stringArg(input.Arguments, "key"))
			return ToolResult{Summary: "Business object retrieved", Data: map[string]any{"object": result}}, err
		},
		"ontology.objects.links": func(ctx context.Context, input ExecuteToolInput) (ToolResult, error) {
			result, err := service.Links(ctx, stringArg(input.Arguments, "object_type"), stringArg(input.Arguments, "key"))
			return ToolResult{Summary: "Business relationships retrieved", Data: map[string]any{"relationships": result}}, err
		},
		"ontology.action.execute": func(ctx context.Context, input ExecuteToolInput) (ToolResult, error) {
			typeKey, key, action := stringArg(input.Arguments, "object_type"), stringArg(input.Arguments, "key"), stringArg(input.Arguments, "action")
			if typeKey == "" || key == "" || action == "" {
				return ToolResult{}, fmt.Errorf("%w: object_type, key and action are required", ErrValidation)
			}
			executionID, err := optionalUUIDArg(input.Arguments, "tool_execution_id")
			if err != nil {
				return ToolResult{}, err
			}
			sessionID, err := optionalUUIDArg(input.Arguments, "assistant_session_id")
			if err != nil {
				return ToolResult{}, err
			}
			actorID := input.ActorID
			result, err := service.Execute(erp.WithApprovedAction(ctx), typeKey, key, action, erp.ActionInput{
				Data: mapArg(input.Arguments, "data"), ActorID: &actorID, ActorType: input.ActorType,
				IdempotencyKey: input.IdempotencyKey, Source: "ontology_tool", ToolExecutionID: executionID, AssistantSessionID: sessionID,
			})
			return ToolResult{Summary: "Approved business action executed", Data: map[string]any{"ontology_action": result}}, err
		},
	}
}

func OntologyToolDefinitions() []CreateToolInput {
	definitions := []CreateToolInput{}
	for _, spec := range []struct {
		name, zh, en string
		required     []string
	}{
		{"ontology.types.list", "查询业务对象类型", "List accessible business object types and their properties, links and actions", nil},
		{"ontology.objects.query", "查询业务对象", "Query authoritative business objects using semantic property filters; follow next_cursor for more results", []string{"object_type"}},
		{"ontology.objects.get", "读取业务对象", "Get an authoritative business object", []string{"object_type", "key"}},
		{"ontology.objects.links", "查询业务对象关系", "Read the upstream and downstream relationships of a business object", []string{"object_type", "key"}},
		{"ontology.action.execute", "执行已批准的业务动作", "Request approval and execute a business action on an existing object; use action parameters from ontology.types.list", []string{"object_type", "key", "action"}},
	} {
		properties := map[string]any{}
		for _, name := range spec.required {
			properties[name] = map[string]any{"type": "string"}
		}
		if spec.name == "ontology.objects.query" {
			properties["filters"] = map[string]any{"type": "object"}
			properties["search"], properties["cursor"] = map[string]any{"type": "string"}, map[string]any{"type": "string"}
		}
		if spec.name == "ontology.action.execute" {
			properties["data"] = map[string]any{"type": "object"}
		}
		definition := CreateToolInput{Name: spec.name, Description: spec.en, SourceType: SourceInternalAPI, DefaultPolicy: PolicyNotify, RiskLevel: "low", RequiredLevel: "L1", ToolCategory: ToolCategoryExecutionOperation, ApprovalTierRequired: ApprovalTierExecutor,
			InputSchema: map[string]any{"type": "object", "properties": properties}, Metadata: bilingualToolMetadata(spec.zh, spec.en)}
		if len(spec.required) > 0 {
			definition.InputSchema["required"] = spec.required
		}
		if spec.name == "ontology.action.execute" {
			definition.DefaultPolicy, definition.RiskLevel, definition.RequiredLevel = PolicyApprove, "high", "L3"
			definition.ToolCategory, definition.ApprovalTierRequired = ToolCategoryBusinessApproval, ApprovalTierReviewer
		}
		definitions = append(definitions, definition)
	}
	return definitions
}
