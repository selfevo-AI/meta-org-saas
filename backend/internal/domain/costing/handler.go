package costing

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/dberrors"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/costing/currencies", h.listCurrencies)
	r.Post("/costing/currencies", h.upsertCurrency)
	r.Patch("/costing/currencies/{code}", h.upsertCurrency)
	r.Post("/costing/currencies/{code}/void", h.voidCurrency)
	r.Get("/costing/exchange-rates", h.listExchangeRates)
	r.Post("/costing/exchange-rates", h.createExchangeRate)
	r.Patch("/costing/exchange-rates/{id}", h.updateExchangeRate)
	r.Post("/costing/exchange-rates/{id}/void", h.voidExchangeRate)
	r.Post("/costing/convert", h.convert)
	r.Get("/costing/rate-cards", h.listRateCards)
	r.Post("/costing/rate-cards", h.createRateCard)
	r.Patch("/costing/rate-cards/{id}", h.updateRateCard)
	r.Post("/costing/rate-cards/{id}/void", h.voidRateCard)
	r.Get("/costing/ledger-entries", h.listLedgerEntries)
	r.Post("/costing/ledger-entries", h.createLedgerEntry)
	r.Patch("/costing/ledger-entries/{id}", h.updateLedgerEntry)
	r.Post("/costing/ledger-entries/{id}/void", h.voidLedgerEntry)
	r.Get("/costing/summary", h.summary)
	r.Get("/costing/budgets", h.listBudgets)
	r.Post("/costing/budgets", h.createBudget)
	r.Patch("/costing/budgets/{id}", h.updateBudget)
	r.Post("/costing/budgets/{id}/void", h.voidBudget)
}

func (h *Handler) listCurrencies(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListCurrencies(r.Context())
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) upsertCurrency(w http.ResponseWriter, r *http.Request) {
	var input CreateCurrencyInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if code := chi.URLParam(r, "code"); code != "" {
		input.Code = code
	}
	result, err := h.service.UpsertCurrency(r.Context(), input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) voidCurrency(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.VoidCurrency(r.Context(), chi.URLParam(r, "code"))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) listExchangeRates(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListExchangeRates(r.Context(), queryInt(r, "limit", 50))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) createExchangeRate(w http.ResponseWriter, r *http.Request) {
	var input CreateExchangeRateInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreateExchangeRate(r.Context(), input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) updateExchangeRate(w http.ResponseWriter, r *http.Request) {
	var input UpdateExchangeRateInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.UpdateExchangeRate(r.Context(), chi.URLParam(r, "id"), input)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) voidExchangeRate(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.VoidExchangeRate(r.Context(), chi.URLParam(r, "id"))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) convert(w http.ResponseWriter, r *http.Request) {
	var input ConvertInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.Convert(r.Context(), input)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) listRateCards(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListRateCards(r.Context(), queryInt(r, "limit", 50))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) createRateCard(w http.ResponseWriter, r *http.Request) {
	var input CreateRateCardInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreateRateCard(r.Context(), input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) updateRateCard(w http.ResponseWriter, r *http.Request) {
	var input UpdateRateCardInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.UpdateRateCard(r.Context(), chi.URLParam(r, "id"), input)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) voidRateCard(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.VoidRateCard(r.Context(), chi.URLParam(r, "id"))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) listLedgerEntries(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListLedgerEntries(r.Context(), summaryFilter(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) createLedgerEntry(w http.ResponseWriter, r *http.Request) {
	var input CreateLedgerEntryInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreateLedgerEntry(r.Context(), input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) updateLedgerEntry(w http.ResponseWriter, r *http.Request) {
	var input UpdateLedgerEntryInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.UpdateLedgerEntry(r.Context(), chi.URLParam(r, "id"), input)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) voidLedgerEntry(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.VoidLedgerEntry(r.Context(), chi.URLParam(r, "id"))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) summary(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.Summary(r.Context(), summaryFilter(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) listBudgets(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListBudgets(r.Context(), queryInt(r, "limit", 50))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) createBudget(w http.ResponseWriter, r *http.Request) {
	var input CreateBudgetInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreateBudget(r.Context(), input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) updateBudget(w http.ResponseWriter, r *http.Request) {
	var input UpdateBudgetInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.UpdateBudget(r.Context(), chi.URLParam(r, "id"), input)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) voidBudget(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.VoidBudget(r.Context(), chi.URLParam(r, "id"))
	writeResult(w, http.StatusOK, result, err)
}

func summaryFilter(r *http.Request) SummaryFilter {
	var scopeID *uuid.UUID
	if raw := r.URL.Query().Get("scope_id"); raw != "" {
		if id, err := uuid.Parse(raw); err == nil {
			scopeID = &id
		}
	}
	return SummaryFilter{
		ScopeType: r.URL.Query().Get("scope_type"),
		ScopeID:   scopeID,
		Currency:  r.URL.Query().Get("currency"),
		Limit:     queryInt(r, "limit", 50),
	}
}

func queryInt(r *http.Request, key string, fallback int) int {
	value, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil {
		return fallback
	}
	return value
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return false
	}
	return true
}

func writeResult(w http.ResponseWriter, status int, payload any, err error) {
	if err != nil {
		code := http.StatusInternalServerError
		if errors.Is(err, ErrValidation) {
			code = http.StatusBadRequest
		} else if dberrors.IsUniqueViolation(err) {
			code = http.StatusConflict
		} else if errors.Is(err, ErrNotFound) {
			code = http.StatusNotFound
		}
		writeError(w, code, err.Error())
		return
	}
	writeJSON(w, status, payload)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
