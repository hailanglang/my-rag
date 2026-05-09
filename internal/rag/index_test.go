package rag

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"my-rag/internal/store"
	"my-rag/internal/vector"
)

// fakeEmb returns 2-D vectors with first coordinate (i+1) so Search([1,0]) prefers chunk 0.
type fakeEmb struct{}

func (fakeEmb) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{float32(i + 1), 0}
	}
	return out, nil
}

func TestIndexer_IndexDocument_andSearch(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}

	docPath := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(docPath, []byte("hello world"), 0o600); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UnixMilli()
	_, err = db.ExecContext(ctx, `
INSERT INTO documents (id, filename, mime, size, storage_path, status, error_message, created_at, updated_at)
VALUES ('d1', 'hello.txt', 'text/plain', 11, ?, 'pending', NULL, ?, ?)`, docPath, now, now)
	if err != nil {
		t.Fatal(err)
	}

	ix := NewIndexer(db, fakeEmb{})
	if err := ix.IndexDocument(ctx, "d1"); err != nil {
		t.Fatal(err)
	}

	var st string
	if err := db.QueryRowContext(ctx, `SELECT status FROM documents WHERE id='d1'`).Scan(&st); err != nil {
		t.Fatal(err)
	}
	if st != "ready" {
		t.Fatalf("status=%q", st)
	}

	vs := vector.NewStore(db)
	hits, err := vs.Search(ctx, []float32{1, 0}, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) < 1 {
		t.Fatal("expected search hits")
	}
	if hits[0].DocumentID != "d1" {
		t.Fatalf("hit=%+v", hits[0])
	}
	if hits[0].Text != "hello world" {
		t.Fatalf("text=%q", hits[0].Text)
	}
}

func TestIndexer_IndexDocument_extractErrorMarksFailed(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "b.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}

	missing := filepath.Join(dir, "nope.txt")
	now := time.Now().UnixMilli()
	_, err = db.ExecContext(ctx, `
INSERT INTO documents (id, filename, mime, size, storage_path, status, error_message, created_at, updated_at)
VALUES ('d1', 'nope.txt', 'text/plain', 0, ?, 'pending', NULL, ?, ?)`, missing, now, now)
	if err != nil {
		t.Fatal(err)
	}

	ix := NewIndexer(db, fakeEmb{})
	if err := ix.IndexDocument(ctx, "d1"); err == nil {
		t.Fatal("expected error")
	}

	var st, msg string
	_ = db.QueryRowContext(ctx, `SELECT status, error_message FROM documents WHERE id='d1'`).Scan(&st, &msg)
	if st != "failed" || msg == "" {
		t.Fatalf("status=%q msg=%q", st, msg)
	}
}
