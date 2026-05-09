package session

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"my-rag/internal/store"
)

func TestPurgeOlderThan_removesOldMessagesAndCitations(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "p.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}

	repo := NewRepository(db)
	sid, err := repo.CreateSession(ctx, "s")
	if err != nil {
		t.Fatal(err)
	}

	old := time.Now().Add(-31 * 24 * time.Hour).UnixMilli()
	recent := time.Now().Add(-time.Hour).UnixMilli()

	_, err = db.ExecContext(ctx, `
INSERT INTO documents (id, filename, mime, size, storage_path, status, error_message, created_at, updated_at)
VALUES ('d1', 'f', 'text/plain', 1, '/x', 'ready', NULL, ?, ?)`, old, old)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `
INSERT INTO chunks (id, document_id, chunk_index, text, embedding, created_at) VALUES ('c1', 'd1', 0, 't', ?, ?)`, []byte{0, 0, 0, 0}, old)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.ExecContext(ctx, `
INSERT INTO messages (id, session_id, role, content, created_at) VALUES ('m-old', ?, 'user', 'x', ?)`, sid, old)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `
INSERT INTO message_citations (id, message_id, chunk_id, quote, document_id, created_at)
VALUES ('cit1', 'm-old', 'c1', 'q', 'd1', ?)`, old)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `
INSERT INTO messages (id, session_id, role, content, created_at) VALUES ('m-new', ?, 'user', 'y', ?)`, sid, recent)
	if err != nil {
		t.Fatal(err)
	}

	if err := PurgeOlderThan(ctx, db, 30*24*time.Hour); err != nil {
		t.Fatal(err)
	}

	var nMsg, nCit int
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM messages`).Scan(&nMsg)
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM message_citations`).Scan(&nCit)
	if nMsg != 1 || nCit != 0 {
		t.Fatalf("messages=%d citations=%d", nMsg, nCit)
	}

	var content string
	if err := db.QueryRowContext(ctx, `SELECT content FROM messages WHERE id='m-new'`).Scan(&content); err != nil {
		t.Fatal(err)
	}
	if content != "y" {
		t.Fatalf("content=%q", content)
	}

	var nSess int
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions WHERE id = ?`, sid).Scan(&nSess)
	if nSess != 1 {
		t.Fatalf("session should remain with recent message, n=%d", nSess)
	}
}

func TestPurgeOlderThan_deletesEmptySession(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "p2.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}

	old := time.Now().Add(-40 * 24 * time.Hour).UnixMilli()
	_, err = db.ExecContext(ctx, `
INSERT INTO sessions (id, title, created_at, updated_at) VALUES ('s-old', 't', ?, ?)`, old, old)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `
INSERT INTO messages (id, session_id, role, content, created_at) VALUES ('m1', 's-old', 'user', 'x', ?)`, old)
	if err != nil {
		t.Fatal(err)
	}

	if err := PurgeOlderThan(ctx, db, 30*24*time.Hour); err != nil {
		t.Fatal(err)
	}

	var nSess int
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions WHERE id='s-old'`).Scan(&nSess)
	if nSess != 0 {
		t.Fatalf("session still exists: %d", nSess)
	}
}
