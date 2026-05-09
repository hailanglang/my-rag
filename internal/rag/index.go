package rag

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"my-rag/internal/chunk"
	"my-rag/internal/embed"
	"my-rag/internal/parse"
	"my-rag/internal/vector"
)

// Indexer runs parse → chunk → embed → persist chunks for one document.
type Indexer struct {
	DB  *sql.DB
	Emb embed.Embedder
}

// NewIndexer builds an indexer. emb must return one vector per input text (same dimension each call).
func NewIndexer(db *sql.DB, emb embed.Embedder) *Indexer {
	return &Indexer{DB: db, Emb: emb}
}

// IndexDocument reads the file at documents.storage_path, extracts text, chunks, embeds, and replaces all chunks for docID.
func (ix *Indexer) IndexDocument(ctx context.Context, docID string) error {
	var storagePath string
	err := ix.DB.QueryRowContext(ctx, `SELECT storage_path FROM documents WHERE id = ?`, docID).Scan(&storagePath)
	if err == sql.ErrNoRows {
		return fmt.Errorf("rag: document %q not found", docID)
	}
	if err != nil {
		return err
	}

	now := time.Now().UnixMilli()
	if _, err := ix.DB.ExecContext(ctx, `UPDATE documents SET status = ?, error_message = NULL, updated_at = ? WHERE id = ?`, "processing", now, docID); err != nil {
		return err
	}

	text, err := parse.Extract(storagePath)
	if err != nil {
		_, _ = ix.DB.ExecContext(ctx, `UPDATE documents SET status = ?, error_message = ?, updated_at = ? WHERE id = ?`, "failed", err.Error(), time.Now().UnixMilli(), docID)
		return err
	}

	parts := chunk.Chunk(text)
	var vecs [][]float32
	if len(parts) > 0 {
		vecs, err = ix.Emb.Embed(ctx, parts)
		if err != nil {
			_, _ = ix.DB.ExecContext(ctx, `UPDATE documents SET status = ?, error_message = ?, updated_at = ? WHERE id = ?`, "failed", err.Error(), time.Now().UnixMilli(), docID)
			return err
		}
		if len(vecs) != len(parts) {
			msg := fmt.Sprintf("rag: expected %d embeddings, got %d", len(parts), len(vecs))
			_, _ = ix.DB.ExecContext(ctx, `UPDATE documents SET status = ?, error_message = ?, updated_at = ? WHERE id = ?`, "failed", msg, time.Now().UnixMilli(), docID)
			return fmt.Errorf("%s", msg)
		}
	}

	tx, err := ix.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := vector.ExecDeleteChunksByDocument(ctx, tx, docID); err != nil {
		return err
	}
	for i := range parts {
		chunkID := docID + ":" + strconv.Itoa(i)
		if err := vector.ExecUpsertChunk(ctx, tx, chunkID, docID, i, parts[i], vecs[i]); err != nil {
			return err
		}
	}

	if _, err := tx.ExecContext(ctx, `UPDATE documents SET status = ?, error_message = NULL, updated_at = ? WHERE id = ?`, "ready", time.Now().UnixMilli(), docID); err != nil {
		return err
	}
	return tx.Commit()
}
