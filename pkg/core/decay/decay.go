package decay

import (
	"math"
	"time"

	"github.com/hiparker/echo-fade-memory/pkg/core/model"
)

// Params: minimal. τ=halflife (days), α=shape, ε=reinforcement weight.
type Params struct {
	Tau    float64
	Alpha  float64
	Epsilon float64
}

func DefaultParams() Params {
	return Params{Tau: 90, Alpha: 1.5, Epsilon: 0.1}
}

// ParamsFromFullArgs for config compatibility.
type ParamsFromFullArgs struct {
	Tau, Alpha, Epsilon float64
	Lambda, AccessBoost, EmotionalProtect float64
	HorizonDays float64
	DecayMode   string
	ClarityFull, ClaritySummary, ClarityKeywords, ClarityFragment float64
	StageSummary, StageKeywords, StageFragment, StageOutline     int
}

func ParamsFromFull(args ParamsFromFullArgs) Params {
	p := DefaultParams()
	if args.Tau > 0 {
		p.Tau = args.Tau
	} else if args.Lambda > 0 {
		p.Tau = 1 / args.Lambda
	} else if args.HorizonDays > 0 {
		p.Tau = args.HorizonDays / 10
	}
	if args.Alpha > 0 {
		p.Alpha = args.Alpha
	}
	if args.Epsilon > 0 {
		p.Epsilon = args.Epsilon
	} else if args.AccessBoost > 0 {
		p.Epsilon = args.AccessBoost * 2
	}
	return p
}

// Strength returns [0,1]. Single formula, no stages.
func Strength(m *model.Memory, params Params) float64 {
	age := time.Since(m.CreatedAt).Hours() / 24
	tau, alpha, eps := params.Tau, params.Alpha, params.Epsilon
	if tau <= 0 {
		tau = 90
	}
	if alpha <= 0 {
		alpha = 1.5
	}
	decay := 1.0 / (1.0 + math.Pow(age/tau, alpha))
	reinforce := 1.0 + eps*(float64(m.AccessCount)+m.Importance+m.EmotionalWeight)
	if reinforce > 3 {
		reinforce = 3
	}
	s := decay * reinforce
	if s > 1 {
		s = 1
	}
	if s < 0 {
		s = 0
	}
	return s
}

func Clarity(m *model.Memory, params Params) float64 {
	return Strength(m, params)
}

func ResidualFormFromClarity(strength float64, _ Params) model.DecayStage {
	if strength >= 0.8 {
		return model.StageFull
	}
	if strength >= 0.5 {
		return model.StageSummary
	}
	if strength >= 0.2 {
		return model.StageKeywords
	}
	if strength >= 0.05 {
		return model.StageFragment
	}
	return model.StageOutline
}

func ResidualForm(ageDays int, params Params) model.DecayStage {
	m := &model.Memory{CreatedAt: time.Now().Add(-time.Duration(ageDays) * 24 * time.Hour)}
	return ResidualFormFromClarity(Strength(m, params), params)
}

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
