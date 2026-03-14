package transform

import (
	"strings"

	"github.com/echo-fade-memory/echo-fade-memory/internal/model"
)

// Summarize produces a short summary (simplified: first N chars for MVP).
func Summarize(text string, maxLen int) string {
	text = strings.TrimSpace(text)
	if len(text) <= maxLen {
		return text
	}
	// Simple truncation for MVP; could use LLM later
	runes := []rune(text)
	if len(runes) <= maxLen {
		return string(runes)
	}
	return string(runes[:maxLen]) + "..."
}

// ExtractKeywords extracts keywords (simplified: split by space, take first 10).
func ExtractKeywords(text string, max int) string {
	words := strings.Fields(strings.ToLower(text))
	seen := make(map[string]bool)
	var out []string
	for _, w := range words {
		if len(w) < 2 {
			continue
		}
		if seen[w] {
			continue
		}
		seen[w] = true
		out = append(out, w)
		if len(out) >= max {
			break
		}
	}
	return strings.Join(out, " ")
}

// ToResidual produces residual content based on stage.
func ToResidual(content string, stage model.DecayStage) string {
	switch stage {
	case model.StageFull:
		return content
	case model.StageSummary:
		return Summarize(content, 200)
	case model.StageKeywords:
		return ExtractKeywords(content, 15)
	case model.StageFragment:
		return ExtractKeywords(content, 5)
	case model.StageOutline:
		return ExtractKeywords(content, 3)
	default:
		return content
	}
}
