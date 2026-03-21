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

	"github.com/google/uuid"
	"github.com/hiparker/echo-fade-memory/pkg/basic/util/safe"
	"github.com/hiparker/echo-fade-memory/pkg/config"
	"github.com/hiparker/echo-fade-memory/pkg/core/decay"
	"github.com/hiparker/echo-fade-memory/pkg/core/entity"
	"github.com/hiparker/echo-fade-memory/pkg/core/model"
	"github.com/hiparker/echo-fade-memory/pkg/core/transform"
	"github.com/hiparker/echo-fade-memory/pkg/port/embedding"
	"github.com/hiparker/echo-fade-memory/pkg/port/imageproc"
	"github.com/hiparker/echo-fade-memory/pkg/port/imageproc/basic"
	"github.com/hiparker/echo-fade-memory/pkg/port/imagestore"
	"github.com/hiparker/echo-fade-memory/pkg/port/kgstore"
	"github.com/hiparker/echo-fade-memory/pkg/port/memstore"
	"github.com/hiparker/echo-fade-memory/pkg/port/store"
	"github.com/hiparker/echo-fade-memory/pkg/port/storefactory"
)

// Engine is the core memory engine.
type Engine struct {
	cfg           *config.Config
	mem           memstore.MemoryStore
	vector        store.VectorStore
	bleve         *store.BleveStore
	embed         embedding.Provider
	kg            kgstore.Store
	images        imagestore.Store
	imageVector   store.VectorStore
	imageBleve    *store.BleveStore
	imageAnalyzer imageproc.Analyzer
	decay         decay.Params
	mu            sync.RWMutex
	decayMu       sync.Mutex
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

	kg, err := storefactory.NewKGStore(cfg)
	if err != nil {
		mem.Close()
		if closer, ok := vector.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
		_ = bleve.Close()
		return nil, err
	}

	images, err := storefactory.NewImageStore(cfg)
	if err != nil {
		mem.Close()
		if closer, ok := vector.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
		_ = bleve.Close()
		_ = kg.Close()
		return nil, err
	}
	imageVector, err := storefactory.NewImageVectorStore(cfg)
	if err != nil {
		mem.Close()
		if closer, ok := vector.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
		_ = bleve.Close()
		_ = kg.Close()
		_ = images.Close()
		return nil, err
	}
	imageBleve, err := store.OpenOrCreateBleve(cfg.ImageBlevePath())
	if err != nil {
		mem.Close()
		if closer, ok := vector.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
		if closer, ok := imageVector.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
		_ = bleve.Close()
		_ = kg.Close()
		_ = images.Close()
		return nil, err
	}
	return NewWithDepsFull(cfg, mem, vector, bleve, embed, kg, images, imageVector, imageBleve, basic.New()), nil
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
	return NewWithDepsAndKG(cfg, mem, vector, bleve, embed, nil)
}

// NewWithDepsAndKG creates an Engine with an optional graph store dependency.
func NewWithDepsAndKG(
	cfg *config.Config,
	mem memstore.MemoryStore,
	vector store.VectorStore,
	bleve *store.BleveStore,
	embed embedding.Provider,
	kg kgstore.Store,
) *Engine {
	return NewWithDepsFull(cfg, mem, vector, bleve, embed, kg, nil, nil, nil, nil)
}

