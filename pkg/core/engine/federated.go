package engine

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/hiparker/echo-fade-memory/pkg/core/entity"
	"github.com/hiparker/echo-fade-memory/pkg/core/model"
)

type ToolStoreRequest struct {
	ObjectType      string
	Content         string
	Summary         string
	Importance      float64
	MemoryType      string
	ConflictGroup   string
	SourceKind      string
	SourceRef       string
	SourceSession   string
	SourceActor     string
	FilePath        string
	URL             string
	Caption         string
	Tags            []string
	OCRText         string
	LinkedMemoryIDs []string
	Links           []model.ImageLink
}

type ToolStoreResult struct {
	Status     string
	ObjectType string
	ID         string
	Duplicate  bool
	Memory     *model.Memory
	Image      *model.ImageAsset
}

type ToolForgetResult struct {
	Status     string
	ObjectType string
	ID         string
	Query      string
	Match      *FederatedRecallItem
}

type FederatedRecallItem struct {
	ObjectType string  `json:"object_type"`
	ID         string  `json:"id"`
	Score      float64 `json:"score"`
	Title      string  `json:"title,omitempty"`
	Summary    string  `json:"summary,omitempty"`
}

type FederatedRecallResult struct {
	Query    string                    `json:"query"`
	Memories []RecallResult            `json:"memories"`
	Images   []model.ImageRecallResult `json:"images"`
	Entities []*model.Entity           `json:"entities"`
	Mixed    []FederatedRecallItem     `json:"mixed"`
	Explain  *ExplainResult            `json:"explain,omitempty"`
}

func (e *Engine) StoreTool(ctx context.Context, req ToolStoreRequest) (*ToolStoreResult, error) {
	objectType := strings.ToLower(strings.TrimSpace(req.ObjectType))
	if objectType == "" {
		switch {
		case strings.TrimSpace(req.FilePath) != "" || strings.TrimSpace(req.URL) != "":
			objectType = "image"
		default:
			objectType = "memory"
		}
	}
	switch objectType {
	case "memory":
		content := strings.TrimSpace(req.Content)
		if content == "" {
			return nil, fmt.Errorf("content required")
		}
		importance := req.Importance
		if importance == 0 {
			importance = 0.8
		}
		sourceKind := strings.TrimSpace(req.SourceKind)
		if sourceKind == "" {
			sourceKind = "chat"
		}
		sourceRef := strings.TrimSpace(req.SourceRef)
		if sourceRef == "" {
			sourceRef = "session:tool"
		}
		m, err := e.Remember(ctx, RememberRequest{
			Content:       content,
			Summary:       strings.TrimSpace(req.Summary),
			Importance:    importance,
			MemoryType:    strings.TrimSpace(req.MemoryType),
			ConflictGroup: strings.TrimSpace(req.ConflictGroup),
			SourceRefs: []model.SourceRef{
				{Kind: sourceKind, Ref: sourceRef},
			},
		})
		if err != nil {
			return nil, err
		}
		return &ToolStoreResult{
			Status:     "stored",
			ObjectType: "memory",
			ID:         m.ID,
			Memory:     m,
		}, nil
	case "image":
		asset, duplicate, err := e.StoreImage(ctx, StoreImageRequest{
			FilePath:        strings.TrimSpace(req.FilePath),
			URL:             strings.TrimSpace(req.URL),
			SourceSession:   strings.TrimSpace(req.SourceSession),
			SourceKind:      strings.TrimSpace(req.SourceKind),
			SourceActor:     strings.TrimSpace(req.SourceActor),
			Caption:         strings.TrimSpace(req.Caption),
			Tags:            req.Tags,
			OCRText:         strings.TrimSpace(req.OCRText),
			LinkedMemoryIDs: req.LinkedMemoryIDs,
			Links:           req.Links,
		})
		if err != nil {
			return nil, err
		}
		return &ToolStoreResult{
			Status:     "stored",
			ObjectType: "image",
			ID:         asset.ID,
			Duplicate:  duplicate,
			Image:      asset,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported object_type %q", objectType)
	}
}

func (e *Engine) RecallTool(ctx context.Context, query string, k int, includeExplain bool) (*FederatedRecallResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query required")
	}
	if k <= 0 {
		k = 5
	}
	if k > 100 {
		k = 100
	}
	candidateK := maxRecallCandidates(k)

	var memories []RecallResult
	var images []model.ImageRecallResult
	var entities []*model.Entity
	var explain *ExplainResult
	results, err := e.Recall(ctx, query, k, 0)
	if err != nil {
		return nil, err
	}
	memories = results
	imageResults, err := e.RecallImages(ctx, query, candidateK)
	if err != nil {
		return nil, err
	}
	if len(imageResults) > k {
		imageResults = imageResults[:k]
	}
	images = imageResults
	entityResults, err := e.recallEntities(ctx, query, candidateK)
	if err != nil {
		return nil, err
	}
	if len(entityResults) > k {
		entityResults = entityResults[:k]
	}
	entities = entityResults
	if includeExplain {
		result, err := e.Explain(ctx, query, k, 0)
		if err != nil {
			return nil, err
		}
		explain = result
	}

	mixed := make([]FederatedRecallItem, 0, len(memories)+len(images)+len(entities))
	for _, item := range memories {
		mixed = append(mixed, FederatedRecallItem{
			ObjectType: "memory",
			ID:         item.MemoryID,
			Score:      item.Score,
			Title:      item.Summary,
			Summary:    item.Memory.ResidualContent,
		})
	}
	for _, item := range images {
		title := ""
		if item.Asset != nil {
			title = firstNonEmpty(item.Asset.Caption, item.Asset.FilePath, item.Asset.URL, item.ImageID)
		}
		mixed = append(mixed, FederatedRecallItem{
			ObjectType: "image",
			ID:         item.ImageID,
			Score:      item.Score,
			Title:      title,
			Summary:    strings.TrimSpace(strings.Join(item.LinkedMemoryIDs, ", ")),
		})
	}
	for i, item := range entities {
		score := 1.0 / float64(i+1)
		mixed = append(mixed, FederatedRecallItem{
			ObjectType: "entity",
			ID:         item.ID,
			Score:      score,
			Title:      firstNonEmpty(item.DisplayName, item.CanonicalName, item.ID),
			Summary:    item.EntityType,
		})
	}
	sort.Slice(mixed, func(i, j int) bool {
		if mixed[i].Score == mixed[j].Score {
			if mixed[i].ObjectType == mixed[j].ObjectType {
				return mixed[i].ID < mixed[j].ID
			}
			return mixed[i].ObjectType < mixed[j].ObjectType
		}
		return mixed[i].Score > mixed[j].Score
	})

	return &FederatedRecallResult{
		Query:    query,
		Memories: memories,
		Images:   images,
		Entities: entities,
		Mixed:    mixed,
		Explain:  explain,
	}, nil
}

