package engine

import (
	"context"
	"strings"
	"time"

	"github.com/hiparker/echo-fade-memory/pkg/core/model"
)

type EntityMemoryResult struct {
	MemoryID        string            `json:"memory_id"`
	Summary         string            `json:"summary"`
	Content         string            `json:"content"`
	ResidualContent string            `json:"residual_content"`
	Clarity         float64           `json:"clarity"`
	DecayStage      string            `json:"decay_stage"`
	LifecycleState  string            `json:"lifecycle_state,omitempty"`
	ConflictGroup   string            `json:"conflict_group,omitempty"`
	Version         int               `json:"version,omitempty"`
	SourceRefs      []model.SourceRef `json:"source_refs,omitempty"`
	LinkRole        string            `json:"link_role,omitempty"`
	LinkMention     string            `json:"link_mention,omitempty"`
	LinkConfidence  float64           `json:"link_confidence,omitempty"`
}

type EntityStats struct {
	TotalEntities  int             `json:"total_entities"`
	TotalRelations int             `json:"total_relations"`
	TotalLinks     int             `json:"total_links"`
	TopEntities    []*model.Entity `json:"top_entities"`
}

func (e *Engine) ListEntities(ctx context.Context, query string, limit int) ([]*model.Entity, error) {
	_ = ctx
	if e.kg == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	return e.kg.FindEntities(strings.TrimSpace(query), limit)
}

func (e *Engine) GetEntity(ctx context.Context, id string) (*model.Entity, error) {
	_ = ctx
	if e.kg == nil {
		return nil, nil
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, nil
	}
	return e.kg.GetEntity(id)
}

func (e *Engine) EntityRelations(ctx context.Context, id string, limit int) ([]*model.Relation, error) {
	_ = ctx
	if e.kg == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	return e.kg.ListRelations(strings.TrimSpace(id), limit)
}

func (e *Engine) EntityMemories(ctx context.Context, id string, limit int) ([]EntityMemoryResult, error) {
	_ = ctx
	if e.kg == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	links, err := e.kg.ListEntityMemoryLinks(strings.TrimSpace(id), limit)
	if err != nil {
		return nil, err
	}
	results := make([]EntityMemoryResult, 0, len(links))
	for _, link := range links {
		if strings.TrimSpace(link.MemoryID) == "" {
			continue
		}
		m, err := e.mem.Get(link.MemoryID)
		if err != nil || m == nil {
			continue
		}
		normalizeLoadedMemory(m)
		results = append(results, EntityMemoryResult{
			MemoryID:        m.ID,
			Summary:         recallSummary(m),
			Content:         m.Content,
			ResidualContent: m.ResidualContent,
			Clarity:         m.Clarity,
			DecayStage:      m.ResidualForm,
			LifecycleState:  m.LifecycleState,
			ConflictGroup:   m.ConflictGroup,
			Version:         m.Version,
			SourceRefs:      m.SourceRefs,
			LinkRole:        link.Role,
			LinkMention:     link.Mention,
			LinkConfidence:  link.Confidence,
		})
	}
	return results, nil
}

func (e *Engine) StatsEntities(ctx context.Context, topK int) (*EntityStats, error) {
	_ = ctx
	if e.kg == nil {
		return &EntityStats{}, nil
	}
	if topK <= 0 {
		topK = 10
	}
	if topK > 100 {
		topK = 100
	}
	totalEntities, err := e.kg.CountEntities()
	if err != nil {
		return nil, err
	}
	totalRelations, err := e.kg.CountRelations()
	if err != nil {
		return nil, err
	}
	totalLinks, err := e.kg.CountMemoryEntityLinks()
	if err != nil {
		return nil, err
	}
	topEntities, err := e.kg.FindEntities("", topK)
	if err != nil {
		return nil, err
	}
	return &EntityStats{
		TotalEntities:  totalEntities,
		TotalRelations: totalRelations,
		TotalLinks:     totalLinks,
		TopEntities:    topEntities,
	}, nil
}

func formatEntityTime(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	return ts.Format(time.RFC3339)
}
