package engine

import (
	"context"
	"math"
	"sort"
	"strings"
	"time"
)

const (
	defaultOverviewWindowDays = 30
	defaultIntegritySample    = 200
	maxIntegritySample        = 1000
)

// TrendPoint is one daily bucket for trend views.
type TrendPoint struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

// OverviewQuality summarizes data quality signals.
type OverviewQuality struct {
	EmptySummaryCount   int     `json:"empty_summary_count"`
	NeedsGroundingCount int     `json:"needs_grounding_count"`
	AvgClarity          float64 `json:"avg_clarity"`
}

// OverviewStats contains aggregate dashboard metrics.
type OverviewStats struct {
	WindowDays           int             `json:"window_days"`
	TotalMemories        int             `json:"total_memories"`
	NewMemoriesWindow    int             `json:"new_memories_window"`
	NewMemoriesToday     int             `json:"new_memories_today"`
	NewMemoriesYesterday int             `json:"new_memories_yesterday"`
	GrowthRateDayOverDay float64         `json:"growth_rate_day_over_day"`
	HighDecayRiskCount   int             `json:"high_decay_risk_count"`
	ByLifecycleState     map[string]int  `json:"by_lifecycle_state"`
	ByDecayStage         map[string]int  `json:"by_decay_stage"`
	ByMemoryType         map[string]int  `json:"by_memory_type"`
	TrendCreatedDaily    []TrendPoint    `json:"trend_created_daily"`
	TopNewMemories       []TopMemoryItem `json:"top_new_memories"`
	TopDecayRiskMemories []TopRiskItem   `json:"top_decay_risk_memories"`
	TopAccessedMemories  []TopAccessItem `json:"top_accessed_memories"`
	Quality              OverviewQuality `json:"quality"`
}

// TopMemoryItem represents a newest-memory entry.
type TopMemoryItem struct {
	ID             string  `json:"id"`
	Summary        string  `json:"summary"`
	MemoryType     string  `json:"memory_type"`
	CreatedAt      string  `json:"created_at"`
	Clarity        float64 `json:"clarity"`
	LifecycleState string  `json:"lifecycle_state"`
}

// TopRiskItem represents a decay-risk entry.
type TopRiskItem struct {
	ID           string  `json:"id"`
	Summary      string  `json:"summary"`
	Clarity      float64 `json:"clarity"`
	LastAccessed string  `json:"last_accessed_at"`
	IdleDays     int     `json:"idle_days"`
	RiskScore    float64 `json:"risk_score"`
	DecayStage   string  `json:"decay_stage"`
}

// TopAccessItem represents a top-accessed entry.
type TopAccessItem struct {
	ID           string  `json:"id"`
	Summary      string  `json:"summary"`
	AccessCount  int     `json:"access_count"`
	MemoryType   string  `json:"memory_type"`
	LastAccessed string  `json:"last_accessed_at"`
	Clarity      float64 `json:"clarity"`
}

// IntegrityStats reports lightweight SQL/vector consistency checks.
type IntegrityStats struct {
	VectorBackend         string `json:"vector_backend"`
	Capability            string `json:"capability"`
	SQLTotal              int    `json:"sql_total"`
	VectorTotal           int    `json:"vector_total"`
	SampleChecked         int    `json:"sample_checked"`
	SampleSize            int    `json:"sample_size"`
	MissingInVector       int    `json:"missing_in_vector"`
	OrphanInVectorSampled int    `json:"orphan_in_vector_sampled"`
	Status                string `json:"status"`
	Message               string `json:"message"`
}

type vectorCountCap interface {
	VectorCount() int
}

type vectorHasIDCap interface {
	HasVectorID(id string) bool
}

// OverviewOptions controls overview aggregation behavior.
type OverviewOptions struct {
	WindowDays    int
	TopK          int
	RiskWClarity  float64
	RiskWIdleDays float64
}

// StatsOverview returns aggregate memory metrics for dashboard overview.
func (e *Engine) StatsOverview(ctx context.Context, windowDays int) (*OverviewStats, error) {
	return e.StatsOverviewWithOptions(ctx, OverviewOptions{WindowDays: windowDays})
}

