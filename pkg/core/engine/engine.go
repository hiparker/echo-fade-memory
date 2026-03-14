package engine

import (
	"context"
	"fmt"
	"hash/fnv"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/echo-fade-memory/echo-fade-memory/pkg/basic/util/safe"
	"github.com/echo-fade-memory/echo-fade-memory/pkg/config"
	"github.com/echo-fade-memory/echo-fade-memory/pkg/core/decay"
	"github.com/echo-fade-memory/echo-fade-memory/pkg/core/model"
	"github.com/echo-fade-memory/echo-fade-memory/pkg/core/transform"
	"github.com/echo-fade-memory/echo-fade-memory/pkg/port/embedding"
	"github.com/echo-fade-memory/echo-fade-memory/pkg/port/memstore"
	"github.com/echo-fade-memory/echo-fade-memory/pkg/port/store"
	"github.com/echo-fade-memory/echo-fade-memory/pkg/port/storefactory"
	"github.com/google/uuid"
)

// Engine is the core memory engine.
type Engine struct {
	cfg           *config.Config
	mem           memstore.MemoryStore
	vector        store.VectorStore
	bleve         *store.BleveStore
	embed         embedding.Provider
	decay         decay.Params
	mu            sync.RWMutex
	lastDecayAt   time.Time
	decayCacheTTL time.Duration
}

// RememberRequest captures middleware-facing memory metadata.
type RememberRequest struct {
	Content       string
	Summary       string
	Importance    float64
	MemoryType    string
	SourceRefs    []model.SourceRef
	ConflictGroup string
}

// RecallEvidence explains which backend contributed to a hit.
type RecallEvidence struct {
	Backend string  `json:"backend"`
	Score   float64 `json:"score"`
}

// New creates a new Engine.
func New(cfg *config.Config) (*Engine, error) {
	if err := os.MkdirAll(cfg.DataPath, 0755); err != nil {
		return nil, err
	}

	mem, err := storefactory.NewMemoryStore(cfg)
	if err != nil {
		return nil, err
	}

	vector, err := storefactory.NewVectorStore(cfg)
	if err != nil {
		mem.Close()
		return nil, err
	}

	bleve, err := store.OpenOrCreateBleve(cfg.BlevePath())
	if err != nil {
		mem.Close()
		return nil, err
	}

	embed, err := embedding.NewProvider(cfg)
	if err != nil {
		mem.Close()
		return nil, err
	}
	decayParams := decay.ParamsFromFull(decay.ParamsFromFullArgs{
		Tau:         cfg.Decay.Tau,
		Alpha:       cfg.Decay.Alpha,
		Epsilon:     cfg.Decay.Epsilon,
		Lambda:      cfg.Decay.Lambda,
		AccessBoost: cfg.Decay.AccessBoost,
		HorizonDays: cfg.Decay.HorizonDays,
	})

	decayCacheTTL := time.Duration(cfg.Decay.CacheTTLMin) * time.Minute
	return &Engine{
		cfg:           cfg,
		mem:           mem,
		vector:        vector,
		bleve:         bleve,
		embed:         embed,
		decay:         decayParams,
		decayCacheTTL: decayCacheTTL,
	}, nil
}

// Store adds a new memory.
func (e *Engine) Store(ctx context.Context, content string, importance float64) (*model.Memory, error) {
	return e.Remember(ctx, RememberRequest{
		Content:    content,
		Importance: importance,
		MemoryType: model.MemoryTypeLongTerm,
	})
}

