package chromem

import (
	"context"
	"fmt"
	"runtime"

	chromemgo "github.com/philippgille/chromem-go"
)

const collectionName = "echo_fade_memory"

// Store implements store.VectorStore using chromem-go (pure Go, embedded).
type Store struct {
	db   *chromemgo.DB
	coll *chromemgo.Collection
}

// New creates a persistent chromem-go vector store at the given directory.
func New(path string) (*Store, error) {
	db, err := chromemgo.NewPersistentDB(path, false)
	if err != nil {
		return nil, fmt.Errorf("chromem: open db: %w", err)
	}

	// No-op embedding function; we always supply pre-computed vectors.
	noop := func(_ context.Context, _ string) ([]float32, error) {
		return nil, nil
	}

	coll, err := db.GetOrCreateCollection(collectionName, nil, noop)
	if err != nil {
		return nil, fmt.Errorf("chromem: create collection: %w", err)
	}

	return &Store{db: db, coll: coll}, nil
}

func (s *Store) Add(id string, vec []float32) error {
	return s.coll.AddDocument(context.Background(), chromemgo.Document{
		ID:        id,
		Content:   id,
		Embedding: vec,
	})
}

func (s *Store) Search(ctx context.Context, query []float32, k int) ([]string, []float32, error) {
	if s.coll.Count() == 0 {
		return nil, nil, nil
	}
	n := k
	if n > s.coll.Count() {
		n = s.coll.Count()
	}
	results, err := s.coll.QueryEmbedding(ctx, query, n, nil, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("chromem: query: %w", err)
	}
	ids := make([]string, len(results))
	scores := make([]float32, len(results))
	for i, r := range results {
		ids[i] = r.ID
		scores[i] = r.Similarity
	}
	return ids, scores, nil
}

func (s *Store) Remove(id string) error {
	return s.coll.Delete(context.Background(), nil, nil, id)
}

// VectorCount returns number of vectors in collection.
func (s *Store) VectorCount() int {
	return s.coll.Count()
}

// Compact triggers a GC to release memory after bulk operations.
func (s *Store) Close() error {
	runtime.GC()
	return nil
}
