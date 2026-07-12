package aigateway

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/dberrors"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/model-providers", h.createProvider)
	r.Get("/model-providers", h.listProviders)
	r.Patch("/model-providers/{id}", h.updateProvider)
	r.Post("/model-providers/{id}/rotate-key", h.rotateProviderKey)
	r.Post("/model-providers/{id}/test", h.testProvider)
	r.Post("/model-providers/{id}/channels", h.createChannel)
	r.Get("/model-providers/{id}/channels", h.listProviderChannels)
	r.Get("/model-provider-channels", h.listChannels)
	r.Patch("/model-provider-channels/{id}", h.updateChannel)
	r.Post("/model-provider-channels/{id}/rotate-key", h.rotateChannelKey)
	r.Post("/model-provider-channels/{id}/test", h.testChannel)
	r.Post("/models", h.createModel)
	r.Get("/models", h.listModels)
	r.Patch("/models/{id}", h.updateModel)
	r.Post("/ai-gateway/invoke", h.invoke)
	r.Get("/ai-gateway/stream", h.stream)
	r.Post("/ai-gateway/stream", h.streamPost)
	r.Get("/ai-gateway/routing-rules", h.listRoutingRules)
	r.Post("/ai-gateway/routing-rules", h.createRoutingRule)
	r.Get("/ai-gateway/model-groups", h.listModelGroups)
	r.Post("/ai-gateway/model-groups", h.createModelGroup)
	r.Get("/ai-gateway/model-channel-abilities", h.listModelChannelAbilities)
	r.Post("/ai-gateway/model-channel-abilities", h.createModelChannelAbility)
	r.Get("/ai-gateway/access-tokens", h.listAccessTokens)
	r.Post("/ai-gateway/access-tokens", h.createAccessToken)
	r.Get("/ai-gateway/adapters", h.adapterCatalog)
	r.Get("/ai-gateway/balance", h.getGatewayBalance)
	r.Post("/ai-gateway/balance-adjustments", h.adjustGatewayBalance)
	r.Get("/ai-gateway/balance-transactions", h.listBalanceTransactions)
	r.Get("/ai-gateway/usage-analysis", h.usageAnalysis)
	r.Post("/ai-gateway/estimate-cost", h.estimateCost)
	r.Get("/ai-gateway/invocations", h.listInvocations)
	r.Get("/ai-gateway/invocations/{id}", h.getInvocation)
	r.Get("/ai-gateway/cost-summary", h.costSummary)
}

func (h *Handler) RegisterTenantRoutes(r chi.Router) {
	r.Get("/model-providers", h.listProviders)
	r.Get("/models", h.listModels)
	r.Post("/ai-gateway/invoke", h.invoke)
	r.Get("/ai-gateway/stream", h.stream)
	r.Post("/ai-gateway/stream", h.streamPost)
	r.Get("/ai-gateway/invocations", h.listInvocations)
	r.Get("/ai-gateway/invocations/{id}", h.getInvocation)
	r.Get("/ai-gateway/cost-summary", h.costSummary)
}

func (h *Handler) RegisterCompatibleRoutes(r chi.Router, middlewares ...func(http.Handler) http.Handler) {
	router := r
	if len(middlewares) > 0 {
		router = r.With(middlewares...)
	}
	router.Get("/v1/models", h.compatibleListModels)
	router.Post("/v1/chat/completions", h.compatibleChatCompletions)
	router.Post("/v1/embeddings", h.compatibleUnsupportedOperation("embeddings"))
	router.Post("/v1/images/generations", h.compatibleUnsupportedOperation("images"))
	router.Post("/v1/audio/transcriptions", h.compatibleUnsupportedOperation("audio"))
	router.Post("/v1/moderations", h.compatibleUnsupportedOperation("moderations"))
}

func (h *Handler) createProvider(w http.ResponseWriter, r *http.Request) {
	var input CreateProviderInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreateProvider(r.Context(), input)
	writeResult(w, http.StatusCreated, result, err)
}

type compatibleChatCompletionRequest struct {
	Model       string     `json:"model"`
	Messages    []Message  `json:"messages"`
	Temperature *float64   `json:"temperature,omitempty"`
	MaxTokens   int        `json:"max_tokens,omitempty"`
	Stream      bool       `json:"stream,omitempty"`
	Tools       []ToolSpec `json:"tools,omitempty"`
}

type compatibleChatCompletionResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []compatibleChatChoice `json:"choices"`
	Usage   compatibleChatUsage    `json:"usage"`
}

type compatibleChatChoice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type compatibleChatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func (h *Handler) compatibleChatCompletions(w http.ResponseWriter, r *http.Request) {
	token, ok := bearerToken(w, r)
	if !ok {
		return
	}
	var input compatibleChatCompletionRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	tools := make([]ToolDefinition, 0, len(input.Tools))
	for _, tool := range input.Tools {
		tools = append(tools, ToolDefinition{Name: tool.Name, Description: tool.Description, Schema: tool.Schema})
	}
	result, err := h.service.InvokeWithAccessToken(r.Context(), token, InvokeInput{
		ProviderType: ProviderOpenAI,
		Model:        input.Model,
		Messages:     input.Messages,
		Temperature:  input.Temperature,
		MaxTokens:    input.MaxTokens,
		Tools:        tools,
		Attribution:  Attribution{SourceSurface: "openai_compatible_api"},
	})
	if err != nil {
		writeJSON(w, statusFromError(err), map[string]any{"error": map[string]string{"message": err.Error(), "type": "ai_gateway_error"}})
		return
	}
	responseID := result.ProviderRequestID
	if responseID == "" {
		responseID = result.InvocationID.String()
	}
	writeJSON(w, http.StatusOK, compatibleChatCompletionResponse{
		ID:      responseID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   result.Model,
		Choices: []compatibleChatChoice{{
			Index:        0,
			Message:      Message{Role: "assistant", Content: result.Content},
			FinishReason: "stop",
		}},
		Usage: compatibleChatUsage{
			PromptTokens:     result.Usage.InputTokens,
			CompletionTokens: result.Usage.OutputTokens,
			TotalTokens:      result.Usage.InputTokens + result.Usage.OutputTokens,
		},
	})
}

func (h *Handler) compatibleListModels(w http.ResponseWriter, r *http.Request) {
	if _, ok := bearerToken(w, r); !ok {
		return
	}
	models, err := h.service.ListModels(r.Context(), nil, queryLimit(r))
	if err != nil {
		writeJSON(w, statusFromError(err), map[string]any{"error": map[string]string{"message": err.Error(), "type": "ai_gateway_error"}})
		return
	}
	data := make([]map[string]any, 0, len(models))
	for _, model := range models {
		data = append(data, map[string]any{
			"id":       model.ModelKey,
			"object":   "model",
			"owned_by": model.ProviderID.String(),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": data})
}

func bearerToken(w http.ResponseWriter, r *http.Request) (string, bool) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(strings.ToLower(header), "bearer ") {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]string{"message": "bearer token is required", "type": "authentication_error"}})
		return "", false
	}
	token := strings.TrimSpace(header[len("Bearer "):])
	if token == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]string{"message": "bearer token is required", "type": "authentication_error"}})
		return "", false
	}
	return token, true
}

func (h *Handler) compatibleUnsupportedOperation(operation string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := bearerToken(w, r); !ok {
			return
		}
		writeJSON(w, http.StatusNotImplemented, map[string]any{
			"error": map[string]string{
				"message": operation + " is not enabled for this AI gateway adapter",
				"type":    "unsupported_operation",
			},
		})
	}
}