// Remember adds a new memory with richer metadata.
func (e *Engine) Remember(ctx context.Context, req RememberRequest) (*model.Memory, error) {
	content := req.Content
	vec, err := e.embed.Embed(ctx, content)
	if err != nil {
		return nil, err
	}

	summary := req.Summary
	if summary == "" {
		summary = transform.Summarize(content, 200)
	}
	memoryType := req.MemoryType
	if memoryType == "" {
		memoryType = model.MemoryTypeLongTerm
	}
	conflictGroup := req.ConflictGroup
	if conflictGroup == "" {
		conflictGroup = deriveConflictGroup(memoryType, summary, req.SourceRefs)
	}
	version, err := e.nextVersion(conflictGroup)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	m := &model.Memory{
		ID:              uuid.New().String(),
		Content:         content,
		Summary:         summary,
		MemoryType:      memoryType,
		LifecycleState:  model.LifecycleFresh,
		SourceRefs:      req.SourceRefs,
		GroundingStatus: groundingStatus(req.SourceRefs),
		ConflictGroup:   conflictGroup,
		Version:         version,
		Embedding:       vec,
		Importance:      req.Importance,
		EmotionalWeight: 0,
		Clarity:         1.0,
		ResidualForm:    "full",
		ResidualContent: content,
		CreatedAt:       now,
		LastAccessedAt:  now,
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// Strong consistency: sequential writes, rollback on any failure
	if err := e.mem.Save(m); err != nil {
		return nil, err
	}
	if err := e.vector.Add(m.ID, vec); err != nil {
		_ = e.mem.Delete(m.ID)
		return nil, err
	}
	if err := e.bleve.Index(m.ID, content); err != nil {
		_ = e.mem.Delete(m.ID)
		_ = e.vector.Remove(m.ID)
		return nil, err
	}
	return m, nil
}

// RecallResult holds a recalled memory with score.
type RecallResult struct {
	Memory         *model.Memory     `json:"-"`
	MemoryID       string            `json:"memory_id"`
	Summary        string            `json:"summary"`
	Score          float64           `json:"score"`
	Strength       float64           `json:"strength"`
	Freshness      float64           `json:"freshness"`
	Fuzziness      float64           `json:"fuzziness"`
	DecayStage     string            `json:"decay_stage"`
	LastAccessedAt time.Time         `json:"last_accessed_at"`
	NeedsGrounding bool              `json:"needs_grounding"`
	Source         string            `json:"source,omitempty"`
	SourceRefs     []model.SourceRef `json:"source_refs,omitempty"`
	WhyRecalled    []string          `json:"why_recalled,omitempty"`
	Evidence       []RecallEvidence  `json:"evidence,omitempty"`
}

// Recall performs hybrid recall (vector + BM25, RRF fusion).
func (e *Engine) Recall(ctx context.Context, query string, k int, minClarity float64) ([]RecallResult, error) {
	if e.decayCacheTTL == 0 || time.Since(e.lastDecayAt) > e.decayCacheTTL {
		_ = e.DecayAll(ctx)
		e.lastDecayAt = time.Now()
	}
	vec, err := e.embed.Embed(ctx, query)
	if err != nil {
		return nil, err
	}

	var vecIDs []string
	var vecScores []float32
	var bm25IDs []string
	var bm25Scores []float64
	g, gctx := safe.WithContext(ctx)
	g.Go(func() error {
		ids, scores, err := e.vector.Search(gctx, vec, k*2)
		if err != nil {
			return err
		}
		vecIDs, vecScores = ids, scores
		return nil
	})
	g.Go(func() error {
		ids, scores, err := e.bleve.Search(gctx, query, k*2)
		if err != nil {
			return err
		}
		bm25IDs, bm25Scores = ids, scores
		return nil
	})
	if err := g.Wait(); err != nil {
		return nil, err
	}

	rrfK := 60.0
	combined := rrfFusion(vecIDs, vecScores, bm25IDs, bm25Scores, rrfK)
	vecScoreMap := make(map[string]float64, len(vecIDs))
	for i, id := range vecIDs {
		vecScoreMap[id] = float64(vecScores[i])
	}
	bm25ScoreMap := make(map[string]float64, len(bm25IDs))
	for i, id := range bm25IDs {
		bm25ScoreMap[id] = bm25Scores[i]
	}

	var results []RecallResult
	for i, id := range combined {
		if i >= k {
			break
		}
		m, err := e.mem.Get(id)
		if err != nil || m == nil {
			continue
		}
		if m.Clarity < minClarity {
			continue
		}
		if m.LifecycleState == "" {
			m.LifecycleState = model.LifecycleStateFromClarity(m.Clarity)
		}
		m.AccessCount++
		_ = e.mem.UpdateAccess(id, m.AccessCount)
		score := 1.0 / float64(i+1+60)
		evidence := recallEvidence(id, vecScoreMap, bm25ScoreMap)
		results = append(results, RecallResult{
			Memory:         m,
			MemoryID:       m.ID,
			Summary:        recallSummary(m),
			Score:          score,
			Strength:       m.Clarity,
			Freshness:      freshnessScore(m, time.Now()),
			Fuzziness:      m.Fuzziness(),
			DecayStage:     m.ResidualForm,
			LastAccessedAt: time.Now(),
			NeedsGrounding: needsGrounding(m, evidence),
			Source:         m.PrimarySource(),
			SourceRefs:     m.SourceRefs,
			WhyRecalled:    whyRecalled(m, evidence),
			Evidence:       evidence,
		})
	}

	return results, nil
}

func rrfFusion(vecIDs []string, vecScores []float32, bm25IDs []string, bm25Scores []float64, k float64) []string {
	scores := make(map[string]float64)
	for i, id := range vecIDs {
		scores[id] += 1.0 / (k + float64(i+1))
	}
	for i, id := range bm25IDs {
		scores[id] += 1.0 / (k + float64(i+1))
	}

	type item struct {
		id    string
		score float64
	}
	items := make([]item, 0, len(scores))
	for id, s := range scores {
		items = append(items, item{id, s})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].score > items[j].score })
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.id
	}
	return out
}

