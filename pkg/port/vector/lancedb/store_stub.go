//go:build !cgo || !lancedb

package lancedb

import (
	"context"
	"fmt"

	"github.com/echo-fade-memory/echo-fade-memory/pkg/config"
)

// Store is unavailable unless the LanceDB build tag and CGO are enabled.
type Store struct{}

// New returns a helpful error when the real LanceDB adapter is not compiled in.
func New(_ *config.Config) (*Store, error) {
	return nil, fmt.Errorf("vector_store type=lancedb requires `-tags lancedb` and a CGO-enabled build with LanceDB native libraries configured")
}

func (s *Store) Add(id string, vec []float32) error {
	return fmt.Errorf("lancedb adapter unavailable: rebuild with `-tags lancedb` and CGO native libraries")
}

func (s *Store) Search(ctx context.Context, query []float32, k int) ([]string, []float32, error) {
	return nil, nil, fmt.Errorf("lancedb adapter unavailable: rebuild with `-tags lancedb` and CGO native libraries")
}

func (s *Store) Remove(id string) error {
	return fmt.Errorf("lancedb adapter unavailable: rebuild with `-tags lancedb` and CGO native libraries")
}

func (s *Store) Close() error {
	return nil
}
