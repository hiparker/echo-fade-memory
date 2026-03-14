package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/echo-fade-memory/echo-fade-memory/internal/model"
	_ "modernc.org/sqlite"
)

// SQLiteStore persists memory metadata and linkage.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore opens or creates the SQLite database.
func NewSQLiteStore(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	s := &SQLiteStore{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *SQLiteStore) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS memories (
			id TEXT PRIMARY KEY,
			content TEXT NOT NULL,
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
	return err
}

// Save stores a memory.
func (s *SQLiteStore) Save(m *model.Memory) error {
	emb, _ := json.Marshal(m.Embedding)
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO memories (id, content, embedding, created_at, last_accessed_at, access_count, importance, emotional_weight, clarity, residual_form, residual_content)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, m.ID, m.Content, emb, m.CreatedAt.Unix(), m.LastAccessedAt.Unix(), m.AccessCount, m.Importance, m.EmotionalWeight, m.Clarity, m.ResidualForm, m.ResidualContent)
	return err
}

// Get loads a memory by ID.
func (s *SQLiteStore) Get(id string) (*model.Memory, error) {
	var m model.Memory
	var emb []byte
	var createdAt, lastAccessed int64
	err := s.db.QueryRow(`
		SELECT id, content, embedding, created_at, last_accessed_at, access_count, importance, emotional_weight, clarity, residual_form, residual_content
		FROM memories WHERE id = ?
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

// List returns all memory IDs (for vector/bleve sync).
func (s *SQLiteStore) List() ([]string, error) {
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

// UpdateAccess updates access count and last_accessed_at.
func (s *SQLiteStore) UpdateAccess(id string, count int) error {
	_, err := s.db.Exec("UPDATE memories SET last_accessed_at = ?, access_count = ? WHERE id = ?", time.Now().Unix(), count, id)
	return err
}

// UpdateDecay updates clarity and residual.
func (s *SQLiteStore) UpdateDecay(id string, clarity float64, residualForm, residualContent string) error {
	_, err := s.db.Exec("UPDATE memories SET clarity = ?, residual_form = ?, residual_content = ? WHERE id = ?", clarity, residualForm, residualContent, id)
	return err
}

// Close closes the database.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// GetByClarity returns memory IDs with clarity above threshold.
func (s *SQLiteStore) GetByClarity(minClarity float64) ([]string, error) {
	rows, err := s.db.Query("SELECT id FROM memories WHERE clarity >= ? ORDER BY clarity DESC", minClarity)
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
