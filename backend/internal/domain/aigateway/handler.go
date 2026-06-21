package aigateway

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/selfevo-AI/meta-org/backend/internal/pkg/dberrors"
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

func (h *Handler) createProvider(w http.ResponseWriter, r *http.Request) {
	var input CreateProviderInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreateProvider(r.Context(), input)
	writeResult(w, http.StatusCreated, result, err)
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
