package vector

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"my-rag/internal/store"
)

func TestStore_Search_topK(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "v.db"))
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
VALUES ('d1', 'f.txt', 'text/plain', 1, '/x', 'ready', NULL, ?, ?)`, now, now)
	if err != nil {
		t.Fatal(err)
	}

	vs := NewStore(db)
	// 2-D vectors: e1, e2, diagonal
	if err := vs.UpsertChunk(ctx, "c1", "d1", 0, "a", []float32{1, 0}); err != nil {
		t.Fatal(err)
	}
	if err := vs.UpsertChunk(ctx, "c2", "d1", 1, "b", []float32{0, 1}); err != nil {
		t.Fatal(err)
	}
	if err := vs.UpsertChunk(ctx, "c3", "d1", 2, "c", []float32{1, 1}); err != nil {
		t.Fatal(err)
	}

	hits, err := vs.Search(ctx, []float32{1, 0}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 {
		t.Fatalf("len=%d", len(hits))
	}
	if hits[0].ChunkID != "c1" {
		t.Fatalf("first=%v", hits[0])
	}
	// second should be c3 (cos ~0.707) not c2 (0)
	if hits[1].ChunkID != "c3" {
		t.Fatalf("second=%v", hits[1])
	}
}

func TestStore_DeleteByDocument(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "v2.db"))
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
VALUES ('d1', 'f.txt', 'text/plain', 1, '/x', 'ready', NULL, ?, ?)`, now, now)
	if err != nil {
		t.Fatal(err)
	}
	vs := NewStore(db)
	_ = vs.UpsertChunk(ctx, "c1", "d1", 0, "x", []float32{1, 0})
	if err := vs.DeleteByDocument(ctx, "d1"); err != nil {
		t.Fatal(err)
	}
	var n int
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM chunks`).Scan(&n)
	if n != 0 {
		t.Fatalf("chunks=%d", n)
	}
}
