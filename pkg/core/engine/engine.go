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
	Rank    int     `json:"rank"`
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
	return NewWithDeps(cfg, mem, vector, bleve, embed), nil
}

// NewWithDeps creates an Engine from explicit dependencies.
// This is useful for tests and alternative runtimes.
func NewWithDeps(
	cfg *config.Config,
	mem memstore.MemoryStore,
	vector store.VectorStore,
	bleve *store.BleveStore,
	embed embedding.Provider,
) *Engine {
	if cfg == nil {
		cfg = config.Default()
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
	}
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
	ConflictGroup  string            `json:"conflict_group,omitempty"`
	Version        int               `json:"version,omitempty"`
	LifecycleState string            `json:"lifecycle_state,omitempty"`
	ConflictWarn   bool              `json:"conflict_warning,omitempty"`
	Suppressed     []int             `json:"suppressed_versions,omitempty"`
}

// RecallTraceCandidate captures accepted or filtered recall candidates.
type RecallTraceCandidate struct {
	MemoryID        string            `json:"memory_id"`
	Summary         string            `json:"summary,omitempty"`
	ConflictGroup   string            `json:"conflict_group,omitempty"`
	Version         int               `json:"version,omitempty"`
	LifecycleState  string            `json:"lifecycle_state,omitempty"`
	Strength        float64           `json:"strength"`
	Freshness       float64           `json:"freshness"`
	NeedsGrounding  bool              `json:"needs_grounding"`
	Source          string            `json:"source,omitempty"`
	SourceRefs      []model.SourceRef `json:"source_refs,omitempty"`
	VectorScore     float64           `json:"vector_score,omitempty"`
	VectorRank      int               `json:"vector_rank,omitempty"`
	BM25Score       float64           `json:"bm25_score,omitempty"`
	BM25Rank        int               `json:"bm25_rank,omitempty"`
	FusedScore      float64           `json:"fused_score"`
	Accepted        bool              `json:"accepted"`
	FilteredReasons []string          `json:"filtered_reasons,omitempty"`
	LatestVersion   int               `json:"latest_version,omitempty"`
}

// ExplainResult exposes accepted and filtered candidates for auditability.
type ExplainResult struct {
	Query    string                 `json:"query"`
	Accepted []RecallResult         `json:"accepted"`
	Filtered []RecallTraceCandidate `json:"filtered"`
}

// Recall performs hybrid recall (vector + BM25, RRF fusion).
func (e *Engine) Recall(ctx context.Context, query string, k int, minClarity float64) ([]RecallResult, error) {
	results, _, err := e.recallWithTrace(ctx, query, k, minClarity, true)
	return results, err
}

// Explain returns accepted and filtered recall candidates without reinforcement side effects.
func (e *Engine) Explain(ctx context.Context, query string, k int, minClarity float64) (*ExplainResult, error) {
	accepted, filtered, err := e.recallWithTrace(ctx, query, k, minClarity, false)
	if err != nil {
		return nil, err
	}
	return &ExplainResult{
		Query:    query,
		Accepted: accepted,
		Filtered: filtered,
	}, nil
}