// Reinforce strengthens a memory by updating its access stats.
func (e *Engine) Reinforce(ctx context.Context, id string) (*model.Memory, error) {
	m, err := e.mem.Get(id)
	if err != nil || m == nil {
		return m, err
	}
	m.AccessCount++
	m.LastAccessedAt = time.Now()
	m.LifecycleState = model.LifecycleReinforced
	if err := e.mem.UpdateAccess(id, m.AccessCount); err != nil {
		return nil, err
	}
	return m, nil
}

// Forget deletes a memory across metadata, vector, and BM25 stores.
func (e *Engine) Forget(ctx context.Context, id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.mem.Delete(id); err != nil {
		return err
	}
	if err := e.vector.Remove(id); err != nil {
		return err
	}
	return e.bleve.Delete(id)
}

// GroundResult returns the memory together with its source references.
type GroundResult struct {
	MemoryID        string            `json:"memory_id"`
	Summary         string            `json:"summary"`
	Content         string            `json:"content"`
	ResidualContent string            `json:"residual_content"`
	Strength        float64           `json:"strength"`
	DecayStage      string            `json:"decay_stage"`
	Source          string            `json:"source,omitempty"`
	SourceRefs      []model.SourceRef `json:"source_refs,omitempty"`
	GroundingStatus string            `json:"grounding_status,omitempty"`
}

// Ground loads the original memory and its provenance.
func (e *Engine) Ground(ctx context.Context, id string) (*GroundResult, error) {
	m, err := e.mem.Get(id)
	if err != nil || m == nil {
		return nil, err
	}
	return &GroundResult{
		MemoryID:        m.ID,
		Summary:         recallSummary(m),
		Content:         m.Content,
		ResidualContent: m.ResidualContent,
		Strength:        m.Clarity,
		DecayStage:      m.ResidualForm,
		Source:          m.PrimarySource(),
		SourceRefs:      m.SourceRefs,
		GroundingStatus: m.GroundingStatus,
	}, nil
}

