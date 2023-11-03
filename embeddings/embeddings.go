package embeddings

import "context"

// Embedder is the embeddings interface
type Embedder interface {
	EmbedChunks(ctx context.Context, chunks []string, options ...Option) ([][]float64, error)
	EmbedQuery(ctx context.Context, query string, options ...Option) ([]float64, error)
}
