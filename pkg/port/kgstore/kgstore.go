package kgstore

import "github.com/hiparker/echo-fade-memory/pkg/core/model"

// Store is the portable property-graph persistence abstraction.
// Implementations should remain compatible with SQLite, PostgreSQL, and MySQL.
type Store interface {
	UpsertEntity(entity *model.Entity, aliases []string) error
	GetEntity(id string) (*model.Entity, error)
	FindEntities(query string, limit int) ([]*model.Entity, error)

	UpsertRelation(relation *model.Relation) error
	ListRelations(entityID string, limit int) ([]*model.Relation, error)

	ReplaceMemoryEntityLinks(memoryID string, links []model.MemoryEntityLink) error
	ListMemoryEntityLinks(memoryID string) ([]model.MemoryEntityLink, error)
	ListEntityMemoryLinks(entityID string, limit int) ([]model.MemoryEntityLink, error)
	CountEntities() (int, error)
	CountRelations() (int, error)
	CountMemoryEntityLinks() (int, error)

	Close() error
}
