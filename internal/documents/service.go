package documents

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"my-rag/internal/embed"
	"my-rag/internal/rag"
)

// Service coordinates uploads, indexing, and deletion.
type Service struct {
	repo         *Repository
	uploadDir    string
	indexer      *rag.Indexer
	indexTimeout time.Duration
}

// NewService constructs a document service. indexTimeout should be positive (e.g. from config.IndexTimeout).
func NewService(db *sql.DB, uploadDir string, emb embed.Embedder, indexTimeout time.Duration) *Service {
	if indexTimeout <= 0 {
		indexTimeout = 5 * time.Minute
	}
	return &Service{
		repo:         NewRepository(db),
		uploadDir:    uploadDir,
		indexer:      rag.NewIndexer(db, emb),
		indexTimeout: indexTimeout,
	}
}

// Upload saves multipart "file" into uploadDir and inserts a pending document, then indexes asynchronously.
func (s *Service) Upload(ctx context.Context, r *http.Request) (*Document, error) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		return nil, err
	}
	fh, header, err := r.FormFile("file")
	if err != nil {
		return nil, err
	}
	defer fh.Close()

	if err := os.MkdirAll(s.uploadDir, 0o755); err != nil {
		return nil, err
	}

	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = ".bin"
	}
	id := uuid.New().String()
	dest := filepath.Join(s.uploadDir, id+ext)

	dst, err := os.Create(dest)
	if err != nil {
		return nil, err
	}
	n, err := io.Copy(dst, fh)
	if closeErr := dst.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(dest)
		return nil, err
	}

	mime := header.Header.Get("Content-Type")
	if mime == "" {
		mime = "application/octet-stream"
	}

	now := time.Now().UnixMilli()
	doc := Document{
		ID: id, Filename: header.Filename, Mime: mime, Size: n,
		StoragePath: dest, Status: "pending", CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repo.Insert(ctx, doc); err != nil {
		_ = os.Remove(dest)
		return nil, err
	}

	go s.indexAsync(id)
	return &doc, nil
}

func (s *Service) indexAsync(docID string) {
	ctx, cancel := context.WithTimeout(context.Background(), s.indexTimeout)
	defer cancel()
	_ = s.indexer.IndexDocument(ctx, docID)
}

// List returns all documents (newest first).
func (s *Service) List(ctx context.Context) ([]Document, error) {
	return s.repo.List(ctx)
}

// Delete removes DB row (and chunks via FK) then deletes the file on disk.
func (s *Service) Delete(ctx context.Context, id string) error {
	path, err := s.repo.GetStoragePath(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("documents: remove file: %w", err)
	}
	return nil
}
