package model

import "time"

// Entity represents a normalized graph node extracted from memories.
type Entity struct {
	ID             string    `json:"id"`
	CanonicalName  string    `json:"canonical_name"`
	DisplayName    string    `json:"display_name"`
	EntityType     string    `json:"entity_type"`
	Description    string    `json:"description,omitempty"`
	Confidence     float64   `json:"confidence,omitempty"`
	MemoryCount    int       `json:"memory_count,omitempty"`
	FirstSeenAt    time.Time `json:"first_seen_at"`
	LastSeenAt     time.Time `json:"last_seen_at"`
}

// EntityAlias maps an alternate surface form to an entity.
type EntityAlias struct {
	EntityID string `json:"entity_id"`
	Alias    string `json:"alias"`
}

// Relation represents an edge between two entities.
type Relation struct {
	ID            string    `json:"id"`
	FromEntityID  string    `json:"from_entity_id"`
	ToEntityID    string    `json:"to_entity_id"`
	RelationType  string    `json:"relation_type"`
	Evidence      string    `json:"evidence,omitempty"`
	SourceMemoryID string   `json:"source_memory_id,omitempty"`
	Weight        float64   `json:"weight,omitempty"`
	Confidence    float64   `json:"confidence,omitempty"`
	FirstSeenAt   time.Time `json:"first_seen_at"`
	LastSeenAt    time.Time `json:"last_seen_at"`
}

// MemoryEntityLink links a memory to an extracted entity mention.
type MemoryEntityLink struct {
	MemoryID    string  `json:"memory_id"`
	EntityID    string  `json:"entity_id"`
	Role        string  `json:"role,omitempty"`
	Mention     string  `json:"mention,omitempty"`
	Confidence  float64 `json:"confidence,omitempty"`
}
