package decay

import (
	"math"
	"time"

	"github.com/echo-fade-memory/echo-fade-memory/internal/model"
)

// Params holds decay algorithm parameters.
type Params struct {
	Lambda           float64 // time decay rate
	AccessBoost      float64 // boost per access
	EmotionalProtect float64 // emotional_weight reduces decay
}

// DefaultParams returns sensible defaults.
func DefaultParams() Params {
	return Params{
		Lambda:           0.01, // ~1% per day decay base
		AccessBoost:      0.05, // each access adds 5% clarity
		EmotionalProtect: 0.3,  // emotional_weight 1.0 -> 30% slower decay
	}
}

// Clarity computes current clarity from memory state.
func Clarity(m *model.Memory, params Params) float64 {
	age := time.Since(m.CreatedAt).Hours() / 24
	accessBoost := float64(m.AccessCount) * params.AccessBoost
	emotionalProtect := m.EmotionalWeight * params.EmotionalProtect

	decay := math.Exp(-params.Lambda * age * (1 - emotionalProtect))
	base := 1.0 * decay
	clarity := base + accessBoost
	if clarity > 1.0 {
		clarity = 1.0
	}
	if clarity < 0 {
		clarity = 0
	}
	return clarity
}

// ResidualForm returns the degradation stage for given age.
func ResidualForm(ageDays int) model.DecayStage {
	if ageDays < 7 {
		return model.StageFull
	}
	if ageDays < 30 {
		return model.StageSummary
	}
	if ageDays < 90 {
		return model.StageKeywords
	}
	if ageDays < 180 {
		return model.StageFragment
	}
	return model.StageOutline
}

// ResidualFormName returns string name for stage.
func ResidualFormName(s model.DecayStage) string {
	switch s {
	case model.StageFull:
		return "full"
	case model.StageSummary:
		return "summary"
	case model.StageKeywords:
		return "keywords"
	case model.StageFragment:
		return "fragment"
	case model.StageOutline:
		return "outline"
	default:
		return "full"
	}
}
