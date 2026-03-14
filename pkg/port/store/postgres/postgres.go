package postgres

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/echo-fade-memory/echo-fade-memory/pkg/core/model"
	"github.com/echo-fade-memory/echo-fade-memory/pkg/port/memstore"
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
		CREATE INDEX IF NOT EXISTS idx_memories_created ON memories(created_at);
		CREATE INDEX IF NOT EXISTS idx_memories_clarity ON memories(clarity);
	`)
	return err
}

func (s *Store) Save(m *model.Memory) error {
	emb, _ := json.Marshal(m.Embedding)
	_, err := s.db.Exec(`
		INSERT INTO memories (id, content, embedding, created_at, last_accessed_at, access_count, importance, emotional_weight, clarity, residual_form, residual_content)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (id) DO UPDATE SET
			content = EXCLUDED.content,
			embedding = EXCLUDED.embedding,
			created_at = EXCLUDED.created_at,
			last_accessed_at = EXCLUDED.last_accessed_at,
			access_count = EXCLUDED.access_count,
			importance = EXCLUDED.importance,
			emotional_weight = EXCLUDED.emotional_weight,
			clarity = EXCLUDED.clarity,
			residual_form = EXCLUDED.residual_form,
			residual_content = EXCLUDED.residual_content
	`, m.ID, m.Content, emb, m.CreatedAt.Unix(), m.LastAccessedAt.Unix(), m.AccessCount, m.Importance, m.EmotionalWeight, m.Clarity, m.ResidualForm, m.ResidualContent)
	return err
}

func (s *Store) Delete(id string) error {
	_, err := s.db.Exec("DELETE FROM memories WHERE id = $1", id)
	return err
}

func (s *Store) Get(id string) (*model.Memory, error) {
	var m model.Memory
	var emb []byte
	var createdAt, lastAccessed int64
	err := s.db.QueryRow(`
		SELECT id, content, embedding, created_at, last_accessed_at, access_count, importance, emotional_weight, clarity, residual_form, residual_content
		FROM memories WHERE id = $1
	`, id).Scan(&m.ID, &m.Content, &emb, &createdAt, &lastAccessed, &m.AccessCount, &m.Importance, &m.EmotionalWeight, &m.Clarity, &m.ResidualForm, &m.ResidualContent)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	m.CreatedAt = time.Unix(createdAt, 0)
	m.LastAccessedAt = time.Unix(lastAccessed, 0)
	_ = json.Unmarshal(emb, &m.Embedding)
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
	rows, err := s.db.Query("SELECT id, content, embedding, created_at, last_accessed_at, access_count, importance, emotional_weight, clarity, residual_form, residual_content FROM memories")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Memory
	for rows.Next() {
		var m model.Memory
		var emb []byte
		var createdAt, lastAccessed int64
		if err := rows.Scan(&m.ID, &m.Content, &emb, &createdAt, &lastAccessed, &m.AccessCount, &m.Importance, &m.EmotionalWeight, &m.Clarity, &m.ResidualForm, &m.ResidualContent); err != nil {
			return nil, err
		}
		m.CreatedAt = time.Unix(createdAt, 0)
		m.LastAccessedAt = time.Unix(lastAccessed, 0)
		_ = json.Unmarshal(emb, &m.Embedding)
		out = append(out, &m)
	}
	return out, rows.Err()
}

func (s *Store) UpdateAccess(id string, count int) error {
	_, err := s.db.Exec("UPDATE memories SET last_accessed_at = $1, access_count = $2 WHERE id = $3", time.Now().Unix(), count, id)
	return err
}

func (s *Store) UpdateDecay(id string, clarity float64, residualForm, residualContent string) error {
	_, err := s.db.Exec("UPDATE memories SET clarity = $1, residual_form = $2, residual_content = $3 WHERE id = $4", clarity, residualForm, residualContent, id)
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
	stmt, err := tx.Prepare("UPDATE memories SET clarity = $1, residual_form = $2, residual_content = $3 WHERE id = $4")
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, u := range updates {
		_, err = stmt.Exec(u.Clarity, u.ResidualForm, u.ResidualContent, u.ID)
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
