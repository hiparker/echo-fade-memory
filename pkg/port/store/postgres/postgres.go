package postgres

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hiparker/echo-fade-memory/pkg/core/model"
	"github.com/hiparker/echo-fade-memory/pkg/port/memstore"
	_ "github.com/lib/pq"
)

// Store implements memstore.MemoryStore with PostgreSQL.
type Store struct {
	db *sql.DB
}

// New creates a PostgreSQL memory store.
func New(dsn string) (*Store, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS memories (
			id TEXT PRIMARY KEY,
			content TEXT NOT NULL,
			summary TEXT DEFAULT '',
			memory_type TEXT DEFAULT 'long_term',
			lifecycle_state TEXT DEFAULT 'fresh',
			grounding_status TEXT DEFAULT 'derived',
			embedding BYTEA,
			created_at BIGINT NOT NULL,
			last_accessed_at BIGINT NOT NULL,
			access_count INTEGER DEFAULT 0,
			importance DOUBLE PRECISION DEFAULT 0.5,
			emotional_weight DOUBLE PRECISION DEFAULT 0,
			clarity DOUBLE PRECISION DEFAULT 1.0,
			residual_form TEXT DEFAULT '',
			residual_content TEXT DEFAULT ''
		);
		CREATE TABLE IF NOT EXISTS memory_versions (
			memory_id TEXT PRIMARY KEY REFERENCES memories(id) ON DELETE CASCADE,
			conflict_group TEXT NOT NULL,
			version INTEGER NOT NULL,
			created_at BIGINT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS memory_sources (
			memory_id TEXT NOT NULL REFERENCES memories(id) ON DELETE CASCADE,
			seq INTEGER NOT NULL,
			kind TEXT DEFAULT '',
			ref TEXT NOT NULL,
			title TEXT DEFAULT '',
			snippet TEXT DEFAULT '',
			PRIMARY KEY (memory_id, seq)
		);
		CREATE INDEX IF NOT EXISTS idx_memories_created ON memories(created_at);
		CREATE INDEX IF NOT EXISTS idx_memories_clarity ON memories(clarity);
		CREATE INDEX IF NOT EXISTS idx_memory_versions_group_version ON memory_versions(conflict_group, version DESC);
		CREATE INDEX IF NOT EXISTS idx_memory_sources_memory_id ON memory_sources(memory_id);
	`)
	if err != nil {
		return err
	}
	for _, stmt := range []string{
		"ALTER TABLE memories ADD COLUMN summary TEXT DEFAULT ''",
		"ALTER TABLE memories ADD COLUMN memory_type TEXT DEFAULT 'long_term'",
		"ALTER TABLE memories ADD COLUMN lifecycle_state TEXT DEFAULT 'fresh'",
		"ALTER TABLE memories ADD COLUMN grounding_status TEXT DEFAULT 'derived'",
	} {
		if _, err := s.db.Exec(stmt); err != nil && !strings.Contains(strings.ToLower(err.Error()), "already exists") {
			return err
		}
	}
	if err := s.migrateLegacyVersions(); err != nil {
		return err
	}
	return s.migrateLegacySources()
}

func (s *Store) Save(m *model.Memory) error {
	emb, _ := json.Marshal(m.Embedding)
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	_, err = tx.Exec(`
		INSERT INTO memories (id, content, summary, memory_type, lifecycle_state, grounding_status, embedding, created_at, last_accessed_at, access_count, importance, emotional_weight, clarity, residual_form, residual_content)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		ON CONFLICT (id) DO UPDATE SET
			content = EXCLUDED.content,
			summary = EXCLUDED.summary,
			memory_type = EXCLUDED.memory_type,
			lifecycle_state = EXCLUDED.lifecycle_state,
			grounding_status = EXCLUDED.grounding_status,
			embedding = EXCLUDED.embedding,
			created_at = EXCLUDED.created_at,
			last_accessed_at = EXCLUDED.last_accessed_at,
			access_count = EXCLUDED.access_count,
			importance = EXCLUDED.importance,
			emotional_weight = EXCLUDED.emotional_weight,
			clarity = EXCLUDED.clarity,
			residual_form = EXCLUDED.residual_form,
			residual_content = EXCLUDED.residual_content
	`, m.ID, m.Content, m.Summary, m.MemoryType, m.LifecycleState, m.GroundingStatus, emb, m.CreatedAt.Unix(), m.LastAccessedAt.Unix(), m.AccessCount, m.Importance, m.EmotionalWeight, m.Clarity, m.ResidualForm, m.ResidualContent)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	_, err = tx.Exec(`
		INSERT INTO memory_versions (memory_id, conflict_group, version, created_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (memory_id) DO UPDATE SET
			conflict_group = EXCLUDED.conflict_group,
			version = EXCLUDED.version,
			created_at = EXCLUDED.created_at
	`, m.ID, m.ConflictGroup, m.Version, m.CreatedAt.Unix())
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := replaceSourceRefsPostgres(tx, m.ID, m.SourceRefs); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) Delete(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM memory_sources WHERE memory_id = $1", id); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.Exec("DELETE FROM memory_versions WHERE memory_id = $1", id); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.Exec("DELETE FROM memories WHERE id = $1", id); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) Get(id string) (*model.Memory, error) {
	var m model.Memory
	var emb []byte
	var createdAt, lastAccessed int64
	err := s.db.QueryRow(`
		SELECT
			m.id, m.content, m.summary, m.memory_type, m.lifecycle_state, m.grounding_status,
			COALESCE(v.conflict_group, ''), COALESCE(v.version, 1),
			m.embedding, m.created_at, m.last_accessed_at, m.access_count, m.importance,
			m.emotional_weight, m.clarity, m.residual_form, m.residual_content
		FROM memories m
		LEFT JOIN memory_versions v ON v.memory_id = m.id
		WHERE m.id = $1
	`, id).Scan(&m.ID, &m.Content, &m.Summary, &m.MemoryType, &m.LifecycleState, &m.GroundingStatus, &m.ConflictGroup, &m.Version, &emb, &createdAt, &lastAccessed, &m.AccessCount, &m.Importance, &m.EmotionalWeight, &m.Clarity, &m.ResidualForm, &m.ResidualContent)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	m.CreatedAt = time.Unix(createdAt, 0)
	m.LastAccessedAt = time.Unix(lastAccessed, 0)
	_ = json.Unmarshal(emb, &m.Embedding)
	sourceRefs, err := loadSourceRefsPostgres(s.db, m.ID)
	if err != nil {
		return nil, err
	}
	m.SourceRefs = sourceRefs
	normalizeLoadedMemory(&m)
	return &m, nil
}

func (s *Store) List() ([]string, error) {
	rows, err := s.db.Query("SELECT id FROM memories ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) ListAll() ([]*model.Memory, error) {
	rows, err := s.db.Query(`
		SELECT
			m.id, m.content, m.summary, m.memory_type, m.lifecycle_state, m.grounding_status,
			COALESCE(v.conflict_group, ''), COALESCE(v.version, 1),
			m.embedding, m.created_at, m.last_accessed_at, m.access_count, m.importance,
			m.emotional_weight, m.clarity, m.residual_form, m.residual_content
		FROM memories m
		LEFT JOIN memory_versions v ON v.memory_id = m.id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Memory
	for rows.Next() {
		var m model.Memory
		var emb []byte
		var createdAt, lastAccessed int64
		if err := rows.Scan(&m.ID, &m.Content, &m.Summary, &m.MemoryType, &m.LifecycleState, &m.GroundingStatus, &m.ConflictGroup, &m.Version, &emb, &createdAt, &lastAccessed, &m.AccessCount, &m.Importance, &m.EmotionalWeight, &m.Clarity, &m.ResidualForm, &m.ResidualContent); err != nil {
			return nil, err
		}
		m.CreatedAt = time.Unix(createdAt, 0)
		m.LastAccessedAt = time.Unix(lastAccessed, 0)
		_ = json.Unmarshal(emb, &m.Embedding)
		out = append(out, &m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sourceRefsByMemory, err := loadAllSourceRefsPostgres(s.db)
	if err != nil {
		return nil, err
	}
	for _, m := range out {
		m.SourceRefs = sourceRefsByMemory[m.ID]
		normalizeLoadedMemory(m)
	}
	return out, nil
}

func (s *Store) ListByConflictGroup(conflictGroup string) ([]*model.Memory, error) {
	rows, err := s.db.Query(`
		SELECT
			m.id, m.content, m.summary, m.memory_type, m.lifecycle_state, m.grounding_status,
			v.conflict_group, v.version, m.embedding, m.created_at, m.last_accessed_at,
			m.access_count, m.importance, m.emotional_weight, m.clarity, m.residual_form, m.residual_content
		FROM memory_versions v
		JOIN memories m ON m.id = v.memory_id
		WHERE v.conflict_group = $1
		ORDER BY v.version DESC
	`, conflictGroup)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Memory
	for rows.Next() {
		var m model.Memory
		var emb []byte
		var createdAt, lastAccessed int64
		if err := rows.Scan(&m.ID, &m.Content, &m.Summary, &m.MemoryType, &m.LifecycleState, &m.GroundingStatus, &m.ConflictGroup, &m.Version, &emb, &createdAt, &lastAccessed, &m.AccessCount, &m.Importance, &m.EmotionalWeight, &m.Clarity, &m.ResidualForm, &m.ResidualContent); err != nil {
			return nil, err
		}
		m.CreatedAt = time.Unix(createdAt, 0)
		m.LastAccessedAt = time.Unix(lastAccessed, 0)
		_ = json.Unmarshal(emb, &m.Embedding)
		out = append(out, &m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sourceRefsByMemory, err := loadAllSourceRefsPostgres(s.db)
	if err != nil {
		return nil, err
	}
	for _, m := range out {
		m.SourceRefs = sourceRefsByMemory[m.ID]
		normalizeLoadedMemory(m)
	}
	return out, nil
}

func (s *Store) GetLatestByConflictGroup(conflictGroup string) (*model.Memory, error) {
	memories, err := s.ListByConflictGroup(conflictGroup)
	if err != nil || len(memories) == 0 {
		return nil, err
	}
	return memories[0], nil
}

func (s *Store) UpdateAccess(id string, count int) error {
	_, err := s.db.Exec("UPDATE memories SET last_accessed_at = $1, access_count = $2 WHERE id = $3", time.Now().Unix(), count, id)
	return err
}

func (s *Store) UpdateDecay(id string, clarity float64, lifecycleState, residualForm, residualContent string) error {
	_, err := s.db.Exec("UPDATE memories SET clarity = $1, lifecycle_state = $2, residual_form = $3, residual_content = $4 WHERE id = $5", clarity, lifecycleState, residualForm, residualContent, id)
	return err
}

func (s *Store) UpdateDecayBatch(updates []memstore.DecayUpdate) error {
	if len(updates) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("UPDATE memories SET clarity = $1, lifecycle_state = $2, residual_form = $3, residual_content = $4 WHERE id = $5")
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, u := range updates {
		_, err = stmt.Exec(u.Clarity, u.LifecycleState, u.ResidualForm, u.ResidualContent, u.ID)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrateLegacyVersions() error {
	hasConflictGroup, err := s.columnExists("memories", "conflict_group")
	if err != nil {
		return err
	}
	hasVersion, err := s.columnExists("memories", "version")
	if err != nil {
		return err
	}
	if !hasConflictGroup || !hasVersion {
		return nil
	}
	_, err = s.db.Exec(`
		INSERT INTO memory_versions (memory_id, conflict_group, version, created_at)
		SELECT id, conflict_group, version, created_at
		FROM memories
		WHERE BTRIM(COALESCE(conflict_group, '')) <> ''
		ON CONFLICT (memory_id) DO NOTHING
	`)
	return err
}

func (s *Store) migrateLegacySources() error {
	hasSourceRefs, err := s.columnExists("memories", "source_refs")
	if err != nil {
		return err
	}
	if !hasSourceRefs {
		return nil
	}
	rows, err := s.db.Query(`
		SELECT id, source_refs
		FROM memories
		WHERE source_refs IS NOT NULL
		  AND BTRIM(source_refs) <> ''
		  AND BTRIM(source_refs) <> '[]'
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	for rows.Next() {
		var memoryID string
		var raw string
		if err := rows.Scan(&memoryID, &raw); err != nil {
			_ = tx.Rollback()
			return err
		}
		var refs []model.SourceRef
		if err := json.Unmarshal([]byte(raw), &refs); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migrate legacy source_refs for %s: %w", memoryID, err)
		}
		if err := replaceSourceRefsPostgres(tx, memoryID, refs); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	if err := rows.Err(); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) columnExists(table, column string) (bool, error) {
	var exists bool
	err := s.db.QueryRow(`
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = current_schema()
			  AND table_name = $1
			  AND column_name = $2
		)
	`, table, column).Scan(&exists)
	return exists, err
}

func normalizeLoadedMemory(m *model.Memory) {
	if m == nil {
		return
	}
	if m.LifecycleState == "" {
		m.LifecycleState = model.LifecycleStateFromClarity(m.Clarity)
	}
}

func replaceSourceRefsPostgres(tx *sql.Tx, memoryID string, refs []model.SourceRef) error {
	if _, err := tx.Exec("DELETE FROM memory_sources WHERE memory_id = $1", memoryID); err != nil {
		return err
	}
	if len(refs) == 0 {
		return nil
	}
	stmt, err := tx.Prepare(`
		INSERT INTO memory_sources (memory_id, seq, kind, ref, title, snippet)
		VALUES ($1, $2, $3, $4, $5, $6)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for i, ref := range refs {
		if _, err := stmt.Exec(memoryID, i, ref.Kind, ref.Ref, ref.Title, ref.Snippet); err != nil {
			return err
		}
	}
	return nil
}

func loadSourceRefsPostgres(db *sql.DB, memoryID string) ([]model.SourceRef, error) {
	rows, err := db.Query(`
		SELECT kind, ref, title, snippet
		FROM memory_sources
		WHERE memory_id = $1
		ORDER BY seq ASC
	`, memoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var refs []model.SourceRef
	for rows.Next() {
		var ref model.SourceRef
		if err := rows.Scan(&ref.Kind, &ref.Ref, &ref.Title, &ref.Snippet); err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, rows.Err()
}

func loadAllSourceRefsPostgres(db *sql.DB) (map[string][]model.SourceRef, error) {
	rows, err := db.Query(`
		SELECT memory_id, kind, ref, title, snippet
		FROM memory_sources
		ORDER BY memory_id ASC, seq ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string][]model.SourceRef)
	for rows.Next() {
		var memoryID string
		var ref model.SourceRef
		if err := rows.Scan(&memoryID, &ref.Kind, &ref.Ref, &ref.Title, &ref.Snippet); err != nil {
			return nil, err
		}
		out[memoryID] = append(out[memoryID], ref)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