func (e *Engine) recallWithTrace(ctx context.Context, query string, k int, minClarity float64, reinforce bool) ([]RecallResult, []RecallTraceCandidate, error) {
	if e.decayCacheTTL == 0 || time.Since(e.lastDecayAt) > e.decayCacheTTL {
		_ = e.DecayAll(ctx)
		e.lastDecayAt = time.Now()
	}
	vec, err := e.embed.Embed(ctx, query)
	if err != nil {
		return nil, nil, err
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
		return nil, nil, err
	}

	rrfK := 60.0
	combined := rrfFusionDetailed(vecIDs, vecScores, bm25IDs, bm25Scores, rrfK)
	vecScoreMap := make(map[string]float64, len(vecIDs))
	vecRankMap := make(map[string]int, len(vecIDs))
	for i, id := range vecIDs {
		vecScoreMap[id] = float64(vecScores[i])
		vecRankMap[id] = i + 1
	}
	bm25ScoreMap := make(map[string]float64, len(bm25IDs))
	bm25RankMap := make(map[string]int, len(bm25IDs))
	for i, id := range bm25IDs {
		bm25ScoreMap[id] = bm25Scores[i]
		bm25RankMap[id] = i + 1
	}

	memories, err := e.mem.ListAll()
	if err != nil {
		return nil, nil, err
	}
	conflicts := buildConflictIndex(memories)

	var results []RecallResult
	var filtered []RecallTraceCandidate
	now := time.Now()
	for _, item := range combined {
		m, err := e.mem.Get(item.id)
		if err != nil || m == nil {
			filtered = append(filtered, RecallTraceCandidate{
				MemoryID:        item.id,
				FusedScore:      item.score,
				Accepted:        false,
				FilteredReasons: []string{"load_failed"},
			})
			continue
		}
		if m.LifecycleState == "" {
			m.LifecycleState = model.LifecycleStateFromClarity(m.Clarity)
		}
		trace := buildTraceCandidate(m, item, vecScoreMap, vecRankMap, bm25ScoreMap, bm25RankMap, conflicts, now)
		filterReasons := make([]string, 0, 4)
		if m.Clarity < minClarity {
			filterReasons = append(filterReasons, "below_min_clarity")
		}
		if conflict, ok := conflicts[m.ConflictGroup]; ok && conflict.count > 1 && m.Version < conflict.latestVersion {
			filterReasons = append(filterReasons, "superseded_by_newer_version")
		}
		if len(results) >= k {
			filterReasons = append(filterReasons, "rank_cutoff")
		}
		if len(filterReasons) > 0 {
			trace.Accepted = false
			trace.FilteredReasons = filterReasons
			filtered = append(filtered, trace)
			continue
		}
		if reinforce {
			m.AccessCount++
			_ = e.mem.UpdateAccess(m.ID, m.AccessCount)
		}
		evidence := recallEvidence(m.ID, vecScoreMap, vecRankMap, bm25ScoreMap, bm25RankMap)
		conflict := conflicts[m.ConflictGroup]
		results = append(results, RecallResult{
			Memory:         m,
			MemoryID:       m.ID,
			Summary:        recallSummary(m),
			Score:          item.score,
			Strength:       m.Clarity,
			Freshness:      freshnessScore(m, now),
			Fuzziness:      m.Fuzziness(),
			DecayStage:     m.ResidualForm,
			LastAccessedAt: now,
			NeedsGrounding: needsGrounding(m, evidence, conflict),
			Source:         m.PrimarySource(),
			SourceRefs:     m.SourceRefs,
			WhyRecalled:    whyRecalled(m, evidence, conflict),
			Evidence:       evidence,
			ConflictGroup:  m.ConflictGroup,
			Version:        m.Version,
			LifecycleState: m.LifecycleState,
			ConflictWarn:   conflict.count > 1,
			Suppressed:     suppressedVersions(m.Version, conflict.versions),
		})
	}

	return results, filtered, nil
}

type rankedResult struct {
	id    string
	score float64
}

type conflictInfo struct {
	latestVersion int
	count         int
	versions      []int
}

func rrfFusionDetailed(vecIDs []string, vecScores []float32, bm25IDs []string, bm25Scores []float64, k float64) []rankedResult {
	scores := make(map[string]float64)
	for i, id := range vecIDs {
		scores[id] += 1.0 / (k + float64(i+1))
	}
	for i, id := range bm25IDs {
		scores[id] += 1.0 / (k + float64(i+1))
	}

	items := make([]rankedResult, 0, len(scores))
	for id, s := range scores {
		items = append(items, rankedResult{id: id, score: s})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].score > items[j].score })
	return items
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
	ConflictGroup   string            `json:"conflict_group,omitempty"`
	Version         int               `json:"version,omitempty"`
	LifecycleState  string            `json:"lifecycle_state,omitempty"`
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
		ConflictGroup:   m.ConflictGroup,
		Version:         m.Version,
		LifecycleState:  m.LifecycleState,
	}, nil
}

// Get returns a fully described memory by id.
func (e *Engine) Get(ctx context.Context, id string) (*GroundResult, error) {
	return e.Ground(ctx, id)
}

