package procurement

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/procurement/requisitions", h.createRequisition)
	r.Get("/procurement/requisitions", h.listRequisitions)
	r.Post("/procurement/requisitions/{id}/submit", h.submitRequisition)
	r.Post("/procurement/requisitions/{id}/approve", h.approveRequisition)
	r.Post("/procurement/orders", h.createOrder)
	r.Get("/procurement/orders", h.listOrders)
	r.Post("/procurement/orders/{id}/submit", h.submitOrder)
	r.Post("/procurement/orders/{id}/approve", h.approveOrder)
	r.Post("/procurement/receipts", h.createReceipt)
	r.Get("/procurement/receipts", h.listReceipts)
	r.Post("/procurement/receipts/{id}/post", h.postReceipt)
	r.Post("/procurement/returns", h.createReturn)
	r.Get("/procurement/returns", h.listReturns)
}

func (h *Handler) createRequisition(w http.ResponseWriter, r *http.Request) {
	var input CreatePurchaseRequisitionInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreateRequisition(r.Context(), input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) listRequisitions(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListRequisitions(r.Context(), queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) submitRequisition(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	result, err := h.service.SubmitRequisition(r.Context(), id)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) approveRequisition(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	result, err := h.service.ApproveRequisition(r.Context(), id)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) createOrder(w http.ResponseWriter, r *http.Request) {
	var input CreatePurchaseOrderInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreateOrder(r.Context(), input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) listOrders(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListOrders(r.Context(), queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) submitOrder(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	result, err := h.service.SubmitOrder(r.Context(), id)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) approveOrder(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	result, err := h.service.ApproveOrder(r.Context(), id)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) createReceipt(w http.ResponseWriter, r *http.Request) {
	var input CreatePurchaseReceiptInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreateReceipt(r.Context(), input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) listReceipts(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListReceipts(r.Context(), queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) postReceipt(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	result, err := h.service.PostReceipt(r.Context(), id)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) createReturn(w http.ResponseWriter, r *http.Request) {
	var input CreatePurchaseReturnInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreateReturn(r.Context(), input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) listReturns(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListReturns(r.Context(), queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(target); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return false
	}
	return true
}

func parseID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return uuid.Nil, false
	}
	return id, true
}

func queryLimit(r *http.Request) int {
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			return parsed
		}
	}
	return 100
}

func writeResult(w http.ResponseWriter, status int, payload any, err error) {
	if err != nil {
		code := http.StatusInternalServerError
		if errors.Is(err, ErrValidation) {
			code = http.StatusBadRequest
		} else if errors.Is(err, ErrNotFound) {
			code = http.StatusNotFound
		}
		writeJSON(w, code, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, status, payload)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
