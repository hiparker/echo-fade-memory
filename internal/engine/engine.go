package engine

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/echo-fade-memory/echo-fade-memory/internal/config"
	"github.com/echo-fade-memory/echo-fade-memory/internal/decay"
	"github.com/echo-fade-memory/echo-fade-memory/internal/embedding"
	"github.com/echo-fade-memory/echo-fade-memory/internal/model"
	"github.com/echo-fade-memory/echo-fade-memory/internal/store"
	"github.com/echo-fade-memory/echo-fade-memory/internal/transform"
	"github.com/google/uuid"
)

// Engine is the core memory engine.
type Engine struct {
	cfg     *config.Config
	sqlite  *store.SQLiteStore
	vector  *store.VectorStore
	bleve   *store.BleveStore
	embed   *embedding.Client
	decay   decay.Params
	mu      sync.RWMutex
}

// New creates a new Engine.
func New(cfg *config.Config) (*Engine, error) {
	if err := os.MkdirAll(cfg.DataPath, 0755); err != nil {
		return nil, err
	}

	sqlite, err := store.NewSQLiteStore(cfg.SQLitePath())
	if err != nil {
		return nil, err
	}

	vector, err := store.NewVectorStore(cfg.VectorPath())
	if err != nil {
		sqlite.Close()
		return nil, err
	}

	bleve, err := store.OpenOrCreateBleve(cfg.BlevePath())
	if err != nil {
		sqlite.Close()
		return nil, err
	}

	embed := embedding.NewOllamaClient(cfg.OllamaURL, cfg.EmbedModel, cfg.EmbedDim)

	return &Engine{
		cfg:    cfg,
		sqlite: sqlite,
		vector: vector,
		bleve:  bleve,
		embed:  embed,
		decay:  decay.DefaultParams(),
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

	if err := e.sqlite.Save(m); err != nil {
		return nil, err
	}
	if err := e.vector.Add(m.ID, vec); err != nil {
		return nil, err
	}
	if err := e.bleve.Index(m.ID, content); err != nil {
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
// Runs DecayAll first to ensure fresh clarity values.
func (e *Engine) Recall(ctx context.Context, query string, k int, minClarity float64) ([]RecallResult, error) {
	_ = e.DecayAll(ctx)
	vec, err := e.embed.Embed(ctx, query)
	if err != nil {
		return nil, err
	}

	// Vector search
	vecIDs, vecScores, err := e.vector.Search(ctx, vec, k*2)
	if err != nil {
		return nil, err
	}

	// BM25 search
	bm25IDs, bm25Scores, err := e.bleve.Search(ctx, query, k*2)
	if err != nil {
		return nil, err
	}

	// RRF fusion (k=60 typical)
	rrfK := 60.0
	combined := rrfFusion(vecIDs, vecScores, bm25IDs, bm25Scores, rrfK)

	// Load memories, filter by clarity
	var results []RecallResult
	for i, id := range combined {
		if i >= k {
			break
		}
		m, err := e.sqlite.Get(id)
		if err != nil || m == nil {
			continue
		}
		if m.Clarity < minClarity {
			continue
		}
		// Update access (reinforcement)
		m.AccessCount++
		_ = e.sqlite.UpdateAccess(id, m.AccessCount)

		score := 1.0 / float64(i+1+60) // RRF-style score for ranking
		results = append(results, RecallResult{Memory: m, Score: score})
	}

	return results, nil
}

// rrfFusion merges vector and BM25 results using Reciprocal Rank Fusion.
func rrfFusion(vecIDs []string, vecScores []float32, bm25IDs []string, bm25Scores []float64, k float64) []string {
	scores := make(map[string]float64)
	for i, id := range vecIDs {
		scores[id] += 1.0 / (k + float64(i+1))
	}
	for i, id := range bm25IDs {
		scores[id] += 1.0 / (k + float64(i+1))
	}

	// Sort by score
	type item struct {
		id    string
		score float64
	}
	var items []item
	for id, s := range scores {
		items = append(items, item{id, s})
	}
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].score > items[i].score {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.id
	}
	return out
}

// DecayAll recomputes clarity and residual for all memories.
func (e *Engine) DecayAll(ctx context.Context) error {
	ids, err := e.sqlite.List()
	if err != nil {
		return err
	}

	now := time.Now()
	for _, id := range ids {
		m, err := e.sqlite.Get(id)
		if err != nil || m == nil {
			continue
		}
		ageDays := int(now.Sub(m.CreatedAt).Hours() / 24)
		if ageDays < 0 {
			ageDays = 0
		}

		clarity := decay.Clarity(m, e.decay)
		stage := decay.ResidualForm(ageDays)
		residualForm := decay.ResidualFormName(stage)
		residualContent := transform.ToResidual(m.Content, stage)

		_ = e.sqlite.UpdateDecay(id, clarity, residualForm, residualContent)
	}
	return nil
}

// Close closes all stores.
func (e *Engine) Close() error {
	_ = e.sqlite.Close()
	_ = e.bleve.Close()
	return nil
}