// StatsOverviewWithOptions returns aggregate dashboard metrics with Top lists.
func (e *Engine) StatsOverviewWithOptions(ctx context.Context, opts OverviewOptions) (*OverviewStats, error) {
	_ = ctx
	windowDays := opts.WindowDays
	if windowDays <= 0 {
		windowDays = defaultOverviewWindowDays
	}
	topK := opts.TopK
	if topK <= 0 {
		topK = 10
	}
	if topK > 100 {
		topK = 100
	}
	wClarity := opts.RiskWClarity
	wIdle := opts.RiskWIdleDays
	if wClarity == 0 && wIdle == 0 {
		wClarity = 0.7
		wIdle = 0.3
	}
	if wClarity < 0 {
		wClarity = 0
	}
	if wIdle < 0 {
		wIdle = 0
	}
	weightSum := wClarity + wIdle
	if weightSum <= 0 {
		wClarity, wIdle = 0.7, 0.3
		weightSum = 1.0
	}
	wClarity /= weightSum
	wIdle /= weightSum

	if windowDays <= 0 {
		windowDays = defaultOverviewWindowDays
	}
	memories, err := e.mem.ListAll()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	start := now.AddDate(0, 0, -windowDays+1)
	byLifecycle := map[string]int{}
	byStage := map[string]int{}
	byType := map[string]int{}
	trendMap := make(map[string]int, windowDays)
	emptySummary := 0
	needsGrounding := 0
	clarityTotal := 0.0
	newWindow := 0
	newToday := 0
	newYesterday := 0
	topNewSource := make([]*TopMemoryItem, 0, len(memories))
	topRiskSource := make([]*TopRiskItem, 0, len(memories))
	topAccessSource := make([]*TopAccessItem, 0, len(memories))

	for _, m := range memories {
		if m == nil {
			continue
		}
		normalizeLoadedMemory(m)
		lifecycle := strings.TrimSpace(m.LifecycleState)
		if lifecycle == "" {
			lifecycle = "unknown"
		}
		byLifecycle[lifecycle]++

		stage := strings.TrimSpace(m.ResidualForm)
		if stage == "" {
			stage = "unknown"
		}
		byStage[stage]++

		mType := strings.TrimSpace(m.MemoryType)
		if mType == "" {
			mType = "unknown"
		}
		byType[mType]++

		if strings.TrimSpace(m.Summary) == "" {
			emptySummary++
		}
		if len(m.SourceRefs) == 0 {
			needsGrounding++
		}
		clarityTotal += m.Clarity
		createdAt := m.CreatedAt
		if createdAt.IsZero() {
			createdAt = now
		}
		if sameDay(createdAt, now) {
			newToday++
		}
		if sameDay(createdAt, now.AddDate(0, 0, -1)) {
			newYesterday++
		}

		if !m.CreatedAt.IsZero() && !m.CreatedAt.Before(start) {
			newWindow++
			day := m.CreatedAt.Format("2006-01-02")
			trendMap[day]++
		}

		topNewSource = append(topNewSource, &TopMemoryItem{
			ID:             m.ID,
			Summary:        recallSummary(m),
			MemoryType:     mType,
			CreatedAt:      createdAt.Format(time.RFC3339),
			Clarity:        m.Clarity,
			LifecycleState: lifecycle,
		})

		idleDays := idleDays(now, m.LastAccessedAt)
		riskScore := ((1 - clamp01(m.Clarity)) * wClarity) + (normalizeIdleDays(idleDays) * wIdle)
		topRiskSource = append(topRiskSource, &TopRiskItem{
			ID:           m.ID,
			Summary:      recallSummary(m),
			Clarity:      m.Clarity,
			LastAccessed: m.LastAccessedAt.Format(time.RFC3339),
			IdleDays:     idleDays,
			RiskScore:    riskScore,
			DecayStage:   stage,
		})

		topAccessSource = append(topAccessSource, &TopAccessItem{
			ID:           m.ID,
			Summary:      recallSummary(m),
			AccessCount:  m.AccessCount,
			MemoryType:   mType,
			LastAccessed: m.LastAccessedAt.Format(time.RFC3339),
			Clarity:      m.Clarity,
		})
	}

	trend := make([]TrendPoint, 0, windowDays)
	for i := 0; i < windowDays; i++ {
		day := start.AddDate(0, 0, i).Format("2006-01-02")
		trend = append(trend, TrendPoint{
			Date:  day,
			Count: trendMap[day],
		})
	}
	sort.Slice(trend, func(i, j int) bool { return trend[i].Date < trend[j].Date })

	avgClarity := 0.0
	if len(memories) > 0 {
		avgClarity = clarityTotal / float64(len(memories))
	}
	growth := 0.0
	if newYesterday > 0 {
		growth = (float64(newToday-newYesterday) / float64(newYesterday)) * 100
	} else if newToday > 0 {
		growth = 100
	}

	sort.Slice(topNewSource, func(i, j int) bool {
		return topNewSource[i].CreatedAt > topNewSource[j].CreatedAt
	})
	sort.Slice(topRiskSource, func(i, j int) bool {
		if topRiskSource[i].RiskScore == topRiskSource[j].RiskScore {
			return topRiskSource[i].LastAccessed < topRiskSource[j].LastAccessed
		}
		return topRiskSource[i].RiskScore > topRiskSource[j].RiskScore
	})
	sort.Slice(topAccessSource, func(i, j int) bool {
		if topAccessSource[i].AccessCount == topAccessSource[j].AccessCount {
			return topAccessSource[i].LastAccessed > topAccessSource[j].LastAccessed
		}
		return topAccessSource[i].AccessCount > topAccessSource[j].AccessCount
	})

	topNew := sliceTopNew(topNewSource, topK)
	topRisk := sliceTopRisk(topRiskSource, topK)
	topAccess := sliceTopAccess(topAccessSource, topK)
	highRiskCount := 0
	for _, item := range topRiskSource {
		if item.RiskScore >= 0.6 {
			highRiskCount++
		}
	}

	return &OverviewStats{
		WindowDays:           windowDays,
		TotalMemories:        len(memories),
		NewMemoriesWindow:    newWindow,
		NewMemoriesToday:     newToday,
		NewMemoriesYesterday: newYesterday,
		GrowthRateDayOverDay: growth,
		HighDecayRiskCount:   highRiskCount,
		ByLifecycleState:     byLifecycle,
		ByDecayStage:         byStage,
		ByMemoryType:         byType,
		TrendCreatedDaily:    trend,
		TopNewMemories:       topNew,
		TopDecayRiskMemories: topRisk,
		TopAccessedMemories:  topAccess,
		Quality: OverviewQuality{
			EmptySummaryCount:   emptySummary,
			NeedsGroundingCount: needsGrounding,
			AvgClarity:          avgClarity,
		},
	}, nil
}