func (h *Handler) listProviders(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListProviders(r.Context(), queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) updateProvider(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var input UpdateProviderInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.UpdateProvider(r.Context(), id, input)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) rotateProviderKey(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var input RotateProviderKeyInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.RotateProviderKey(r.Context(), id, input)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) testProvider(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var input TestProviderInput
	if !decodeJSON(w, r, &input) {
		return
	}
	err := h.service.TestProvider(r.Context(), id, input)
	writeResult(w, http.StatusOK, map[string]string{"status": "ok"}, err)
}

func (h *Handler) createChannel(w http.ResponseWriter, r *http.Request) {
	providerID, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var input CreateChannelInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.ProviderID = providerID
	result, err := h.service.CreateChannel(r.Context(), input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) listProviderChannels(w http.ResponseWriter, r *http.Request) {
	providerID, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	result, err := h.service.ListChannels(r.Context(), &providerID, queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) listChannels(w http.ResponseWriter, r *http.Request) {
	var providerID *uuid.UUID
	if raw := r.URL.Query().Get("provider_id"); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid provider_id"})
			return
		}
		providerID = &parsed
	}
	result, err := h.service.ListChannels(r.Context(), providerID, queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) updateChannel(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var input UpdateChannelInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.UpdateChannel(r.Context(), id, input)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) rotateChannelKey(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var input RotateChannelKeyInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.RotateChannelKey(r.Context(), id, input)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) testChannel(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var input TestChannelInput
	if !decodeJSON(w, r, &input) {
		return
	}
	err := h.service.TestChannel(r.Context(), id, input)
	writeResult(w, http.StatusOK, map[string]string{"status": "ok"}, err)
}

func (h *Handler) createModel(w http.ResponseWriter, r *http.Request) {
	var input CreateModelInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreateModel(r.Context(), input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) listModels(w http.ResponseWriter, r *http.Request) {
	var providerID *uuid.UUID
	if raw := r.URL.Query().Get("provider_id"); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid provider_id"})
			return
		}
		providerID = &parsed
	}
	result, err := h.service.ListModels(r.Context(), providerID, queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) updateModel(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var input UpdateModelInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.UpdateModel(r.Context(), id, input)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) invoke(w http.ResponseWriter, r *http.Request) {
	var input InvokeInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.Invoke(r.Context(), input)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) stream(w http.ResponseWriter, r *http.Request) {
	input, ok := streamInputFromQuery(w, r)
	if !ok {
		return
	}
	h.writeStream(w, r, input)
}

func (h *Handler) streamPost(w http.ResponseWriter, r *http.Request) {
	var input InvokeInput
	if !decodeJSON(w, r, &input) {
		return
	}
	h.writeStream(w, r, input)
}

func (h *Handler) writeStream(w http.ResponseWriter, r *http.Request, input InvokeInput) {
	result, err := h.service.Stream(r.Context(), input)
	if err != nil {
		writeJSON(w, statusFromError(err), map[string]string{"error": err.Error()})
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming unsupported"})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	writeSSE(w, "lifecycle", map[string]any{"invocation_id": result.InvocationID, "status": StatusStreaming})
	flusher.Flush()

	for event := range result.Events {
		eventName := event.Type
		if eventName == "" {
			eventName = "message"
		}
		writeSSE(w, eventName, map[string]any{
			"invocation_id": result.InvocationID,
			"delta":         event.Delta,
			"usage":         event.Usage,
			"tool_call":     event.ToolCall,
			"error":         event.Error,
			"done":          event.Done,
		})
		flusher.Flush()
	}
}

func (h *Handler) listRoutingRules(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListRoutingRules(r.Context(), queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) createRoutingRule(w http.ResponseWriter, r *http.Request) {
	var input CreateRoutingRuleInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreateRoutingRule(r.Context(), input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) createAccessToken(w http.ResponseWriter, r *http.Request) {
	var input CreateAccessTokenInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreateAccessToken(r.Context(), input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) listAccessTokens(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := optionalQueryUUID(w, r, "organization_id")
	if !ok {
		return
	}
	result, err := h.service.ListAccessTokens(r.Context(), organizationID, queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) createModelGroup(w http.ResponseWriter, r *http.Request) {
	var input CreateModelGroupInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreateModelGroup(r.Context(), input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) listModelGroups(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := optionalQueryUUID(w, r, "organization_id")
	if !ok {
		return
	}
	result, err := h.service.ListModelGroups(r.Context(), organizationID, queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) createModelChannelAbility(w http.ResponseWriter, r *http.Request) {
	var input CreateModelChannelAbilityInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreateModelChannelAbility(r.Context(), input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) listModelChannelAbilities(w http.ResponseWriter, r *http.Request) {
	modelGroupID, ok := optionalQueryUUID(w, r, "model_group_id")
	if !ok {
		return
	}
	result, err := h.service.ListModelChannelAbilities(r.Context(), modelGroupID, queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) getGatewayBalance(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := requiredQueryUUID(w, r, "organization_id")
	if !ok {
		return
	}
	result, err := h.service.GetGatewayBalance(r.Context(), organizationID, r.URL.Query().Get("currency"))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) adjustGatewayBalance(w http.ResponseWriter, r *http.Request) {
	var input AdjustGatewayBalanceInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.AdjustGatewayBalance(r.Context(), input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) listBalanceTransactions(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := requiredQueryUUID(w, r, "organization_id")
	if !ok {
		return
	}
	result, err := h.service.ListBalanceTransactions(r.Context(), organizationID, queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) adapterCatalog(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.service.AdapterCatalog())
}

func (h *Handler) usageAnalysis(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.UsageAnalysis(r.Context(), UsageAnalysisFilter{Limit: queryLimit(r)})
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) estimateCost(w http.ResponseWriter, r *http.Request) {
	var input EstimateCostInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.EstimateCost(r.Context(), input)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) listInvocations(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListInvocations(r.Context(), queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) getInvocation(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	result, err := h.service.GetInvocation(r.Context(), id)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) costSummary(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.CostSummary(r.Context())
	writeResult(w, http.StatusOK, result, err)
}

func streamInputFromQuery(w http.ResponseWriter, r *http.Request) (InvokeInput, bool) {
	input := InvokeInput{
		ProviderType: r.URL.Query().Get("provider_type"),
		Model:        r.URL.Query().Get("model"),
		Messages:     []Message{{Role: "user", Content: r.URL.Query().Get("message")}},
		Attribution: Attribution{
			SourceSurface: r.URL.Query().Get("source_surface"),
		},
	}
	if input.Messages[0].Content == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "message is required"})
		return input, false
	}
	if raw := r.URL.Query().Get("provider_id"); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid provider_id"})
			return input, false
		}
		input.ProviderID = &parsed
	}
	if raw := r.URL.Query().Get("model_id"); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid model_id"})
			return input, false
		}
		input.ModelID = &parsed
	}
	if raw := r.URL.Query().Get("preferred_channel_id"); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid preferred_channel_id"})
			return input, false
		}
		input.PreferredChannelID = &parsed
	}
	input.ServiceTier = r.URL.Query().Get("service_tier")
	input.ReasoningEffort = r.URL.Query().Get("reasoning_effort")
	if raw := r.URL.Query().Get("max_tokens"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid max_tokens"})
			return input, false
		}
		input.MaxTokens = parsed
	}
	return input, true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dest any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(dest); err != nil {
		if errors.Is(err, io.EOF) {
			return true
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return false
	}
	return true
}

func parseID(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return uuid.Nil, false
	}
	return id, true
}

func queryLimit(r *http.Request) int {
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	return limit
}

func optionalQueryUUID(w http.ResponseWriter, r *http.Request, name string) (*uuid.UUID, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return nil, true
	}
	parsed, err := uuid.Parse(raw)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid " + name})
		return nil, false
	}
	return &parsed, true
}

func requiredQueryUUID(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	parsed, ok := optionalQueryUUID(w, r, name)
	if !ok {
		return uuid.Nil, false
	}
	if parsed == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": name + " is required"})
		return uuid.Nil, false
	}
	return *parsed, true
}

func writeResult(w http.ResponseWriter, successStatus int, payload any, err error) {
	if err != nil {
		writeJSON(w, statusFromError(err), map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, successStatus, payload)
}

func statusFromError(err error) int {
	var providerErr *ProviderError
	switch {
	case errors.As(err, &providerErr):
		return http.StatusBadGateway
	case errors.Is(err, ErrValidation):
		return http.StatusBadRequest
	case errors.Is(err, ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, ErrUnavailable):
		return http.StatusServiceUnavailable
	case dberrors.IsUniqueViolation(err):
		return http.StatusConflict
	case errors.Is(err, ErrNotFound), errors.Is(err, pgx.ErrNoRows):
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("aigateway writeJSON error: %v", err)
	}
}

func writeSSE(w http.ResponseWriter, event string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		data = []byte(`{"error":"encode stream event failed"}`)
		event = "error"
	}
	if _, err := w.Write([]byte("event: " + event + "\n")); err != nil {
		log.Printf("aigateway stream write error: %v", err)
		return
	}
	if _, err := w.Write([]byte("data: " + string(data) + "\n\n")); err != nil {
		log.Printf("aigateway stream write error: %v", err)
	}
}
