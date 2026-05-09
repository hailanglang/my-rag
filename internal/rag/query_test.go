package rag

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"my-rag/internal/store"
	"my-rag/internal/vector"
)

// steerEmb returns the same vector for every input text (used to steer search).
type steerEmb struct {
	vec []float32
}

func (s steerEmb) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = append([]float32(nil), s.vec...)
	}
	return out, nil
}

func TestBuildContext_twoChunks_citationsAndBullets(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "q.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UnixMilli()
	_, err = db.ExecContext(ctx, `
INSERT INTO documents (id, filename, mime, size, storage_path, status, error_message, created_at, updated_at)
VALUES ('d1', 'alpha.txt', 'text/plain', 1, '/x', 'ready', NULL, ?, ?)`, now, now)
	if err != nil {
		t.Fatal(err)
	}

	vs := vector.NewStore(db)
	if err := vs.UpsertChunk(ctx, "c1", "d1", 0, "first chunk text", []float32{1, 0}); err != nil {
		t.Fatal(err)
	}
	if err := vs.UpsertChunk(ctx, "c2", "d1", 1, "second chunk text", []float32{0, 1}); err != nil {
		t.Fatal(err)
	}

	q := NewQuerier(db, steerEmb{vec: []float32{1, 0}}).WithTopK(2)
	block, cites, err := q.BuildContext(ctx, "user question")
	if err != nil {
		t.Fatal(err)
	}
	if len(cites) != 2 {
		t.Fatalf("citations=%d", len(cites))
	}
	if cites[0].ChunkID != "c1" || cites[0].DocumentName != "alpha.txt" {
		t.Fatalf("first cite=%+v", cites[0])
	}
	if cites[0].Quote != "first chunk text" {
		t.Fatalf("quote=%q", cites[0].Quote)
	}
	if cites[1].ChunkID != "c2" {
		t.Fatalf("second cite=%+v", cites[1])
	}
	if !strings.Contains(block, "alpha.txt") || !strings.Contains(block, "first chunk text") {
		t.Fatalf("block=%q", block)
	}
}

func TestBuildContext_emptyQuery(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "q2.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	q := NewQuerier(db, steerEmb{vec: []float32{1, 0}})
	_, _, err = q.BuildContext(ctx, "   ")
	if err == nil {
		t.Fatal("expected error")
	}
}