// StatsIntegrity returns lightweight SQL/vector consistency checks.
func (e *Engine) StatsIntegrity(ctx context.Context, sampleSize int) (*IntegrityStats, error) {
	_ = ctx
	if sampleSize <= 0 {
		sampleSize = defaultIntegritySample
	}
	if sampleSize > maxIntegritySample {
		sampleSize = maxIntegritySample
	}

	memories, err := e.mem.ListAll()
	if err != nil {
		return nil, err
	}
	sqlTotal := len(memories)
	vectorTotal := -1
	capability := "count_only"
	message := "vector backend supports count only"

	if c, ok := e.vector.(vectorCountCap); ok {
		vectorTotal = c.VectorCount()
	}

	missingInVector := 0
	sampleChecked := 0
	orphanSampled := 0
	if h, ok := e.vector.(vectorHasIDCap); ok {
		capability = "count_and_sample"
		message = "count and sampled ID checks are available"
		ids := make([]string, 0, len(memories))
		for _, m := range memories {
			if m != nil && m.ID != "" {
				ids = append(ids, m.ID)
			}
		}
		sort.Strings(ids)
		if sampleSize > len(ids) {
			sampleSize = len(ids)
		}
		for i := 0; i < sampleSize; i++ {
			if !h.HasVectorID(ids[i]) {
				missingInVector++
			}
			sampleChecked++
		}
	}

	status := "ok"
	if missingInVector > 0 {
		status = "warn"
	}
	if vectorTotal >= 0 && sqlTotal != vectorTotal {
		status = "warn"
	}

	return &IntegrityStats{
		VectorBackend:         strings.ToLower(strings.TrimSpace(e.cfg.VectorStore.Type)),
		Capability:            capability,
		SQLTotal:              sqlTotal,
		VectorTotal:           vectorTotal,
		SampleChecked:         sampleChecked,
		SampleSize:            sampleSize,
		MissingInVector:       missingInVector,
		OrphanInVectorSampled: orphanSampled,
		Status:                status,
		Message:               message,
	}, nil
}

func sameDay(ts time.Time, ref time.Time) bool {
	y1, m1, d1 := ts.Date()
	y2, m2, d2 := ref.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

func idleDays(now time.Time, lastAccessed time.Time) int {
	if lastAccessed.IsZero() {
		return 365
	}
	hours := now.Sub(lastAccessed).Hours()
	if hours <= 0 {
		return 0
	}
	return int(hours / 24)
}

func normalizeIdleDays(days int) float64 {
	if days <= 0 {
		return 0
	}
	if days >= 30 {
		return 1
	}
	return float64(days) / 30.0
}

func clamp01(v float64) float64 {
	return math.Max(0, math.Min(1, v))
}

func sliceTopNew(in []*TopMemoryItem, topK int) []TopMemoryItem {
	if topK > len(in) {
		topK = len(in)
	}
	out := make([]TopMemoryItem, 0, topK)
	for i := 0; i < topK; i++ {
		out = append(out, *in[i])
	}
	return out
}

func sliceTopRisk(in []*TopRiskItem, topK int) []TopRiskItem {
	if topK > len(in) {
		topK = len(in)
	}
	out := make([]TopRiskItem, 0, topK)
	for i := 0; i < topK; i++ {
		out = append(out, *in[i])
	}
	return out
}

func sliceTopAccess(in []*TopAccessItem, topK int) []TopAccessItem {
	if topK > len(in) {
		topK = len(in)
	}
	out := make([]TopAccessItem, 0, topK)
	for i := 0; i < topK; i++ {
		out = append(out, *in[i])
	}
	return out
}
