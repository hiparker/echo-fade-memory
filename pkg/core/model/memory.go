package model

import "time"

const (
	MemoryTypeLongTerm   = "long_term"
	MemoryTypeWorking    = "working"
	MemoryTypePreference = "preference"
	MemoryTypeProject    = "project"
	MemoryTypeGoal       = "goal"
)

const (
	LifecycleFresh      = "fresh"
	LifecycleReinforced = "reinforced"
	LifecycleWeakening  = "weakening"
	LifecycleBlurred    = "blurred"
	LifecycleArchived   = "archived"
	LifecycleForgotten  = "forgotten"
)

// SourceRef points back to the original fact source.
type SourceRef struct {
	Kind    string `json:"kind"`
	Ref     string `json:"ref"`
	Title   string `json:"title,omitempty"`
	Snippet string `json:"snippet,omitempty"`
}

// Memory represents a memory unit with decay state.
type Memory struct {
	ID              string      `json:"id"`
	Content         string      `json:"content"`
	Summary         string      `json:"summary,omitempty"`
	MemoryType      string      `json:"memory_type,omitempty"`
	LifecycleState  string      `json:"lifecycle_state,omitempty"`
	SourceRefs      []SourceRef `json:"source_refs,omitempty"`
	GroundingStatus string      `json:"grounding_status,omitempty"`
	ConflictGroup   string      `json:"conflict_group,omitempty"`
	Version         int         `json:"version,omitempty"`
	Embedding       []float32   `json:"-"`
	CreatedAt       time.Time   `json:"created_at"`
	LastAccessedAt  time.Time   `json:"last_accessed_at"`
	AccessCount     int         `json:"access_count"`
	Importance      float64     `json:"importance"`
	EmotionalWeight float64     `json:"emotional_weight"`
	Clarity         float64     `json:"clarity"`
	ResidualForm    string      `json:"residual_form"`
	ResidualContent string      `json:"residual_content"`
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

// PrimarySource returns the most representative source ref.
func (m *Memory) PrimarySource() string {
	if len(m.SourceRefs) == 0 {
		return ""
	}
	if m.SourceRefs[0].Kind == "" {
		return m.SourceRefs[0].Ref
	}
	return m.SourceRefs[0].Kind + ":" + m.SourceRefs[0].Ref
}

// Fuzziness returns how blurry the memory currently is.
func (m *Memory) Fuzziness() float64 {
	if m.Clarity <= 0 {
		return 1
	}
	if m.Clarity >= 1 {
		return 0
	}
	return 1 - m.Clarity
}

// LifecycleStateFromClarity maps the decay level to a lifecycle label.
func LifecycleStateFromClarity(clarity float64) string {
	switch {
	case clarity >= 0.95:
		return LifecycleFresh
	case clarity >= 0.75:
		return LifecycleReinforced
	case clarity >= 0.45:
		return LifecycleWeakening
	case clarity >= 0.2:
		return LifecycleBlurred
	case clarity >= 0.05:
		return LifecycleArchived
	default:
		return LifecycleForgotten
	}
}
