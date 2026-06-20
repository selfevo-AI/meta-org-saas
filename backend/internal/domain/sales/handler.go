package sales

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
	r.Post("/sales/quotations", h.createQuotation)
	r.Get("/sales/quotations", h.listQuotations)
	r.Post("/sales/orders", h.createOrder)
	r.Get("/sales/orders", h.listOrders)
	r.Post("/sales/orders/{id}/confirm", h.confirmOrder)
	r.Post("/sales/orders/{id}/approve", h.approveOrder)
	r.Post("/sales/shipments", h.createShipment)
	r.Get("/sales/shipments", h.listShipments)
	r.Post("/sales/shipments/{id}/post", h.postShipment)
	r.Post("/sales/returns", h.createReturn)
	r.Get("/sales/returns", h.listReturns)
}

func (h *Handler) createQuotation(w http.ResponseWriter, r *http.Request) {
	var input CreateSalesQuotationInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreateQuotation(r.Context(), input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) listQuotations(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListQuotations(r.Context(), queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) createOrder(w http.ResponseWriter, r *http.Request) {
	var input CreateSalesOrderInput
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

func (h *Handler) confirmOrder(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	result, err := h.service.ConfirmOrder(r.Context(), id)
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

func (h *Handler) createShipment(w http.ResponseWriter, r *http.Request) {
	var input CreateSalesShipmentInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreateShipment(r.Context(), input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) listShipments(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListShipments(r.Context(), queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) postShipment(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	result, err := h.service.PostShipment(r.Context(), id)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) createReturn(w http.ResponseWriter, r *http.Request) {
	var input CreateSalesReturnInput
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
