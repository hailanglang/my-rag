package rag

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"my-rag/internal/embed"
	"my-rag/internal/vector"
)

// Citation is one retrieved chunk used for grounding and UI.
type Citation struct {
	ChunkID      string `json:"chunk_id"`
	DocumentID   string `json:"document_id"`
	DocumentName string `json:"document_name"`
	Quote        string `json:"quote"`
}

// Querier builds retrieval context from the vector store (no LLM calls).
type Querier struct {
	db   *sql.DB
	vs   *vector.Store
	emb  embed.Embedder
	topK int
}

// NewQuerier returns a querier with default TopK=5.
func NewQuerier(db *sql.DB, emb embed.Embedder) *Querier {
	return &Querier{
		db:   db,
		vs:   vector.NewStore(db),
		emb:  emb,
		topK: 5,
	}
}

// WithTopK overrides the number of chunks to retrieve (must be > 0).
func (q *Querier) WithTopK(k int) *Querier {
	if k > 0 {
		q.topK = k
	}
	return q
}

// BuildContext embeds the user query, runs vector search, and returns a bullet
// context block plus structured citations (document title from DB).
func (q *Querier) BuildContext(ctx context.Context, userQuery string) (contextBlock string, citations []Citation, err error) {
	query := strings.TrimSpace(userQuery)
	if query == "" {
		return "", nil, fmt.Errorf("rag: empty query")
	}
	vecs, err := q.emb.Embed(ctx, []string{query})
	if err != nil {
		return "", nil, err
	}
	if len(vecs) != 1 || len(vecs[0]) == 0 {
		return "", nil, fmt.Errorf("rag: expected one non-empty query embedding")
	}
	hits, err := q.vs.Search(ctx, vecs[0], q.topK)
	if err != nil {
		return "", nil, err
	}
	if len(hits) == 0 {
		return "", []Citation{}, nil
	}

	docIDs := uniqueDocIDs(hits)
	names, err := q.documentFilenames(ctx, docIDs)
	if err != nil {
		return "", nil, err
	}

	citations = make([]Citation, 0, len(hits))
	var b strings.Builder
	for _, h := range hits {
		name := names[h.DocumentID]
		if name == "" {
			name = h.DocumentID
		}
		quote := h.Text
		citations = append(citations, Citation{
			ChunkID:      h.ChunkID,
			DocumentID: h.DocumentID,
			DocumentName: name,
			Quote:        quote,
		})
		b.WriteString("- ")
		b.WriteString(fmt.Sprintf("[%s] ", name))
		b.WriteString(strings.ReplaceAll(strings.TrimSpace(quote), "\n", " "))
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n"), citations, nil
}

func uniqueDocIDs(hits []vector.Hit) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, h := range hits {
		if _, ok := seen[h.DocumentID]; ok {
			continue
		}
		seen[h.DocumentID] = struct{}{}
		out = append(out, h.DocumentID)
	}
	return out
}

func (q *Querier) documentFilenames(ctx context.Context, docIDs []string) (map[string]string, error) {
	out := make(map[string]string, len(docIDs))
	for _, id := range docIDs {
		var fn string
		err := q.db.QueryRowContext(ctx, `SELECT filename FROM documents WHERE id = ?`, id).Scan(&fn)
		if err == sql.ErrNoRows {
			out[id] = ""
			continue
		}
		if err != nil {
			return nil, err
		}
		out[id] = fn
	}
	return out, nil
}
