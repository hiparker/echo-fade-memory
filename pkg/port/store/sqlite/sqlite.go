package sqlite

import (
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/echo-fade-memory/echo-fade-memory/pkg/core/model"
	"github.com/echo-fade-memory/echo-fade-memory/pkg/port/memstore"
	_ "modernc.org/sqlite"
)

// Store implements memstore.MemoryStore with SQLite.
type Store struct {
	db *sql.DB
}

// New opens or creates the SQLite database.
func New(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
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
			source_refs TEXT DEFAULT '[]',
			grounding_status TEXT DEFAULT 'derived',
			conflict_group TEXT DEFAULT '',
			version INTEGER DEFAULT 1,
			embedding BLOB,
			created_at INTEGER NOT NULL,
			last_accessed_at INTEGER NOT NULL,
			access_count INTEGER DEFAULT 0,
			importance REAL DEFAULT 0.5,
			emotional_weight REAL DEFAULT 0,
			clarity REAL DEFAULT 1.0,
			residual_form TEXT DEFAULT '',
			residual_content TEXT DEFAULT ''
		);
		CREATE INDEX IF NOT EXISTS idx_memories_created ON memories(created_at);
		CREATE INDEX IF NOT EXISTS idx_memories_clarity ON memories(clarity);
	`)
	if err != nil {
		return err
	}
	for _, stmt := range []string{
		"ALTER TABLE memories ADD COLUMN summary TEXT DEFAULT ''",
		"ALTER TABLE memories ADD COLUMN memory_type TEXT DEFAULT 'long_term'",
		"ALTER TABLE memories ADD COLUMN lifecycle_state TEXT DEFAULT 'fresh'",
		"ALTER TABLE memories ADD COLUMN source_refs TEXT DEFAULT '[]'",
		"ALTER TABLE memories ADD COLUMN grounding_status TEXT DEFAULT 'derived'",
		"ALTER TABLE memories ADD COLUMN conflict_group TEXT DEFAULT ''",
		"ALTER TABLE memories ADD COLUMN version INTEGER DEFAULT 1",
	} {
		if _, err := s.db.Exec(stmt); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
			return err
		}
	}
	return nil
}

func (s *Store) Save(m *model.Memory) error {
	emb, _ := json.Marshal(m.Embedding)
	sourceRefs, _ := json.Marshal(m.SourceRefs)
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO memories (id, content, summary, memory_type, lifecycle_state, source_refs, grounding_status, conflict_group, version, embedding, created_at, last_accessed_at, access_count, importance, emotional_weight, clarity, residual_form, residual_content)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, m.ID, m.Content, m.Summary, m.MemoryType, m.LifecycleState, string(sourceRefs), m.GroundingStatus, m.ConflictGroup, m.Version, emb, m.CreatedAt.Unix(), m.LastAccessedAt.Unix(), m.AccessCount, m.Importance, m.EmotionalWeight, m.Clarity, m.ResidualForm, m.ResidualContent)
	return err
}

func (s *Store) Delete(id string) error {
	_, err := s.db.Exec("DELETE FROM memories WHERE id = ?", id)
	return err
}

func (s *Store) Get(id string) (*model.Memory, error) {
	var m model.Memory
	var emb []byte
	var sourceRefs string
	var createdAt, lastAccessed int64
	err := s.db.QueryRow(`
		SELECT id, content, summary, memory_type, lifecycle_state, source_refs, grounding_status, conflict_group, version, embedding, created_at, last_accessed_at, access_count, importance, emotional_weight, clarity, residual_form, residual_content
		FROM memories WHERE id = ?
	`, id).Scan(&m.ID, &m.Content, &m.Summary, &m.MemoryType, &m.LifecycleState, &sourceRefs, &m.GroundingStatus, &m.ConflictGroup, &m.Version, &emb, &createdAt, &lastAccessed, &m.AccessCount, &m.Importance, &m.EmotionalWeight, &m.Clarity, &m.ResidualForm, &m.ResidualContent)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	m.CreatedAt = time.Unix(createdAt, 0)
	m.LastAccessedAt = time.Unix(lastAccessed, 0)
	_ = json.Unmarshal(emb, &m.Embedding)
	if sourceRefs != "" {
		_ = json.Unmarshal([]byte(sourceRefs), &m.SourceRefs)
	}
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
	rows, err := s.db.Query("SELECT id, content, summary, memory_type, lifecycle_state, source_refs, grounding_status, conflict_group, version, embedding, created_at, last_accessed_at, access_count, importance, emotional_weight, clarity, residual_form, residual_content FROM memories")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Memory
	for rows.Next() {
		var m model.Memory
		var emb []byte
		var sourceRefs string
		var createdAt, lastAccessed int64
		if err := rows.Scan(&m.ID, &m.Content, &m.Summary, &m.MemoryType, &m.LifecycleState, &sourceRefs, &m.GroundingStatus, &m.ConflictGroup, &m.Version, &emb, &createdAt, &lastAccessed, &m.AccessCount, &m.Importance, &m.EmotionalWeight, &m.Clarity, &m.ResidualForm, &m.ResidualContent); err != nil {
			return nil, err
		}
		m.CreatedAt = time.Unix(createdAt, 0)
		m.LastAccessedAt = time.Unix(lastAccessed, 0)
		_ = json.Unmarshal(emb, &m.Embedding)
		if sourceRefs != "" {
			_ = json.Unmarshal([]byte(sourceRefs), &m.SourceRefs)
		}
		out = append(out, &m)
	}
	return out, rows.Err()
}

func (s *Store) UpdateAccess(id string, count int) error {
	_, err := s.db.Exec("UPDATE memories SET last_accessed_at = ?, access_count = ? WHERE id = ?", time.Now().Unix(), count, id)
	return err
}

func (s *Store) UpdateDecay(id string, clarity float64, lifecycleState, residualForm, residualContent string) error {
	_, err := s.db.Exec("UPDATE memories SET clarity = ?, lifecycle_state = ?, residual_form = ?, residual_content = ? WHERE id = ?", clarity, lifecycleState, residualForm, residualContent, id)
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
	stmt, err := tx.Prepare("UPDATE memories SET clarity = ?, lifecycle_state = ?, residual_form = ?, residual_content = ? WHERE id = ?")
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