// Versions returns all versions in the memory's conflict group.
func (e *Engine) Versions(ctx context.Context, id string) ([]GroundResult, error) {
	current, err := e.mem.Get(id)
	if err != nil || current == nil {
		return nil, err
	}
	memories, err := e.mem.ListAll()
	if err != nil {
		return nil, err
	}
	results := make([]GroundResult, 0)
	for _, m := range memories {
		if m.ConflictGroup != current.ConflictGroup {
			continue
		}
		results = append(results, GroundResult{
			MemoryID:        m.ID,
			Summary:         recallSummary(m),
			Content:         m.Content,
			ResidualContent: m.ResidualContent,
			Strength:        m.Clarity,
			DecayStage:      m.ResidualForm,
			Source:          m.PrimarySource(),
			SourceRefs:      m.SourceRefs,
			GroundingStatus: m.GroundingStatus,
			ConflictGroup:   m.ConflictGroup,
			Version:         m.Version,
			LifecycleState:  m.LifecycleState,
		})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Version > results[j].Version })
	return results, nil
}

// DecayAll recomputes clarity and residual for all memories.
func (e *Engine) DecayAll(ctx context.Context) error {
	memories, err := e.mem.ListAll()
	if err != nil {
		return err
	}
	if len(memories) == 0 {
		return nil
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

func recallEvidence(id string, vecScoreMap map[string]float64, vecRankMap map[string]int, bm25ScoreMap map[string]float64, bm25RankMap map[string]int) []RecallEvidence {
	evidence := make([]RecallEvidence, 0, 2)
	if score, ok := vecScoreMap[id]; ok {
		evidence = append(evidence, RecallEvidence{Backend: "vector", Score: score, Rank: vecRankMap[id]})
	}
	if score, ok := bm25ScoreMap[id]; ok {
		evidence = append(evidence, RecallEvidence{Backend: "bm25", Score: score, Rank: bm25RankMap[id]})
	}
	return evidence
}

func needsGrounding(m *model.Memory, evidence []RecallEvidence, conflict conflictInfo) bool {
	if len(m.SourceRefs) == 0 {
		return true
	}
	if conflict.count > 1 {
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

func whyRecalled(m *model.Memory, evidence []RecallEvidence, conflict conflictInfo) []string {
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
	if conflict.count > 1 {
		reasons = append(reasons, "latest_in_conflict_group")
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

func buildConflictIndex(memories []*model.Memory) map[string]conflictInfo {
	index := make(map[string]conflictInfo)
	for _, m := range memories {
		if m == nil || m.ConflictGroup == "" {
			continue
		}
		info := index[m.ConflictGroup]
		info.count++
		if m.Version > info.latestVersion {
			info.latestVersion = m.Version
		}
		info.versions = append(info.versions, m.Version)
		index[m.ConflictGroup] = info
	}
	for key, info := range index {
		sort.Ints(info.versions)
		index[key] = info
	}
	return index
}

func suppressedVersions(current int, versions []int) []int {
	if len(versions) <= 1 {
		return nil
	}
	out := make([]int, 0, len(versions)-1)
	for _, v := range versions {
		if v != current {
			out = append(out, v)
		}
	}
	return out
}

func buildTraceCandidate(
	m *model.Memory,
	item rankedResult,
	vecScoreMap map[string]float64,
	vecRankMap map[string]int,
	bm25ScoreMap map[string]float64,
	bm25RankMap map[string]int,
	conflicts map[string]conflictInfo,
	now time.Time,
) RecallTraceCandidate {
	conflict := conflicts[m.ConflictGroup]
	return RecallTraceCandidate{
		MemoryID:       m.ID,
		Summary:        recallSummary(m),
		ConflictGroup:  m.ConflictGroup,
		Version:        m.Version,
		LifecycleState: m.LifecycleState,
		Strength:       m.Clarity,
		Freshness:      freshnessScore(m, now),
		NeedsGrounding: needsGrounding(m, recallEvidence(m.ID, vecScoreMap, vecRankMap, bm25ScoreMap, bm25RankMap), conflict),
		Source:         m.PrimarySource(),
		SourceRefs:     m.SourceRefs,
		VectorScore:    vecScoreMap[m.ID],
		VectorRank:     vecRankMap[m.ID],
		BM25Score:      bm25ScoreMap[m.ID],
		BM25Rank:       bm25RankMap[m.ID],
		FusedScore:     item.score,
		Accepted:       true,
		LatestVersion:  conflict.latestVersion,
	}
}
