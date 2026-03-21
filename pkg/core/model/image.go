package model

import "time"

// ImageAsset represents a stored image reference plus derived semantic fields.
type ImageAsset struct {
	ID            string    `json:"id"`
	FilePath      string    `json:"file_path,omitempty"`
	URL           string    `json:"url,omitempty"`
	SHA256        string    `json:"sha256"`
	SourceSession string    `json:"source_session,omitempty"`
	SourceKind    string    `json:"source_kind,omitempty"`
	SourceActor   string    `json:"source_actor,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	Caption       string    `json:"caption,omitempty"`
	Tags          []string  `json:"tags,omitempty"`
	OCRText       string    `json:"ocr_text,omitempty"`
}

// ImageLink binds an image to a memory/session/entity/decision.
type ImageLink struct {
	ImageID     string    `json:"image_id"`
	LinkType    string    `json:"link_type"`
	TargetID    string    `json:"target_id"`
	TargetLabel string    `json:"target_label,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// ImageRecallResult carries a recalled image with backend evidence.
type ImageRecallResult struct {
	Asset           *ImageAsset `json:"asset,omitempty"`
	ImageID         string      `json:"image_id"`
	Score           float64     `json:"score"`
	VectorScore     float64     `json:"vector_score,omitempty"`
	VectorRank      int         `json:"vector_rank,omitempty"`
	KeywordScore    float64     `json:"keyword_score,omitempty"`
	KeywordRank     int         `json:"keyword_rank,omitempty"`
	LinkedBoost     float64     `json:"linked_boost,omitempty"`
	LinkedMemoryIDs []string    `json:"linked_memory_ids,omitempty"`
}
