package sqlite

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hiparker/echo-fade-memory/pkg/core/model"
	_ "modernc.org/sqlite"
)

// Store implements imagestore.Store using SQLite.
type Store struct {
	db *sql.DB
}

// New opens or creates the image SQLite database.
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
		CREATE TABLE IF NOT EXISTS image_assets (
			id TEXT PRIMARY KEY,
			file_path TEXT DEFAULT '',
			url TEXT DEFAULT '',
			sha256 TEXT NOT NULL UNIQUE,
			source_session TEXT DEFAULT '',
			source_kind TEXT DEFAULT '',
			source_actor TEXT DEFAULT '',
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			caption TEXT DEFAULT '',
			tags_json TEXT DEFAULT '[]',
			ocr_text TEXT DEFAULT ''
		);
		CREATE TABLE IF NOT EXISTS image_links (
			image_id TEXT NOT NULL,
			link_type TEXT NOT NULL,
			target_id TEXT NOT NULL,
			target_label TEXT DEFAULT '',
			created_at INTEGER NOT NULL,
			PRIMARY KEY (image_id, link_type, target_id)
		);
		CREATE INDEX IF NOT EXISTS idx_image_assets_sha256 ON image_assets(sha256);
		CREATE INDEX IF NOT EXISTS idx_image_assets_created_at ON image_assets(created_at);
		CREATE INDEX IF NOT EXISTS idx_image_links_target ON image_links(link_type, target_id);
		CREATE INDEX IF NOT EXISTS idx_image_links_image ON image_links(image_id);
	`)
	return err
}

func (s *Store) Upsert(asset *model.ImageAsset) error {
	if asset == nil {
		return nil
	}
	now := time.Now()
	if asset.CreatedAt.IsZero() {
		asset.CreatedAt = now
	}
	if asset.UpdatedAt.IsZero() {
		asset.UpdatedAt = now
	}
	tagsJSON, err := json.Marshal(asset.Tags)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`
		INSERT OR REPLACE INTO image_assets (
			id, file_path, url, sha256, source_session, source_kind, source_actor,
			created_at, updated_at, caption, tags_json, ocr_text
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, asset.ID, asset.FilePath, asset.URL, asset.SHA256, asset.SourceSession, asset.SourceKind, asset.SourceActor, asset.CreatedAt.Unix(), asset.UpdatedAt.Unix(), asset.Caption, string(tagsJSON), asset.OCRText)
	return err
}

func (s *Store) Get(id string) (*model.ImageAsset, error) {
	return s.scanOne(`
		SELECT id, file_path, url, sha256, source_session, source_kind, source_actor,
		       created_at, updated_at, caption, tags_json, ocr_text
		FROM image_assets WHERE id = ?
	`, id)
}

func (s *Store) GetBySHA256(sha256 string) (*model.ImageAsset, error) {
	return s.scanOne(`
		SELECT id, file_path, url, sha256, source_session, source_kind, source_actor,
		       created_at, updated_at, caption, tags_json, ocr_text
		FROM image_assets WHERE sha256 = ?
	`, sha256)
}

