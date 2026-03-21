package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hiparker/echo-fade-memory/pkg/core/model"
	"github.com/hiparker/echo-fade-memory/pkg/port/imageproc"
)

type StoreImageRequest struct {
	FilePath        string
	URL             string
	SourceSession   string
	SourceKind      string
	SourceActor     string
	Caption         string
	Tags            []string
	OCRText         string
	LinkedMemoryIDs []string
	Links           []model.ImageLink
}

type ImageStats struct {
	TotalImages     int                 `json:"total_images"`
	CaptionedImages int                 `json:"captioned_images"`
	OCRImages       int                 `json:"ocr_images"`
	TotalLinks      int                 `json:"total_links"`
	RecentImages    []*model.ImageAsset `json:"recent_images"`
}

func (e *Engine) StoreImage(ctx context.Context, req StoreImageRequest) (*model.ImageAsset, bool, error) {
	if e.images == nil {
		return nil, false, fmt.Errorf("image store not configured")
	}
	filePath := strings.TrimSpace(req.FilePath)
	rawURL := strings.TrimSpace(req.URL)
	if filePath == "" && rawURL == "" {
		return nil, false, fmt.Errorf("file_path or url required")
	}
	if filePath != "" {
		abs, err := filepath.Abs(filePath)
		if err == nil {
			filePath = abs
		}
	}
	sha, err := imageSHA256(filePath, rawURL)
	if err != nil {
		return nil, false, err
	}

	existing, err := e.images.GetBySHA256(sha)
	if err != nil {
		return nil, false, err
	}

	analysis, err := e.analyzeImage(ctx, imageproc.AnalyzeInput{
		FilePath: filePath,
		URL:      rawURL,
		Caption:  req.Caption,
		Tags:     req.Tags,
		OCRText:  req.OCRText,
	})
	if err != nil {
		return nil, false, err
	}

	now := time.Now()
	asset := &model.ImageAsset{
		ID:            uuid.New().String(),
		FilePath:      filePath,
		URL:           rawURL,
		SHA256:        sha,
		SourceSession: strings.TrimSpace(req.SourceSession),
		SourceKind:    defaultString(strings.TrimSpace(req.SourceKind), "image"),
		SourceActor:   strings.TrimSpace(req.SourceActor),
		CreatedAt:     now,
		UpdatedAt:     now,
		Caption:       strings.TrimSpace(analysis.Caption),
		Tags:          uniqueStrings(analysis.Tags),
		OCRText:       strings.TrimSpace(analysis.OCRText),
	}
	isDuplicate := existing != nil
	if existing != nil {
		asset.ID = existing.ID
		asset.CreatedAt = existing.CreatedAt
		asset.UpdatedAt = now
		asset.FilePath = firstNonEmpty(filePath, existing.FilePath)
		asset.URL = firstNonEmpty(rawURL, existing.URL)
		asset.SourceSession = firstNonEmpty(asset.SourceSession, existing.SourceSession)
		asset.SourceKind = firstNonEmpty(asset.SourceKind, existing.SourceKind)
		asset.SourceActor = firstNonEmpty(asset.SourceActor, existing.SourceActor)
		asset.Caption = firstNonEmpty(asset.Caption, existing.Caption)
		asset.OCRText = firstNonEmpty(asset.OCRText, existing.OCRText)
		asset.Tags = uniqueStrings(append(existing.Tags, asset.Tags...))
	}

	indexText := imageIndexText(asset)
	vec, err := e.embed.Embed(ctx, indexText)
	if err != nil {
		return nil, false, err
	}
	if err := e.images.Upsert(asset); err != nil {
		return nil, false, err
	}
	if e.imageVector != nil {
		if err := e.imageVector.Add(asset.ID, vec); err != nil {
			return nil, false, err
		}
	}
	if e.imageBleve != nil {
		if err := e.imageBleve.Index(asset.ID, indexText); err != nil {
			return nil, false, err
		}
	}
	if err := e.replaceImageLinks(asset.ID, req.LinkedMemoryIDs, req.Links); err != nil {
		return nil, false, err
	}
	return asset, isDuplicate, nil
}

