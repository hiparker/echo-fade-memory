package local

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"gonum.org/v1/gonum/floats"
)

// Store provides vector similarity search (pure Go, file-based).
type Store struct {
	path    string
	vectors map[string][]float32
	mu      sync.RWMutex
}

// New creates or loads a file-based vector store.
func New(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	vs := &Store{path: path, vectors: make(map[string][]float32)}
	_ = vs.load()
	return vs, nil
}

func (vs *Store) load() error {
	data, err := os.ReadFile(vs.path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &vs.vectors)
}

func (vs *Store) save() error {
	data, err := json.Marshal(vs.vectors)
	if err != nil {
		return err
	}
	return os.WriteFile(vs.path, data, 0644)
}

func (vs *Store) Add(id string, vec []float32) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	if vs.vectors == nil {
		vs.vectors = make(map[string][]float32)
	}
	vs.vectors[id] = vec
	return vs.save()
}

func (vs *Store) Search(ctx context.Context, query []float32, k int) ([]string, []float32, error) {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	type score struct {
		id    string
		score float32
	}
	query64 := make([]float64, len(query))
	for i, v := range query {
		query64[i] = float64(v)
	}
	normQ := floats.Norm(query64, 2)
	if normQ < 1e-9 {
		return nil, nil, nil
	}

	results := make([]score, 0, len(vs.vectors))
	for id, vec := range vs.vectors {
		vec64 := make([]float64, len(vec))
		for i, v := range vec {
			vec64[i] = float64(v)
		}
		normV := floats.Norm(vec64, 2)
		if normV < 1e-9 {
			continue
		}
		sim := floats.Dot(query64, vec64) / (normQ * normV)
		if sim > 0 {
			results = append(results, score{id, float32(sim)})
		}
	}

	sort.Slice(results, func(i, j int) bool { return results[i].score > results[j].score })
	if k > len(results) {
		k = len(results)
	}
	ids := make([]string, k)
	scores := make([]float32, k)
	for i := 0; i < k; i++ {
		ids[i] = results[i].id
		scores[i] = results[i].score
	}
	return ids, scores, nil
}

func (vs *Store) Remove(id string) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	delete(vs.vectors, id)
	return vs.save()
}

// VectorCount returns the number of stored vectors.
func (vs *Store) VectorCount() int {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	return len(vs.vectors)
}

// HasVectorID checks whether a vector exists for the given id.
func (vs *Store) HasVectorID(id string) bool {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	_, ok := vs.vectors[id]
	return ok
}
