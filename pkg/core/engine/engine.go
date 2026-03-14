package engine

import (
	"context"
	"os"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/echo-fade-memory/echo-fade-memory/pkg/basic/util/safe"
	"github.com/echo-fade-memory/echo-fade-memory/pkg/config"
	"github.com/echo-fade-memory/echo-fade-memory/pkg/core/decay"
	"github.com/echo-fade-memory/echo-fade-memory/pkg/core/embedding"
	"github.com/echo-fade-memory/echo-fade-memory/pkg/core/model"
	"github.com/echo-fade-memory/echo-fade-memory/pkg/core/transform"
	"github.com/echo-fade-memory/echo-fade-memory/pkg/port/store"
	"github.com/google/uuid"
)

// Engine is the core memory engine.
type Engine struct {
	cfg           *config.Config
	mem           store.MemoryStore
	vector        store.VectorStore
	bleve         *store.BleveStore
	embed         *embedding.Client
	decay         decay.Params
	mu            sync.RWMutex
	lastDecayAt   time.Time
	decayCacheTTL time.Duration
}

// New creates a new Engine.
func New(cfg *config.Config) (*Engine, error) {
	if err := os.MkdirAll(cfg.DataPath, 0755); err != nil {
		return nil, err
	}

	mem, err := store.NewMemoryStore(cfg)
	if err != nil {
		return nil, err
	}

	vector, err := store.NewVectorStore(cfg)
	if err != nil {
		mem.Close()
		return nil, err
	}

	bleve, err := store.OpenOrCreateBleve(cfg.BlevePath())
	if err != nil {
		mem.Close()
		return nil, err
	}

	embed := embedding.NewOllamaClient(cfg.Ollama.URL, cfg.Ollama.Model, cfg.Ollama.Dimensions)
	decayParams := decay.ParamsFromFull(decay.ParamsFromFullArgs{
		Tau:          cfg.Decay.Tau,
		Alpha:        cfg.Decay.Alpha,
		Epsilon:      cfg.Decay.Epsilon,
		Lambda:       cfg.Decay.Lambda,
		AccessBoost:  cfg.Decay.AccessBoost,
		HorizonDays:  cfg.Decay.HorizonDays,
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
	vec, err := e.embed.Embed(ctx, content)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	m := &model.Memory{
		ID:              uuid.New().String(),
		Content:         content,
		Embedding:       vec,
		Importance:      importance,
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
	Memory *model.Memory
	Score  float64
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
		m.AccessCount++
		_ = e.mem.UpdateAccess(id, m.AccessCount)
		score := 1.0 / float64(i+1+60)
		results = append(results, RecallResult{Memory: m, Score: score})
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

// DecayAll recomputes clarity and residual for all memories.
func (e *Engine) DecayAll(ctx context.Context) error {
	memories, err := e.mem.ListAll()
	if err != nil {
		return err
	}
	updates := make([]store.DecayUpdate, len(memories))
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
				residualContent := transform.ToResidualContinuous(m.Content, strength)
				updates[base+j] = store.DecayUpdate{
					ID:              m.ID,
					Clarity:         strength,
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
	_ = e.bleve.Close()
	return nil
}
