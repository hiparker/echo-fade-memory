package storefactory

import (
	"fmt"
	"strings"

	"github.com/hiparker/echo-fade-memory/pkg/config"
	"github.com/hiparker/echo-fade-memory/pkg/port/imagestore"
	imgsqlite "github.com/hiparker/echo-fade-memory/pkg/port/imagestore/sqlite"
	"github.com/hiparker/echo-fade-memory/pkg/port/kgstore"
	kgsqlite "github.com/hiparker/echo-fade-memory/pkg/port/kgstore/sqlite"
	"github.com/hiparker/echo-fade-memory/pkg/port/memstore"
	"github.com/hiparker/echo-fade-memory/pkg/port/store"
	"github.com/hiparker/echo-fade-memory/pkg/port/store/mysql"
	"github.com/hiparker/echo-fade-memory/pkg/port/store/postgres"
	"github.com/hiparker/echo-fade-memory/pkg/port/store/sqlite"
	"github.com/hiparker/echo-fade-memory/pkg/port/vector/chromem"
	"github.com/hiparker/echo-fade-memory/pkg/port/vector/local"
	"github.com/hiparker/echo-fade-memory/pkg/port/vector/milvus"
)

// NewVectorStore creates a vector store from config.
func NewVectorStore(cfg *config.Config) (store.VectorStore, error) {
	switch strings.ToLower(cfg.VectorStore.Type) {
	case "local":
		return local.New(cfg.VectorPath())
	case "chromem":
		return chromem.New(cfg.VectorPath())
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

// NewKGStore creates the default graph store from config.
// Phase 2 starts with a portable SQLite implementation and will later add
// PostgreSQL/MySQL implementations behind the same abstraction.
func NewKGStore(cfg *config.Config) (kgstore.Store, error) {
	return kgsqlite.New(cfg.KGSQLitePath())
}

// NewImageStore creates the default image metadata store from config.
func NewImageStore(cfg *config.Config) (imagestore.Store, error) {
	return imgsqlite.New(cfg.ImageSQLitePath())
}

// NewImageVectorStore creates the default image vector store.
// Milvus currently falls back to local sidecar storage so image vectors do not
// collide with text-memory collections.
func NewImageVectorStore(cfg *config.Config) (store.VectorStore, error) {
	switch strings.ToLower(cfg.VectorStore.Type) {
	case "chromem":
		return chromem.New(cfg.ImageVectorPath())
	case "milvus":
		return local.New(cfg.ImageVectorPath())
	default:
		return local.New(cfg.ImageVectorPath())
	}
}
