package embed

import "context"

// Embedder turns text chunks into dense vectors (same dim per model).
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}
