package sqlite

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hiparker/echo-fade-memory/pkg/core/model"
	_ "modernc.org/sqlite"
)

// Store implements kgstore.Store with SQLite.
type Store struct {
	db *sql.DB
}

// New opens or creates the SQLite KG database.
func New(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err := db.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA busy_timeout=5000;`); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA synchronous=NORMAL;`); err != nil {
		_ = db.Close()
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS kg_entities (
			id TEXT PRIMARY KEY,
			canonical_name TEXT NOT NULL,
			display_name TEXT NOT NULL,
			entity_type TEXT NOT NULL,
			description TEXT DEFAULT '',
			confidence REAL DEFAULT 0,
			memory_count INTEGER DEFAULT 0,
			first_seen_at INTEGER NOT NULL,
			last_seen_at INTEGER NOT NULL
		);
		CREATE TABLE IF NOT EXISTS kg_entity_aliases (
			entity_id TEXT NOT NULL,
			alias TEXT NOT NULL,
			PRIMARY KEY (entity_id, alias)
		);
		CREATE TABLE IF NOT EXISTS kg_relations (
			id TEXT PRIMARY KEY,
			from_entity_id TEXT NOT NULL,
			to_entity_id TEXT NOT NULL,
			relation_type TEXT NOT NULL,
			evidence TEXT DEFAULT '',
			source_memory_id TEXT DEFAULT '',
			weight REAL DEFAULT 0,
			confidence REAL DEFAULT 0,
			first_seen_at INTEGER NOT NULL,
			last_seen_at INTEGER NOT NULL
		);
		CREATE TABLE IF NOT EXISTS kg_memory_entity_links (
			memory_id TEXT NOT NULL,
			entity_id TEXT NOT NULL,
			role TEXT DEFAULT '',
			mention TEXT DEFAULT '',
			confidence REAL DEFAULT 0,
			PRIMARY KEY (memory_id, entity_id, role, mention)
		);
		CREATE INDEX IF NOT EXISTS idx_kg_entities_canonical_name ON kg_entities(canonical_name);
		CREATE INDEX IF NOT EXISTS idx_kg_entities_entity_type ON kg_entities(entity_type);
		CREATE INDEX IF NOT EXISTS idx_kg_entity_aliases_alias ON kg_entity_aliases(alias);
		CREATE INDEX IF NOT EXISTS idx_kg_relations_from ON kg_relations(from_entity_id);
		CREATE INDEX IF NOT EXISTS idx_kg_relations_to ON kg_relations(to_entity_id);
		CREATE INDEX IF NOT EXISTS idx_kg_relations_type ON kg_relations(relation_type);
		CREATE INDEX IF NOT EXISTS idx_kg_relations_memory ON kg_relations(source_memory_id);
		CREATE INDEX IF NOT EXISTS idx_kg_memory_entity_links_memory ON kg_memory_entity_links(memory_id);
		CREATE INDEX IF NOT EXISTS idx_kg_memory_entity_links_entity ON kg_memory_entity_links(entity_id);
	`)
	return err
}

func (s *Store) UpsertEntity(entity *model.Entity, aliases []string) error {
	if entity == nil {
		return nil
	}
	now := time.Now()
	if entity.FirstSeenAt.IsZero() {
		entity.FirstSeenAt = now
	}
	if entity.LastSeenAt.IsZero() {
		entity.LastSeenAt = entity.FirstSeenAt
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`
		INSERT OR REPLACE INTO kg_entities (
			id, canonical_name, display_name, entity_type, description, confidence, memory_count, first_seen_at, last_seen_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, entity.ID, entity.CanonicalName, entity.DisplayName, entity.EntityType, entity.Description, entity.Confidence, entity.MemoryCount, entity.FirstSeenAt.Unix(), entity.LastSeenAt.Unix()); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.Exec(`DELETE FROM kg_entity_aliases WHERE entity_id = ?`, entity.ID); err != nil {
		_ = tx.Rollback()
		return err
	}
	for _, alias := range aliases {
		alias = strings.TrimSpace(alias)
		if alias == "" {
			continue
		}
		if _, err := tx.Exec(`INSERT OR REPLACE INTO kg_entity_aliases (entity_id, alias) VALUES (?, ?)`, entity.ID, alias); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) GetEntity(id string) (*model.Entity, error) {
	var entity model.Entity
	var firstSeenAt int64
	var lastSeenAt int64
	err := s.db.QueryRow(`
		SELECT id, canonical_name, display_name, entity_type, description, confidence, memory_count, first_seen_at, last_seen_at
		FROM kg_entities
		WHERE id = ?
	`, id).Scan(
		&entity.ID,
		&entity.CanonicalName,
		&entity.DisplayName,
		&entity.EntityType,
		&entity.Description,
		&entity.Confidence,
		&entity.MemoryCount,
		&firstSeenAt,
		&lastSeenAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	entity.FirstSeenAt = time.Unix(firstSeenAt, 0)
	entity.LastSeenAt = time.Unix(lastSeenAt, 0)
	return &entity, nil
}

func (s *Store) FindEntities(query string, limit int) ([]*model.Entity, error) {
	query = strings.TrimSpace(query)
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.db.Query(`
		SELECT DISTINCT
			e.id, e.canonical_name, e.display_name, e.entity_type, e.description, e.confidence, e.memory_count, e.first_seen_at, e.last_seen_at
		FROM kg_entities e
		LEFT JOIN kg_entity_aliases a ON a.entity_id = e.id
		WHERE e.canonical_name LIKE ? OR e.display_name LIKE ? OR COALESCE(a.alias, '') LIKE ?
		ORDER BY e.memory_count DESC, e.last_seen_at DESC
		LIMIT ?
	`, "%"+query+"%", "%"+query+"%", "%"+query+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []*model.Entity
	for rows.Next() {
		var entity model.Entity
		var firstSeenAt int64
		var lastSeenAt int64
		if err := rows.Scan(
			&entity.ID,
			&entity.CanonicalName,
			&entity.DisplayName,
			&entity.EntityType,
			&entity.Description,
			&entity.Confidence,
			&entity.MemoryCount,
			&firstSeenAt,
			&lastSeenAt,
		); err != nil {
			return nil, err
		}
		entity.FirstSeenAt = time.Unix(firstSeenAt, 0)
		entity.LastSeenAt = time.Unix(lastSeenAt, 0)
		results = append(results, &entity)
	}
	return results, rows.Err()
}

func (s *Store) UpsertRelation(relation *model.Relation) error {
	if relation == nil {
		return nil
	}
	now := time.Now()
	if relation.FirstSeenAt.IsZero() {
		relation.FirstSeenAt = now
	}
	if relation.LastSeenAt.IsZero() {
		relation.LastSeenAt = relation.FirstSeenAt
	}
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO kg_relations (
			id, from_entity_id, to_entity_id, relation_type, evidence, source_memory_id, weight, confidence, first_seen_at, last_seen_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, relation.ID, relation.FromEntityID, relation.ToEntityID, relation.RelationType, relation.Evidence, relation.SourceMemoryID, relation.Weight, relation.Confidence, relation.FirstSeenAt.Unix(), relation.LastSeenAt.Unix())
	return err
}

func (s *Store) ListRelations(entityID string, limit int) ([]*model.Relation, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(`
		SELECT id, from_entity_id, to_entity_id, relation_type, evidence, source_memory_id, weight, confidence, first_seen_at, last_seen_at
		FROM kg_relations
		WHERE from_entity_id = ? OR to_entity_id = ?
		ORDER BY weight DESC, last_seen_at DESC
		LIMIT ?
	`, entityID, entityID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []*model.Relation
	for rows.Next() {
		var relation model.Relation
		var firstSeenAt int64
		var lastSeenAt int64
		if err := rows.Scan(
			&relation.ID,
			&relation.FromEntityID,
			&relation.ToEntityID,
			&relation.RelationType,
			&relation.Evidence,
			&relation.SourceMemoryID,
			&relation.Weight,
			&relation.Confidence,
			&firstSeenAt,
			&lastSeenAt,
		); err != nil {
			return nil, err
		}
		relation.FirstSeenAt = time.Unix(firstSeenAt, 0)
		relation.LastSeenAt = time.Unix(lastSeenAt, 0)
		results = append(results, &relation)
	}
	return results, rows.Err()
}

func (s *Store) ReplaceMemoryEntityLinks(memoryID string, links []model.MemoryEntityLink) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM kg_memory_entity_links WHERE memory_id = ?`, memoryID); err != nil {
		_ = tx.Rollback()
		return err
	}
	for _, link := range links {
		if _, err := tx.Exec(`
			INSERT OR REPLACE INTO kg_memory_entity_links (memory_id, entity_id, role, mention, confidence)
			VALUES (?, ?, ?, ?, ?)
		`, memoryID, link.EntityID, link.Role, link.Mention, link.Confidence); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ListMemoryEntityLinks(memoryID string) ([]model.MemoryEntityLink, error) {
	rows, err := s.db.Query(`
		SELECT memory_id, entity_id, role, mention, confidence
		FROM kg_memory_entity_links
		WHERE memory_id = ?
		ORDER BY entity_id, role, mention
	`, memoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []model.MemoryEntityLink
	for rows.Next() {
		var link model.MemoryEntityLink
		if err := rows.Scan(&link.MemoryID, &link.EntityID, &link.Role, &link.Mention, &link.Confidence); err != nil {
			return nil, err
		}
		results = append(results, link)
	}
	return results, rows.Err()
}

func (s *Store) ListEntityMemoryLinks(entityID string, limit int) ([]model.MemoryEntityLink, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(`
		SELECT memory_id, entity_id, role, mention, confidence
		FROM kg_memory_entity_links
		WHERE entity_id = ?
		ORDER BY confidence DESC, memory_id, role, mention
		LIMIT ?
	`, entityID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []model.MemoryEntityLink
	for rows.Next() {
		var link model.MemoryEntityLink
		if err := rows.Scan(&link.MemoryID, &link.EntityID, &link.Role, &link.Mention, &link.Confidence); err != nil {
			return nil, err
		}
		results = append(results, link)
	}
	return results, rows.Err()
}

func (s *Store) CountEntities() (int, error) {
	return s.countTable("kg_entities")
}

func (s *Store) CountRelations() (int, error) {
	return s.countTable("kg_relations")
}

func (s *Store) CountMemoryEntityLinks() (int, error) {
	return s.countTable("kg_memory_entity_links")
}

func (s *Store) countTable(table string) (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&count)
	return count, err
}

func (s *Store) Close() error {
	return s.db.Close()
}