func (e *Engine) recallEntities(ctx context.Context, query string, limit int) ([]*model.Entity, error) {
	if e.kg == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 5
	}
	terms := entity.QueryTerms(query)
	if len(terms) == 0 {
		terms = []string{query}
	}
	type scoredEntity struct {
		entity *model.Entity
		score  float64
	}
	scored := map[string]*scoredEntity{}
	for termIndex, term := range terms {
		items, err := e.ListEntities(ctx, term, limit)
		if err != nil {
			return nil, err
		}
		for rank, item := range items {
			if item == nil || strings.TrimSpace(item.ID) == "" {
				continue
			}
			entry, ok := scored[item.ID]
			if !ok {
				entry = &scoredEntity{entity: item}
				scored[item.ID] = entry
			}
			entry.score += 1.0/float64(termIndex+1) + 1.0/float64(rank+1)
			entry.score += float64(item.MemoryCount) * 0.01
		}
	}
	results := make([]*scoredEntity, 0, len(scored))
	for _, item := range scored {
		results = append(results, item)
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].score == results[j].score {
			return results[i].entity.ID < results[j].entity.ID
		}
		return results[i].score > results[j].score
	})
	if limit > len(results) {
		limit = len(results)
	}
	out := make([]*model.Entity, 0, limit)
	for i := 0; i < limit; i++ {
		out = append(out, results[i].entity)
	}
	return out, nil
}

func (e *Engine) ForgetTool(ctx context.Context, objectType, id, query string, k int) (*ToolForgetResult, error) {
	objectType = strings.ToLower(strings.TrimSpace(objectType))
	id = strings.TrimSpace(id)
	query = strings.TrimSpace(query)
	if id == "" && query == "" {
		return nil, fmt.Errorf("id or query required")
	}
	var match *FederatedRecallItem
	if id == "" {
		if k <= 0 {
			k = 5
		}
		results, err := e.RecallTool(ctx, query, k, false)
		if err != nil {
			return nil, err
		}
		for _, item := range results.Mixed {
			if item.ObjectType != "memory" && item.ObjectType != "image" {
				continue
			}
			if objectType != "" && objectType != "auto" && item.ObjectType != objectType {
				continue
			}
			candidate := item
			match = &candidate
			id = candidate.ID
			objectType = candidate.ObjectType
			break
		}
		if match == nil {
			return nil, fmt.Errorf("object not found")
		}
	}
	if objectType == "" || objectType == "auto" {
		if m, err := e.mem.Get(id); err == nil && m != nil {
			objectType = "memory"
		} else if e.images != nil {
			if img, err := e.images.Get(id); err == nil && img != nil {
				objectType = "image"
			}
		}
		if objectType == "" {
			return nil, fmt.Errorf("object not found")
		}
	}
	switch objectType {
	case "memory":
		if err := e.Forget(ctx, id); err != nil {
			return nil, err
		}
		return &ToolForgetResult{
			Status:     "forgotten",
			ObjectType: "memory",
			ID:         id,
			Query:      query,
			Match:      match,
		}, nil
	case "image":
		if err := e.ForgetImage(ctx, id); err != nil {
			return nil, err
		}
		return &ToolForgetResult{
			Status:     "forgotten",
			ObjectType: "image",
			ID:         id,
			Query:      query,
			Match:      match,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported object_type %q", objectType)
	}
}

func (e *Engine) ForgetImage(ctx context.Context, id string) error {
	_ = ctx
	if e.images == nil {
		return fmt.Errorf("image store not configured")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("image id required")
	}
	if e.imageVector != nil {
		if err := e.imageVector.Remove(id); err != nil {
			return err
		}
	}
	if e.imageBleve != nil {
		if err := e.imageBleve.Delete(id); err != nil {
			return err
		}
	}
	return e.images.Delete(id)
}
