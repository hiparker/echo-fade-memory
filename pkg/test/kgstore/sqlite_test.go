package kgstore_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/hiparker/echo-fade-memory/pkg/core/model"
	kgsqlite "github.com/hiparker/echo-fade-memory/pkg/port/kgstore/sqlite"
)

func TestSQLiteStoreUpsertAndQuery(t *testing.T) {
	store, err := kgsqlite.New(filepath.Join(t.TempDir(), "kg.db"))
	if err != nil {
		t.Fatalf("kgsqlite.New: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})

	entity := &model.Entity{
		ID:            "entity-project-go",
		CanonicalName: "go project",
		DisplayName:   "Go Project",
		EntityType:    "project",
		Confidence:    0.92,
		MemoryCount:   2,
		FirstSeenAt:   time.Now().Add(-time.Hour),
		LastSeenAt:    time.Now(),
	}
	if err := store.UpsertEntity(entity, []string{"golang project", "echo fade memory"}); err != nil {
		t.Fatalf("UpsertEntity: %v", err)
	}

	got, err := store.GetEntity(entity.ID)
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if got == nil || got.CanonicalName != entity.CanonicalName {
		t.Fatalf("unexpected entity: %+v", got)
	}

	results, err := store.FindEntities("golang", 10)
	if err != nil {
		t.Fatalf("FindEntities: %v", err)
	}
	if len(results) != 1 || results[0].ID != entity.ID {
		t.Fatalf("unexpected entity search results: %+v", results)
	}
}

func TestSQLiteStoreRelationsAndMemoryLinks(t *testing.T) {
	store, err := kgsqlite.New(filepath.Join(t.TempDir(), "kg.db"))
	if err != nil {
		t.Fatalf("kgsqlite.New: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})

	now := time.Now()
	if err := store.UpsertEntity(&model.Entity{
		ID:            "entity-user",
		CanonicalName: "system user",
		DisplayName:   "System User",
		EntityType:    "person",
		FirstSeenAt:   now,
		LastSeenAt:    now,
	}, nil); err != nil {
		t.Fatalf("UpsertEntity user: %v", err)
	}
	if err := store.UpsertEntity(&model.Entity{
		ID:            "entity-theme",
		CanonicalName: "dark mode",
		DisplayName:   "Dark Mode",
		EntityType:    "preference",
		FirstSeenAt:   now,
		LastSeenAt:    now,
	}, nil); err != nil {
		t.Fatalf("UpsertEntity theme: %v", err)
	}

	relation := &model.Relation{
		ID:             "rel-user-prefers-theme",
		FromEntityID:   "entity-user",
		ToEntityID:     "entity-theme",
		RelationType:   "prefers",
		SourceMemoryID: "memory-1",
		Weight:         0.88,
		Confidence:     0.93,
		FirstSeenAt:    now,
		LastSeenAt:     now,
	}
	if err := store.UpsertRelation(relation); err != nil {
		t.Fatalf("UpsertRelation: %v", err)
	}

	relations, err := store.ListRelations("entity-user", 10)
	if err != nil {
		t.Fatalf("ListRelations: %v", err)
	}
	if len(relations) != 1 || relations[0].ID != relation.ID {
		t.Fatalf("unexpected relations: %+v", relations)
	}

	links := []model.MemoryEntityLink{
		{MemoryID: "memory-1", EntityID: "entity-user", Role: "subject", Mention: "user", Confidence: 0.9},
		{MemoryID: "memory-1", EntityID: "entity-theme", Role: "object", Mention: "dark mode", Confidence: 0.95},
	}
	if err := store.ReplaceMemoryEntityLinks("memory-1", links); err != nil {
		t.Fatalf("ReplaceMemoryEntityLinks: %v", err)
	}

	gotLinks, err := store.ListMemoryEntityLinks("memory-1")
	if err != nil {
		t.Fatalf("ListMemoryEntityLinks: %v", err)
	}
	if len(gotLinks) != 2 {
		t.Fatalf("links len = %d, want 2", len(gotLinks))
	}

	entityLinks, err := store.ListEntityMemoryLinks("entity-user", 10)
	if err != nil {
		t.Fatalf("ListEntityMemoryLinks: %v", err)
	}
	if len(entityLinks) != 1 || entityLinks[0].MemoryID != "memory-1" {
		t.Fatalf("unexpected entity links: %+v", entityLinks)
	}

	entityCount, err := store.CountEntities()
	if err != nil {
		t.Fatalf("CountEntities: %v", err)
	}
	if entityCount != 2 {
		t.Fatalf("entity count = %d, want 2", entityCount)
	}

	relationCount, err := store.CountRelations()
	if err != nil {
		t.Fatalf("CountRelations: %v", err)
	}
	if relationCount != 1 {
		t.Fatalf("relation count = %d, want 1", relationCount)
	}

	linkCount, err := store.CountMemoryEntityLinks()
	if err != nil {
		t.Fatalf("CountMemoryEntityLinks: %v", err)
	}
	if linkCount != 2 {
		t.Fatalf("link count = %d, want 2", linkCount)
	}
}
