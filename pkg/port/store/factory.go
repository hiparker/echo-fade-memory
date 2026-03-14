package store

import (
	"fmt"

	"github.com/echo-fade-memory/echo-fade-memory/pkg/config"
)

// NewVectorStore creates a vector store from config.
func NewVectorStore(cfg *config.Config) (VectorStore, error) {
	switch cfg.VectorStore.Type {
	case "local":
		return NewLocalVectorStore(cfg.VectorPath())
	case "lancedb":
		return NewLocalVectorStore(cfg.VectorPath())
	case "milvus":
		return nil, fmt.Errorf("milvus not implemented yet, use type=local")
	default:
		return NewLocalVectorStore(cfg.VectorPath())
	}
}

// NewMemoryStore creates a memory store from config.
func NewMemoryStore(cfg *config.Config) (MemoryStore, error) {
	switch cfg.Storage.Type {
	case "sqlite":
		return NewSQLiteStore(cfg.SQLitePath())
	case "postgres":
		return nil, fmt.Errorf("postgres not implemented yet, use type=sqlite")
	default:
		return NewSQLiteStore(cfg.SQLitePath())
	}
}