func (e *Engine) RecallImages(ctx context.Context, query string, k int) ([]model.ImageRecallResult, error) {
	if e.images == nil {
		return nil, nil
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	if k <= 0 {
		k = 5
	}
	candidateK := maxRecallCandidates(k)
	vec, err := e.embed.Embed(ctx, query)
	if err != nil {
		return nil, err
	}

	var vecIDs []string
	var vecScores []float32
	var bm25IDs []string
	var bm25Scores []float64
	if e.imageVector != nil {
		vecIDs, vecScores, err = e.imageVector.Search(ctx, vec, candidateK)
		if err != nil {
			return nil, err
		}
	}
	if e.imageBleve != nil {
		bm25IDs, bm25Scores, err = e.imageBleve.Search(ctx, query, candidateK)
		if err != nil {
			return nil, err
		}
	}
	linkedIDs, linkedBoostMap, err := e.imageLinksRecall(ctx, query, candidateK)
	if err != nil {
		return nil, err
	}
	combined := rrfFusionDetailed(60.0, vecIDs, bm25IDs, linkedIDs)
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
	results := make([]model.ImageRecallResult, 0, k)
	for _, item := range combined {
		if len(results) >= k {
			break
		}
		asset, err := e.images.Get(item.id)
		if err != nil || asset == nil {
			continue
		}
		links, err := e.images.ListLinks(asset.ID)
		if err != nil {
			return nil, err
		}
		results = append(results, model.ImageRecallResult{
			Asset:           asset,
			ImageID:         asset.ID,
			Score:           item.score + linkedBoostMap[asset.ID],
			VectorScore:     vecScoreMap[asset.ID],
			VectorRank:      vecRankMap[asset.ID],
			KeywordScore:    bm25ScoreMap[asset.ID],
			KeywordRank:     bm25RankMap[asset.ID],
			LinkedBoost:     linkedBoostMap[asset.ID],
			LinkedMemoryIDs: imageLinkedMemoryIDs(links),
		})
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			return results[i].ImageID < results[j].ImageID
		}
		return results[i].Score > results[j].Score
	})
	return results, nil
}

func (e *Engine) ListImages(ctx context.Context, query string, limit int) ([]*model.ImageAsset, error) {
	_ = ctx
	if e.images == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}
	if strings.TrimSpace(query) == "" {
		return e.images.ListRecent(limit)
	}
	return e.images.Find(query, limit)
}

func (e *Engine) GetImage(ctx context.Context, id string) (*model.ImageAsset, error) {
	_ = ctx
	if e.images == nil {
		return nil, nil
	}
	return e.images.Get(strings.TrimSpace(id))
}

func (e *Engine) ImageLinks(ctx context.Context, id string) ([]model.ImageLink, error) {
	_ = ctx
	if e.images == nil {
		return nil, nil
	}
	return e.images.ListLinks(strings.TrimSpace(id))
}

func (e *Engine) LinkImage(ctx context.Context, imageID string, links []model.ImageLink) error {
	_ = ctx
	if e.images == nil {
		return fmt.Errorf("image store not configured")
	}
	if strings.TrimSpace(imageID) == "" {
		return fmt.Errorf("image id required")
	}
	existing, err := e.images.ListLinks(imageID)
	if err != nil {
		return err
	}
	return e.images.ReplaceLinks(imageID, mergeImageLinks(existing, links))
}

func (e *Engine) StatsImages(ctx context.Context, topK int) (*ImageStats, error) {
	_ = ctx
	if e.images == nil {
		return &ImageStats{}, nil
	}
	if topK <= 0 {
		topK = 10
	}
	total, err := e.images.CountAssets()
	if err != nil {
		return nil, err
	}
	captioned, err := e.images.CountAssetsWithCaption()
	if err != nil {
		return nil, err
	}
	ocrCount, err := e.images.CountAssetsWithOCR()
	if err != nil {
		return nil, err
	}
	links, err := e.images.CountLinks()
	if err != nil {
		return nil, err
	}
	recent, err := e.images.ListRecent(topK)
	if err != nil {
		return nil, err
	}
	return &ImageStats{
		TotalImages:     total,
		CaptionedImages: captioned,
		OCRImages:       ocrCount,
		TotalLinks:      links,
		RecentImages:    recent,
	}, nil
}

