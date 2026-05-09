package vector

import (
	"context"
	"database/sql"
	"sort"
	"time"
)

// Hit is one retrieved chunk with similarity score.
type Hit struct {
	ChunkID    string
	DocumentID string
	ChunkIndex int
	Text       string
	Score      float64
}

// Store reads and writes chunk embeddings in SQLite (table chunks).
type Store struct {
	db *sql.DB
}

// NewStore wraps a migrated database (see store.Migrate).
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// UpsertChunk inserts or replaces a chunk row with an embedding blob (float32 little-endian).
func (s *Store) UpsertChunk(ctx context.Context, chunkID, documentID string, chunkIndex int, text string, embedding []float32) error {
	if len(embedding) == 0 {
		return errEmptyEmbedding
	}
	blob := float32SliceToBlob(embedding)
	now := time.Now().UnixMilli()
	_, err := s.db.ExecContext(ctx, `
INSERT OR REPLACE INTO chunks (id, document_id, chunk_index, text, embedding, created_at)
VALUES (?, ?, ?, ?, ?, ?)`,
		chunkID, documentID, chunkIndex, text, blob, now)
	return err
}

// DeleteByDocument removes all chunks for a document.
func (s *Store) DeleteByDocument(ctx context.Context, documentID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM chunks WHERE document_id = ?`, documentID)
	return err
}

// Search returns up to k chunks with highest cosine similarity to query (same dimension as stored vectors).
func (s *Store) Search(ctx context.Context, query []float32, k int) ([]Hit, error) {
	if k <= 0 {
		return nil, nil
	}
	if len(query) == 0 {
		return nil, errEmptyQuery
	}
	wantBytes := len(query) * 4
	rows, err := s.db.QueryContext(ctx, `
SELECT id, document_id, chunk_index, text, embedding
FROM chunks
WHERE embedding IS NOT NULL AND length(embedding) = ?`, wantBytes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type scored struct {
		hit Hit
	}
	var all []scored
	for rows.Next() {
		var id, docID, text string
		var idx int
		var blob []byte
		if err := rows.Scan(&id, &docID, &idx, &text, &blob); err != nil {
			return nil, err
		}
		vec, err := blobToFloat32Slice(blob)
		if err != nil {
			return nil, err
		}
		sc := Cosine(query, vec)
		all = append(all, scored{hit: Hit{
			ChunkID: id, DocumentID: docID, ChunkIndex: idx, Text: text, Score: sc,
		}})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].hit.Score != all[j].hit.Score {
			return all[i].hit.Score > all[j].hit.Score
		}
		return all[i].hit.ChunkID < all[j].hit.ChunkID
	})
	if len(all) > k {
		all = all[:k]
	}
	out := make([]Hit, len(all))
	for i := range all {
		out[i] = all[i].hit
	}
	return out, nil
}
