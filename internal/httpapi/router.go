package httpapi

import (
	"database/sql"
	"net/http"
	"time"

	"my-rag/internal/documents"
	"my-rag/internal/embed"
)

// RouterConfig wires document APIs when DB, UploadDir, and Embedder are set.
type RouterConfig struct {
	DB           *sql.DB
	UploadDir    string
	Embedder     embed.Embedder
	IndexTimeout time.Duration
}

// NewRouter returns the HTTP handler. If cfg is nil or incomplete, only /api/health is served.
func NewRouter(cfg *RouterConfig) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	if cfg != nil && cfg.DB != nil && cfg.Embedder != nil && cfg.UploadDir != "" {
		svc := documents.NewService(cfg.DB, cfg.UploadDir, cfg.Embedder, cfg.IndexTimeout)
		dh := &docHandlers{svc: svc}
		mux.HandleFunc("GET /api/documents", dh.handleList)
		mux.HandleFunc("POST /api/documents", dh.handleUpload)
		mux.HandleFunc("DELETE /api/documents/{id}", dh.handleDelete)
	}
	return mux
}
