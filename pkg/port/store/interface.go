package store

import (
	"context"

	"github.com/echo-fade-memory/echo-fade-memory/pkg/core/model"
)

// VectorStore is the interface for vector similarity search.
type VectorStore interface {
	Add(id string, vec []float32) error
	Search(ctx context.Context, query []float32, k int) ([]string, []float32, error)
	Remove(id string) error
}

// MemoryStore is the interface for memory metadata persistence.
type MemoryStore interface {
	Save(m *model.Memory) error
	Get(id string) (*model.Memory, error)
	Delete(id string) error
	List() ([]string, error)
	ListAll() ([]*model.Memory, error)
	UpdateAccess(id string, count int) error
	UpdateDecay(id string, clarity float64, residualForm, residualContent string) error
	UpdateDecayBatch(updates []DecayUpdate) error
	Close() error
}

// DecayUpdate holds decay fields for batch update.
type DecayUpdate struct {
	ID              string
	Clarity         float64
	ResidualForm    string
	ResidualContent string
}