// DecayAll recomputes clarity and residual for all memories.
func (e *Engine) DecayAll(ctx context.Context) error {
	memories, err := e.mem.ListAll()
	if err != nil {
		return err
	}
	updates := make([]memstore.DecayUpdate, len(memories))
	workers := runtime.NumCPU()
	if workers < 1 {
		workers = 1
	}
	if workers > len(memories) {
		workers = len(memories)
	}
	chunk := (len(memories) + workers - 1) / workers
	g, _ := safe.WithContext(ctx)
	for i := 0; i < workers; i++ {
		start := i * chunk
		end := start + chunk
		if end > len(memories) {
			end = len(memories)
		}
		if start >= end {
			continue
		}
		ms := memories[start:end]
		base := start
		params := e.decay
		g.Go(func() error {
			for j, m := range ms {
				strength := decay.Strength(m, params)
				stage := decay.ResidualFormFromClarity(strength, params)
				residualForm := decay.ResidualFormName(stage)
				residualContent := transform.ToResidual(m.Content, stage)
				updates[base+j] = memstore.DecayUpdate{
					ID:              m.ID,
					Clarity:         strength,
					LifecycleState:  model.LifecycleStateFromClarity(strength),
					ResidualForm:    residualForm,
					ResidualContent: residualContent,
				}
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	return e.mem.UpdateDecayBatch(updates)
}

// Close closes all stores.
func (e *Engine) Close() error {
	_ = e.mem.Close()
	if closer, ok := e.vector.(interface{ Close() error }); ok {
		_ = closer.Close()
	}
	_ = e.bleve.Close()
	return nil
}

func recallSummary(m *model.Memory) string {
	switch {
	case m.Summary != "":
		return m.Summary
	case m.ResidualContent != "":
		return m.ResidualContent
	default:
		return transform.Summarize(m.Content, 200)
	}
}

func freshnessScore(m *model.Memory, now time.Time) float64 {
	ageDays := now.Sub(m.CreatedAt).Hours() / 24
	if ageDays <= 0 {
		return 1
	}
	recentDays := now.Sub(m.LastAccessedAt).Hours() / 24
	if recentDays < 0 {
		recentDays = 0
	}
	ageFactor := 1.0 / (1.0 + ageDays/30.0)
	recentFactor := 1.0 / (1.0 + recentDays/14.0)
	score := 0.6*ageFactor + 0.4*recentFactor
	if score > 1 {
		return 1
	}
	if score < 0 {
		return 0
	}
	return score
}

func recallEvidence(id string, vecScoreMap, bm25ScoreMap map[string]float64) []RecallEvidence {
	evidence := make([]RecallEvidence, 0, 2)
	if score, ok := vecScoreMap[id]; ok {
		evidence = append(evidence, RecallEvidence{Backend: "vector", Score: score})
	}
	if score, ok := bm25ScoreMap[id]; ok {
		evidence = append(evidence, RecallEvidence{Backend: "bm25", Score: score})
	}
	return evidence
}

func needsGrounding(m *model.Memory, evidence []RecallEvidence) bool {
	if len(m.SourceRefs) == 0 {
		return true
	}
	if m.Clarity < 0.5 {
		return true
	}
	if m.ResidualForm == "fragment" || m.ResidualForm == "outline" {
		return true
	}
	return len(evidence) < 2
}

func whyRecalled(m *model.Memory, evidence []RecallEvidence) []string {
	reasons := make([]string, 0, 4)
	for _, ev := range evidence {
		switch ev.Backend {
		case "vector":
			reasons = append(reasons, "semantic_match")
		case "bm25":
			reasons = append(reasons, "keyword_match")
		}
	}
	if m.AccessCount > 1 {
		reasons = append(reasons, "recently_reinforced")
	}
	if m.Importance >= 0.75 {
		reasons = append(reasons, "high_importance")
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "rrf_ranked_match")
	}
	return reasons
}

func groundingStatus(sourceRefs []model.SourceRef) string {
	if len(sourceRefs) == 0 {
		return "derived"
	}
	return "grounded"
}

func deriveConflictGroup(memoryType, summary string, sourceRefs []model.SourceRef) string {
	base := summary
	if len(sourceRefs) > 0 {
		base = sourceRefs[0].Ref
	}
	base = strings.ToLower(strings.TrimSpace(base))
	if base == "" {
		base = memoryType
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(base))
	return fmt.Sprintf("%s:%x", memoryType, h.Sum64())
}

func (e *Engine) nextVersion(conflictGroup string) (int, error) {
	if conflictGroup == "" {
		return 1, nil
	}
	memories, err := e.mem.ListAll()
	if err != nil {
		return 0, err
	}
	maxVersion := 0
	for _, m := range memories {
		if m.ConflictGroup == conflictGroup && m.Version > maxVersion {
			maxVersion = m.Version
		}
	}
	return maxVersion + 1, nil
}
