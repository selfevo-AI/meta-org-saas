package assistant

import (
	"context"

	"github.com/google/uuid"
)

type AssistantRuntimeRunner interface {
	Run(context.Context, AssistantRunRequest) (<-chan RunEvent, error)
	Resume(context.Context, AssistantResumeRequest) (<-chan RunEvent, error)
}

type AssistantRunRequest struct {
	SessionID uuid.UUID
	ActorID   uuid.UUID
	ActorType string
	Input     RunInput
}

type AssistantResumeRequest struct {
	SessionID uuid.UUID
	ActorID   uuid.UUID
	ActorType string
	Input     ResumeInput
}

type ContextPackageBuilder interface {
	BuildContextPackage(context.Context, ContextRequest) (*ContextPackage, error)
}

type AssistantRuntime struct {
	service       *Service
	contextEngine ContextPackageBuilder
	toolRunner    *ToolRunner
	eventSink     EventSink
}

func NewAssistantRuntime(service *Service, contextEngine ContextPackageBuilder, toolRunner *ToolRunner, eventSink EventSink) *AssistantRuntime {
	return &AssistantRuntime{service: service, contextEngine: contextEngine, toolRunner: toolRunner, eventSink: eventSink}
}

func (r *AssistantRuntime) Run(ctx context.Context, request AssistantRunRequest) (<-chan RunEvent, error) {
	return r.service.runWithContextEngine(ctx, request.SessionID, request.ActorID, request.ActorType, request.Input, r.contextEngine)
}

func (r *AssistantRuntime) Resume(ctx context.Context, request AssistantResumeRequest) (<-chan RunEvent, error) {
	return r.service.resumeWithContextEngine(ctx, request.SessionID, request.ActorID, request.ActorType, request.Input, r.contextEngine)
}
