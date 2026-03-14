package memstore

import (
	"github.com/echo-fade-memory/echo-fade-memory/pkg/core/model"
)

// MemoryStore is the interface for memory metadata persistence (SQL).
type MemoryStore interface {
	Save(m *model.Memory) error
	Get(id string) (*model.Memory, error)
	Delete(id string) error
	List() ([]string, error)
	ListAll() ([]*model.Memory, error)
	UpdateAccess(id string, count int) error
	UpdateDecay(id string, clarity float64, lifecycleState, residualForm, residualContent string) error
	UpdateDecayBatch(updates []DecayUpdate) error
	Close() error
}

// DecayUpdate holds decay fields for batch update.
type DecayUpdate struct {
	ID              string
	Clarity         float64
	LifecycleState  string
	ResidualForm    string
	ResidualContent string
}