func (e *Engine) analyzeImage(ctx context.Context, input imageproc.AnalyzeInput) (*imageproc.AnalyzeOutput, error) {
	if e.imageAnalyzer == nil {
		return &imageproc.AnalyzeOutput{
			Caption: strings.TrimSpace(input.Caption),
			Tags:    uniqueStrings(input.Tags),
			OCRText: strings.TrimSpace(input.OCRText),
		}, nil
	}
	return e.imageAnalyzer.Analyze(ctx, input)
}

func (e *Engine) replaceImageLinks(imageID string, linkedMemoryIDs []string, links []model.ImageLink) error {
	if e.images == nil {
		return nil
	}
	existing, err := e.images.ListLinks(imageID)
	if err != nil {
		return err
	}
	merged := append([]model.ImageLink{}, existing...)
	for _, memoryID := range linkedMemoryIDs {
		memoryID = strings.TrimSpace(memoryID)
		if memoryID == "" {
			continue
		}
		merged = append(merged, model.ImageLink{
			ImageID:   imageID,
			LinkType:  "memory",
			TargetID:  memoryID,
			CreatedAt: time.Now(),
		})
	}
	merged = append(merged, links...)
	return e.images.ReplaceLinks(imageID, mergeImageLinks(nil, merged))
}

func (e *Engine) imageLinksRecall(ctx context.Context, query string, limit int) ([]string, map[string]float64, error) {
	boosts := map[string]float64{}
	if e.images == nil {
		return nil, boosts, nil
	}
	trace, err := e.Explain(ctx, query, limit, 0)
	if err != nil {
		return nil, nil, err
	}
	ids := make([]string, 0, limit)
	seen := map[string]struct{}{}
	for idx, memory := range trace.Accepted {
		links, err := e.images.ListLinksByTarget("memory", memory.MemoryID, limit)
		if err != nil {
			return nil, nil, err
		}
		boost := 1.0 / float64(idx+1)
		for _, link := range links {
			boosts[link.ImageID] += boost
			if _, ok := seen[link.ImageID]; ok {
				continue
			}
			seen[link.ImageID] = struct{}{}
			ids = append(ids, link.ImageID)
		}
	}
	return ids, boosts, nil
}

func imageIndexText(asset *model.ImageAsset) string {
	parts := []string{strings.TrimSpace(asset.Caption), strings.Join(asset.Tags, " "), strings.TrimSpace(asset.OCRText)}
	if asset.FilePath != "" {
		parts = append(parts, filepath.Base(asset.FilePath))
	}
	if asset.URL != "" {
		parts = append(parts, asset.URL)
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func imageLinkedMemoryIDs(links []model.ImageLink) []string {
	out := make([]string, 0, len(links))
	for _, link := range links {
		if link.LinkType == "memory" && strings.TrimSpace(link.TargetID) != "" {
			out = append(out, link.TargetID)
		}
	}
	return uniqueStrings(out)
}

func imageSHA256(filePath, rawURL string) (string, error) {
	h := sha256.New()
	switch {
	case filePath != "":
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", err
		}
		_, _ = h.Write(data)
	case rawURL != "":
		_, _ = h.Write([]byte("url:"))
		_, _ = h.Write([]byte(rawURL))
	default:
		return "", fmt.Errorf("file_path or url required")
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func mergeImageLinks(base []model.ImageLink, extra []model.ImageLink) []model.ImageLink {
	seen := map[string]model.ImageLink{}
	for _, item := range append(base, extra...) {
		item.ImageID = strings.TrimSpace(item.ImageID)
		item.LinkType = strings.TrimSpace(item.LinkType)
		item.TargetID = strings.TrimSpace(item.TargetID)
		if item.ImageID == "" || item.LinkType == "" || item.TargetID == "" {
			continue
		}
		if item.CreatedAt.IsZero() {
			item.CreatedAt = time.Now()
		}
		key := item.ImageID + ":" + item.LinkType + ":" + item.TargetID
		seen[key] = item
	}
	out := make([]model.ImageLink, 0, len(seen))
	for _, item := range seen {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			if out[i].LinkType == out[j].LinkType {
				return out[i].TargetID < out[j].TargetID
			}
			return out[i].LinkType < out[j].LinkType
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
