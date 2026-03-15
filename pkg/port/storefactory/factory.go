package storefactory

import (
	"fmt"
	"strings"

	"github.com/hiparker/echo-fade-memory/pkg/config"
	"github.com/hiparker/echo-fade-memory/pkg/port/memstore"
	"github.com/hiparker/echo-fade-memory/pkg/port/store"
	"github.com/hiparker/echo-fade-memory/pkg/port/store/mysql"
	"github.com/hiparker/echo-fade-memory/pkg/port/store/postgres"
	"github.com/hiparker/echo-fade-memory/pkg/port/store/sqlite"
	"github.com/hiparker/echo-fade-memory/pkg/port/vector/lancedb"
	"github.com/hiparker/echo-fade-memory/pkg/port/vector/local"
	"github.com/hiparker/echo-fade-memory/pkg/port/vector/milvus"
)

// NewVectorStore creates a vector store from config.
func NewVectorStore(cfg *config.Config) (store.VectorStore, error) {
	switch strings.ToLower(cfg.VectorStore.Type) {
	case "local":
		return local.New(cfg.VectorPath())
	case "lancedb":
		return lancedb.New(cfg)
	case "milvus":
		if cfg.VectorStore.MilvusHost == "" {
			return nil, fmt.Errorf("vector_store type=milvus requires milvus_host or MILVUS_HOST")
		}
		return milvus.New(cfg)
	default:
		return nil, fmt.Errorf("unsupported vector_store type %q", cfg.VectorStore.Type)
	}
}

// NewMemoryStore creates a memory store from config.
func NewMemoryStore(cfg *config.Config) (memstore.MemoryStore, error) {
	switch strings.ToLower(cfg.Storage.Type) {
	case "", "sqlite":
		return sqlite.New(cfg.SQLitePath())
	case "mysql":
		if cfg.Storage.MySQLDSN == "" {
			return nil, fmt.Errorf("storage type=mysql requires mysql_dsn or MYSQL_DSN")
		}
		return mysql.New(cfg.Storage.MySQLDSN)
	case "postgres":
		if cfg.Storage.PostgresDSN == "" {
			return nil, fmt.Errorf("storage type=postgres requires postgres_dsn or POSTGRES_DSN")
		}
		return postgres.New(cfg.Storage.PostgresDSN)
	default:
		return nil, fmt.Errorf("unsupported storage type %q", cfg.Storage.Type)
	}
}
