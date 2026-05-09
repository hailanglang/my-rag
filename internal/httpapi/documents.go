package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"my-rag/internal/documents"
)

type docHandlers struct {
	svc *documents.Service
}

func (h *docHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	docs, err := h.svc.List(r.Context())
	if err != nil {
		writeAPIError(w, "LIST_FAILED", err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, docs)
}

func (h *docHandlers) handleUpload(w http.ResponseWriter, r *http.Request) {
	doc, err := h.svc.Upload(r.Context(), r)
	if err != nil {
		writeAPIError(w, "UPLOAD_FAILED", err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusCreated, doc)
}

func (h *docHandlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeAPIError(w, "BAD_REQUEST", "missing id", http.StatusBadRequest)
		return
	}
	err := h.svc.Delete(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, "NOT_FOUND", "document not found", http.StatusNotFound)
		return
	}
	if err != nil {
		writeAPIError(w, "DELETE_FAILED", err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeAPIError(w http.ResponseWriter, code, message string, status int) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}
