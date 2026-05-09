package vector

import "errors"

var (
	errEmptyEmbedding = errors.New("vector: embedding must be non-empty")
	errEmptyQuery     = errors.New("vector: query must be non-empty")
)
