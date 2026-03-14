package store

import "context"

// VectorStore is the interface for vector similarity search.
type VectorStore interface {
	Add(id string, vec []float32) error
	Search(ctx context.Context, query []float32, k int) ([]string, []float32, error)
	Remove(id string) error
}
