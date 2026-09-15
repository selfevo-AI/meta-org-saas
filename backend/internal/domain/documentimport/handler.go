package documentimport

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/erp"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/document-imports", h.upload)
	r.Get("/document-imports", h.list)
	r.Get("/document-imports/{id}", h.get)
	r.Get("/document-imports/{id}/files/{file}", h.file)
	r.Post("/document-imports/{id}/recognize", h.recognize)
	r.Patch("/document-imports/{id}/review", h.review)
	r.Post("/document-imports/{id}/confirm", h.confirm)
	r.Post("/document-imports/{id}/reject", h.reject)
}

func (h *Handler) upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, MaxTotalBytes+64*1024)
	defer r.Body.Close()
	reader, err := r.MultipartReader()
	if err != nil {
		respond(w, nil, issue("files", "invalid_upload"))
		return
	}
	files := []SourceFile{}
	objectType := ""
	partCount, total := 0, 0
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		partCount++
		if err != nil || partCount > MaxFiles+1 {
			respond(w, nil, issue("files", "file_count"))
			return
		}
		if part.FormName() == "object_type" && part.FileName() == "" && objectType == "" {
			value, err := io.ReadAll(io.LimitReader(part, 129))
			if err != nil || len(value) > 128 {
				respond(w, nil, issue("object_type", "invalid"))
				return
			}
			objectType = string(value)
		} else if part.FormName() == "files" && part.FileName() != "" {
			content, err := io.ReadAll(io.LimitReader(part, MaxFileBytes+1))
			total += len(content)
			if err != nil || len(content) > MaxFileBytes || total > MaxTotalBytes {
				respond(w, nil, issue("files", "file_size"))
				return
			}
			files = append(files, SourceFile{Name: part.FileName(), Content: content})
		} else {
			respond(w, nil, issue("files", "invalid_upload"))
			return
		}
		_ = part.Close()
	}
	result, err := h.service.Upload(r.Context(), objectType, files)
	respond(w, result, err)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.List(r.Context(), r.URL.Query().Get("object_type"), r.URL.Query().Get("status"), r.URL.Query().Get("cursor"))
	respond(w, result, err)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	result, err := h.service.Get(r.Context(), id)
	respond(w, result, err)
}

func (h *Handler) file(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	fileID, ok := parseID(w, r, "file")
	if !ok {
		return
	}
	file, err := h.service.File(r.Context(), id, fileID)
	if err != nil {
		respond(w, nil, err)
		return
	}
	w.Header().Set("Content-Type", file.MediaType)
	w.Header().Set("Content-Length", strconv.Itoa(len(file.Content)))
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": file.Name}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	http.ServeContent(w, r, file.Name, time.Time{}, bytes.NewReader(file.Content))
}

func (h *Handler) recognize(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var input VersionInput
	if !decode(w, r, &input) {
		return
	}
	result, err := h.service.Recognize(r.Context(), id, input.Version)
	respond(w, result, err)
}

func (h *Handler) review(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var input ReviewInput
	if !decode(w, r, &input) {
		return
	}
	result, err := h.service.Review(r.Context(), id, input)
	respond(w, result, err)
}

func (h *Handler) confirm(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var input ReviewInput
	if !decode(w, r, &input) {
		return
	}
	result, err := h.service.Confirm(r.Context(), id, input)
	respond(w, result, err)
}

func (h *Handler) reject(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var input VersionInput
	if !decode(w, r, &input) {
		return
	}
	result, err := h.service.Reject(r.Context(), id, input.Version)
	respond(w, result, err)
}

func parseID(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		respond(w, nil, issue(name, "invalid"))
		return uuid.Nil, false
	}
	return id, true
}

func decode(w http.ResponseWriter, r *http.Request, input any) bool {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024*1024))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if decoder.Decode(input) != nil || decoder.Decode(new(any)) != io.EOF {
		respond(w, nil, issue("body", "invalid"))
		return false
	}
	return true
}

func respond(w http.ResponseWriter, value any, err error) {
	status := http.StatusOK
	if err != nil {
		code := "operation_failed"
		switch {
		case errors.Is(err, erp.ErrValidation):
			status, code = http.StatusBadRequest, "validation"
		case errors.Is(err, erp.ErrForbidden):
			status, code = http.StatusForbidden, "forbidden"
		case errors.Is(err, erp.ErrNotFound):
			status, code = http.StatusNotFound, "not_found"
		case errors.Is(err, erp.ErrConflict):
			status, code = http.StatusConflict, "conflict"
		default:
			status = http.StatusInternalServerError
		}
		body := map[string]any{"error": "Document import " + code, "code": "document_import." + code}
		var validation *ValidationError
		if errors.As(err, &validation) {
			body["issues"] = validation.Issues
		}
		value = body
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
