package inventory

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/inventory/partners", h.createPartner)
	r.Get("/inventory/partners", h.listPartners)
	r.Post("/inventory/items", h.createItem)
	r.Get("/inventory/items", h.listItems)
	r.Post("/inventory/warehouses", h.createWarehouse)
	r.Get("/inventory/warehouses", h.listWarehouses)
	r.Post("/inventory/locations", h.createLocation)
	r.Get("/inventory/balances", h.listBalances)
	r.Post("/inventory/movements", h.createMovement)
	r.Get("/inventory/movements", h.listMovements)
	r.Post("/inventory/transfers", h.createTransfer)
	r.Get("/inventory/transfers", h.listTransfers)
	r.Post("/inventory/adjustments", h.createAdjustment)
	r.Get("/inventory/adjustments", h.listAdjustments)
	r.Post("/inventory/counts", h.createCount)
	r.Get("/inventory/counts", h.listCounts)
}

func (h *Handler) createPartner(w http.ResponseWriter, r *http.Request) {
	var input CreateBusinessPartnerInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreateBusinessPartner(r.Context(), input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) listPartners(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListBusinessPartners(r.Context(), queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) createItem(w http.ResponseWriter, r *http.Request) {
	var input CreateItemInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreateItem(r.Context(), input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) listItems(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListItems(r.Context(), queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) createWarehouse(w http.ResponseWriter, r *http.Request) {
	var input CreateWarehouseInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreateWarehouse(r.Context(), input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) listWarehouses(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListWarehouses(r.Context(), queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) createLocation(w http.ResponseWriter, r *http.Request) {
	var input CreateWarehouseLocationInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreateWarehouseLocation(r.Context(), input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) listBalances(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListBalances(r.Context(), queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) createMovement(w http.ResponseWriter, r *http.Request) {
	var input CreateInventoryMovementInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.PostMovement(r.Context(), input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) listMovements(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListMovements(r.Context(), queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) createTransfer(w http.ResponseWriter, r *http.Request) {
	var input CreateInventoryTransferInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreateTransfer(r.Context(), input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) listTransfers(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListTransfers(r.Context(), queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) createAdjustment(w http.ResponseWriter, r *http.Request) {
	var input CreateInventoryAdjustmentInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreateAdjustment(r.Context(), input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) listAdjustments(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListAdjustments(r.Context(), queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) createCount(w http.ResponseWriter, r *http.Request) {
	var input CreateInventoryCountInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreateCount(r.Context(), input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) listCounts(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListCounts(r.Context(), queryLimit(r))
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
		} else if errors.Is(err, ErrInsufficientStock) {
			code = http.StatusConflict
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