// NewWithDepsFull creates an Engine with optional graph and image runtime dependencies.
func NewWithDepsFull(
	cfg *config.Config,
	mem memstore.MemoryStore,
	vector store.VectorStore,
	bleve *store.BleveStore,
	embed embedding.Provider,
	kg kgstore.Store,
	images imagestore.Store,
	imageVector store.VectorStore,
	imageBleve *store.BleveStore,
	imageAnalyzer imageproc.Analyzer,
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
		kg:            kg,
		images:        images,
		imageVector:   imageVector,
		imageBleve:    imageBleve,
		imageAnalyzer: imageAnalyzer,
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
	e.syncMemoryGraph(m)
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
	SourceRefs      []model.SourceRef `json:"source_refs,omitempty"`
	VectorScore     float64           `json:"vector_score,omitempty"`
	VectorRank      int               `json:"vector_rank,omitempty"`
	BM25Score       float64           `json:"bm25_score,omitempty"`
	BM25Rank        int               `json:"bm25_rank,omitempty"`
	KGScore         float64           `json:"kg_score,omitempty"`
	KGRank          int               `json:"kg_rank,omitempty"`
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

// Recall performs hybrid recall (vector + BM25 + KG, RRF fusion).
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

func (e *Engine) maybeDecay(ctx context.Context) {
	e.decayMu.Lock()
	defer e.decayMu.Unlock()
	if e.decayCacheTTL > 0 && time.Since(e.lastDecayAt) <= e.decayCacheTTL {
		return
	}
	if err := e.DecayAll(ctx); err == nil {
		e.lastDecayAt = time.Now()
	}
}

func (e *Engine) recallWithTrace(ctx context.Context, query string, k int, minClarity float64, reinforce bool) ([]RecallResult, []RecallTraceCandidate, error) {
	e.maybeDecay(ctx)
	vec, err := e.embed.Embed(ctx, query)
	if err != nil {
		return nil, nil, err
	}

	var vecIDs []string
	var vecScores []float32
	var bm25IDs []string
	var bm25Scores []float64
	var kgIDs []string
	var kgScores []float64
	candidateK := maxRecallCandidates(k)
	g, gctx := safe.WithContext(ctx)
	g.Go(func() error {
		ids, scores, err := e.vector.Search(gctx, vec, candidateK)
		if err != nil {
			return err
		}
		vecIDs, vecScores = ids, scores
		return nil
	})
	g.Go(func() error {
		ids, scores, err := e.bleve.Search(gctx, query, candidateK)
		if err != nil {
			return err
		}
		bm25IDs, bm25Scores = ids, scores
		return nil
	})
	if e.kg != nil {
		g.Go(func() error {
			ids, scores, err := e.kgRecall(gctx, query, candidateK)
			if err != nil {
				return err
			}
			kgIDs, kgScores = ids, scores
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, nil, err
	}

	rrfK := 60.0
	combined := rrfFusionDetailed(rrfK, vecIDs, bm25IDs, kgIDs)
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
	kgScoreMap := make(map[string]float64, len(kgIDs))
	kgRankMap := make(map[string]int, len(kgIDs))
	for i, id := range kgIDs {
		kgScoreMap[id] = kgScores[i]
		kgRankMap[id] = i + 1
	}

	var results []RecallResult
	var filtered []RecallTraceCandidate
	conflicts := make(map[string]conflictInfo)
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
		normalizeLoadedMemory(m)
		conflict, err := e.conflictInfo(m.ConflictGroup, conflicts)
		if err != nil {
			return nil, nil, err
		}
		trace := buildTraceCandidate(m, item, vecScoreMap, vecRankMap, bm25ScoreMap, bm25RankMap, kgScoreMap, kgRankMap, conflicts, now)
		filterReasons := make([]string, 0, 4)
		if m.Clarity < minClarity {
			filterReasons = append(filterReasons, "below_min_clarity")
		}
		if conflict.count > 1 && m.Version < conflict.latestVersion {
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
		evidence := recallEvidence(m.ID, vecScoreMap, vecRankMap, bm25ScoreMap, bm25RankMap, kgScoreMap, kgRankMap)
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

func rrfFusionDetailed(k float64, lists ...[]string) []rankedResult {
	scores := make(map[string]float64)
	for _, ids := range lists {
		for i, id := range ids {
			scores[id] += 1.0 / (k + float64(i+1))
		}
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
	normalizeLoadedMemory(m)
	return &GroundResult{
		MemoryID:        m.ID,
		Summary:         recallSummary(m),
		Content:         m.Content,
		ResidualContent: m.ResidualContent,
		Strength:        m.Clarity,
		DecayStage:      m.ResidualForm,
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
	memories, err := e.mem.ListByConflictGroup(current.ConflictGroup)
	if err != nil {
		return nil, err
	}
	results := make([]GroundResult, 0)
	for _, m := range memories {
		normalizeLoadedMemory(m)
		results = append(results, GroundResult{
			MemoryID:        m.ID,
			Summary:         recallSummary(m),
			Content:         m.Content,
			ResidualContent: m.ResidualContent,
			Strength:        m.Clarity,
			DecayStage:      m.ResidualForm,
			SourceRefs:      m.SourceRefs,
			GroundingStatus: m.GroundingStatus,
			ConflictGroup:   m.ConflictGroup,
			Version:         m.Version,
			LifecycleState:  m.LifecycleState,
		})
	}
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
	if err := e.mem.UpdateDecayBatch(updates); err != nil {
		return err
	}
	for i, update := range updates {
		memories[i].Clarity = update.Clarity
		memories[i].LifecycleState = update.LifecycleState
		memories[i].ResidualForm = update.ResidualForm
		memories[i].ResidualContent = update.ResidualContent
		e.syncMemoryGraph(memories[i])
	}
	return nil
}

// Close closes all stores.
func (e *Engine) Close() error {
	_ = e.mem.Close()
	if closer, ok := e.vector.(interface{ Close() error }); ok {
		_ = closer.Close()
	}
	_ = e.bleve.Close()
	if e.kg != nil {
		_ = e.kg.Close()
	}
	if e.images != nil {
		_ = e.images.Close()
	}
	if closer, ok := e.imageVector.(interface{ Close() error }); ok {
		_ = closer.Close()
	}
	if e.imageBleve != nil {
		_ = e.imageBleve.Close()
	}
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

func recallEvidence(id string, vecScoreMap map[string]float64, vecRankMap map[string]int, bm25ScoreMap map[string]float64, bm25RankMap map[string]int, kgScoreMap map[string]float64, kgRankMap map[string]int) []RecallEvidence {
	evidence := make([]RecallEvidence, 0, 3)
	if score, ok := vecScoreMap[id]; ok {
		evidence = append(evidence, RecallEvidence{Backend: "vector", Score: score, Rank: vecRankMap[id]})
	}
	if score, ok := bm25ScoreMap[id]; ok {
		evidence = append(evidence, RecallEvidence{Backend: "bm25", Score: score, Rank: bm25RankMap[id]})
	}
	if score, ok := kgScoreMap[id]; ok {
		evidence = append(evidence, RecallEvidence{Backend: "kg", Score: score, Rank: kgRankMap[id]})
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
		case "kg":
			reasons = append(reasons, "entity_match")
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
	latest, err := e.mem.GetLatestByConflictGroup(conflictGroup)
	if err != nil {
		return 0, err
	}
	if latest == nil {
		return 1, nil
	}
	return latest.Version + 1, nil
}

func (e *Engine) syncMemoryGraph(memory *model.Memory) {
	if e.kg == nil || memory == nil {
		return
	}
	extraction := entity.ExtractMemoryGraph(memory)
	for _, ent := range extraction.Entities {
		entityValue := ent
		aliases := aliasesForEntity(ent.ID, extraction.Aliases)
		if err := e.kg.UpsertEntity(&entityValue, aliases); err != nil {
			return
		}
	}
	for _, rel := range extraction.Relations {
		relationValue := rel
		if err := e.kg.UpsertRelation(&relationValue); err != nil {
			return
		}
	}
	_ = e.kg.ReplaceMemoryEntityLinks(memory.ID, extraction.Links)
}

func aliasesForEntity(entityID string, aliases []model.EntityAlias) []string {
	out := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		if alias.EntityID == entityID {
			out = append(out, alias.Alias)
		}
	}
	return out
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
	kgScoreMap map[string]float64,
	kgRankMap map[string]int,
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
		NeedsGrounding: needsGrounding(m, recallEvidence(m.ID, vecScoreMap, vecRankMap, bm25ScoreMap, bm25RankMap, kgScoreMap, kgRankMap), conflict),
		SourceRefs:     m.SourceRefs,
		VectorScore:    vecScoreMap[m.ID],
		VectorRank:     vecRankMap[m.ID],
		BM25Score:      bm25ScoreMap[m.ID],
		BM25Rank:       bm25RankMap[m.ID],
		KGScore:        kgScoreMap[m.ID],
		KGRank:         kgRankMap[m.ID],
		FusedScore:     item.score,
		Accepted:       true,
		LatestVersion:  conflict.latestVersion,
	}
}

func (e *Engine) conflictInfo(conflictGroup string, cache map[string]conflictInfo) (conflictInfo, error) {
	if conflictGroup == "" {
		return conflictInfo{}, nil
	}
	if info, ok := cache[conflictGroup]; ok {
		return info, nil
	}
	memories, err := e.mem.ListByConflictGroup(conflictGroup)
	if err != nil {
		return conflictInfo{}, err
	}
	info := buildConflictInfo(memories)
	cache[conflictGroup] = info
	return info, nil
}

func buildConflictInfo(memories []*model.Memory) conflictInfo {
	info := conflictInfo{}
	for _, m := range memories {
		if m == nil {
			continue
		}
		info.count++
		if m.Version > info.latestVersion {
			info.latestVersion = m.Version
		}
		info.versions = append(info.versions, m.Version)
	}
	sort.Ints(info.versions)
	return info
}

func maxRecallCandidates(k int) int {
	if k <= 0 {
		return 10
	}
	if k < 5 {
		return 10
	}
	return k * 2
}

func (e *Engine) kgRecall(ctx context.Context, query string, limit int) ([]string, []float64, error) {
	if e.kg == nil {
		return nil, nil, nil
	}
	if limit <= 0 {
		limit = 10
	}
	terms := entity.QueryTerms(query)
	if len(terms) == 0 {
		return nil, nil, nil
	}

	scores := make(map[string]float64)
	for termIdx, term := range terms {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		default:
		}

		entities, err := e.kg.FindEntities(term, limit)
		if err != nil {
			return nil, nil, err
		}
		for entityIdx, ent := range entities {
			links, err := e.kg.ListEntityMemoryLinks(ent.ID, limit)
			if err != nil {
				return nil, nil, err
			}
			entityBoost := 1.0 / float64(termIdx+entityIdx+1)
			for linkIdx, link := range links {
				if strings.TrimSpace(link.MemoryID) == "" {
					continue
				}
				linkBoost := 1.0 / float64(linkIdx+1)
				scores[link.MemoryID] += (link.Confidence + entityBoost) * linkBoost
			}
		}
	}

	items := make([]rankedResult, 0, len(scores))
	for id, score := range scores {
		items = append(items, rankedResult{id: id, score: score})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].score == items[j].score {
			return items[i].id < items[j].id
		}
		return items[i].score > items[j].score
	})
	if len(items) > limit {
		items = items[:limit]
	}

	ids := make([]string, 0, len(items))
	outScores := make([]float64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.id)
		outScores = append(outScores, item.score)
	}
	return ids, outScores, nil
}

func normalizeLoadedMemory(m *model.Memory) {
	if m == nil {
		return
	}
	if m.LifecycleState == "" {
		m.LifecycleState = model.LifecycleStateFromClarity(m.Clarity)
	}
	if m.ResidualForm == "" {
		stage := decay.ResidualFormFromClarity(m.Clarity, decay.DefaultParams())
		m.ResidualForm = decay.ResidualFormName(stage)
	}
}
