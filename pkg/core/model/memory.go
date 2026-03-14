package model

import "time"

// Memory represents a memory unit with decay state.
type Memory struct {
	ID              string    `json:"id"`
	Content         string    `json:"content"`
	Embedding       []float32 `json:"-"`
	CreatedAt       time.Time `json:"created_at"`
	LastAccessedAt  time.Time `json:"last_accessed_at"`
	AccessCount     int       `json:"access_count"`
	Importance      float64   `json:"importance"`
	EmotionalWeight float64   `json:"emotional_weight"`
	Clarity         float64   `json:"clarity"`
	ResidualForm    string    `json:"residual_form"`
	ResidualContent string    `json:"residual_content"`
}

// DecayStage represents the text degradation stage by age.
type DecayStage int

const (
	StageFull DecayStage = iota
	StageSummary
	StageKeywords
	StageFragment
	StageOutline
)

// StageThresholds define when each stage applies (days).
var StageThresholds = map[DecayStage]int{
	StageFull:     0,
	StageSummary:  7,
	StageKeywords: 30,
	StageFragment: 90,
	StageOutline:  180,
}