func (s *Store) Delete(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM image_links WHERE image_id = ?`, id); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.Exec(`DELETE FROM image_assets WHERE id = ?`, id); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) Find(query string, limit int) ([]*model.ImageAsset, error) {
	if limit <= 0 {
		limit = 10
	}
	query = strings.TrimSpace(query)
	sqlQuery := `
		SELECT id, file_path, url, sha256, source_session, source_kind, source_actor,
		       created_at, updated_at, caption, tags_json, ocr_text
		FROM image_assets
	`
	args := []interface{}{}
	if query != "" {
		sqlQuery += ` WHERE caption LIKE ? OR ocr_text LIKE ? OR tags_json LIKE ? OR file_path LIKE ? OR url LIKE ?`
		pattern := "%" + query + "%"
		args = append(args, pattern, pattern, pattern, pattern, pattern)
	}
	sqlQuery += ` ORDER BY updated_at DESC, created_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.Query(sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAssets(rows)
}

func (s *Store) ListAll() ([]*model.ImageAsset, error) {
	rows, err := s.db.Query(`
		SELECT id, file_path, url, sha256, source_session, source_kind, source_actor,
		       created_at, updated_at, caption, tags_json, ocr_text
		FROM image_assets
		ORDER BY created_at DESC, updated_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAssets(rows)
}

func (s *Store) ListRecent(limit int) ([]*model.ImageAsset, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.db.Query(`
		SELECT id, file_path, url, sha256, source_session, source_kind, source_actor,
		       created_at, updated_at, caption, tags_json, ocr_text
		FROM image_assets
		ORDER BY created_at DESC, updated_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAssets(rows)
}

func (s *Store) ReplaceLinks(imageID string, links []model.ImageLink) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM image_links WHERE image_id = ?`, imageID); err != nil {
		_ = tx.Rollback()
		return err
	}
	for _, link := range links {
		createdAt := link.CreatedAt
		if createdAt.IsZero() {
			createdAt = time.Now()
		}
		if _, err := tx.Exec(`
			INSERT OR REPLACE INTO image_links (image_id, link_type, target_id, target_label, created_at)
			VALUES (?, ?, ?, ?, ?)
		`, imageID, link.LinkType, link.TargetID, link.TargetLabel, createdAt.Unix()); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ListLinks(imageID string) ([]model.ImageLink, error) {
	rows, err := s.db.Query(`
		SELECT image_id, link_type, target_id, target_label, created_at
		FROM image_links
		WHERE image_id = ?
		ORDER BY created_at DESC, link_type, target_id
	`, imageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanLinks(rows)
}

func (s *Store) ListLinksByTarget(linkType, targetID string, limit int) ([]model.ImageLink, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.db.Query(`
		SELECT image_id, link_type, target_id, target_label, created_at
		FROM image_links
		WHERE link_type = ? AND target_id = ?
		ORDER BY created_at DESC, image_id
		LIMIT ?
	`, linkType, targetID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanLinks(rows)
}

func (s *Store) CountAssets() (int, error) { return s.countWhere("image_assets", "1=1") }
func (s *Store) CountAssetsWithCaption() (int, error) {
	return s.countWhere("image_assets", "TRIM(caption) <> ''")
}
func (s *Store) CountAssetsWithOCR() (int, error) {
	return s.countWhere("image_assets", "TRIM(ocr_text) <> ''")
}
func (s *Store) CountLinks() (int, error) { return s.countWhere("image_links", "1=1") }

func (s *Store) countWhere(table, where string) (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM ` + table + ` WHERE ` + where).Scan(&count)
	return count, err
}

func (s *Store) scanOne(query string, args ...interface{}) (*model.ImageAsset, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	assets, err := scanAssets(rows)
	if err != nil {
		return nil, err
	}
	if len(assets) == 0 {
		return nil, nil
	}
	return assets[0], nil
}

func scanAssets(rows *sql.Rows) ([]*model.ImageAsset, error) {
	var results []*model.ImageAsset
	for rows.Next() {
		var asset model.ImageAsset
		var createdAt int64
		var updatedAt int64
		var tagsJSON string
		if err := rows.Scan(&asset.ID, &asset.FilePath, &asset.URL, &asset.SHA256, &asset.SourceSession, &asset.SourceKind, &asset.SourceActor, &createdAt, &updatedAt, &asset.Caption, &tagsJSON, &asset.OCRText); err != nil {
			return nil, err
		}
		asset.CreatedAt = time.Unix(createdAt, 0)
		asset.UpdatedAt = time.Unix(updatedAt, 0)
		asset.Tags = decodeTags(tagsJSON)
		results = append(results, &asset)
	}
	return results, rows.Err()
}

func scanLinks(rows *sql.Rows) ([]model.ImageLink, error) {
	var results []model.ImageLink
	for rows.Next() {
		var link model.ImageLink
		var createdAt int64
		if err := rows.Scan(&link.ImageID, &link.LinkType, &link.TargetID, &link.TargetLabel, &createdAt); err != nil {
			return nil, err
		}
		link.CreatedAt = time.Unix(createdAt, 0)
		results = append(results, link)
	}
	return results, rows.Err()
}

func decodeTags(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var tags []string
	if err := json.Unmarshal([]byte(raw), &tags); err != nil {
		return nil
	}
	return tags
}

func (s *Store) Close() error {
	return s.db.Close()
}
