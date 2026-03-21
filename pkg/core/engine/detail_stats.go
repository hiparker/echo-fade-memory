package engine

import (
	"context"
	"os"
	"sort"
	"strings"
	"time"
)

type DetailImageItem struct {
	ID            string  `json:"id"`
	Caption       string  `json:"caption"`
	FilePath      string  `json:"file_path,omitempty"`
	URL           string  `json:"url,omitempty"`
	SourceKind    string  `json:"source_kind,omitempty"`
	SourceSession string  `json:"source_session,omitempty"`
	CreatedAt     string  `json:"created_at,omitempty"`
	LinkCount     int     `json:"link_count"`
	OCRText       string  `json:"ocr_text,omitempty"`
	CoverageScore float64 `json:"coverage_score,omitempty"`
}

type DetailImageStats struct {
	TotalImages       int               `json:"total_images"`
	CaptionedImages   int               `json:"captioned_images"`
	OCRImages         int               `json:"ocr_images"`
	LinkedImages      int               `json:"linked_images"`
	OrphanImages      int               `json:"orphan_images"`
	BrokenFilePaths   int               `json:"broken_file_paths"`
	VectorTotal       int               `json:"vector_total"`
	CaptionCoverage   float64           `json:"caption_coverage"`
	OCRCoverage       float64           `json:"ocr_coverage"`
	LinkedCoverage    float64           `json:"linked_coverage"`
	BySourceKind      map[string]int    `json:"by_source_kind"`
	TrendCreatedDaily []TrendPoint      `json:"trend_created_daily"`
	TopRecentImages   []DetailImageItem `json:"top_recent_images"`
	TopLinkedImages   []DetailImageItem `json:"top_linked_images"`
}

type DetailStats struct {
	WindowDays int               `json:"window_days"`
	Overview   *OverviewStats    `json:"overview"`
	Integrity  *IntegrityStats   `json:"integrity"`
	Images     *DetailImageStats `json:"images"`
	Entities   *EntityStats      `json:"entities"`
}

func (e *Engine) StatsDetail(ctx context.Context, windowDays, topK, sampleSize int) (*DetailStats, error) {
	if windowDays <= 0 {
		windowDays = defaultOverviewWindowDays
	}
	if topK <= 0 {
		topK = 10
	}
	if sampleSize <= 0 {
		sampleSize = defaultIntegritySample
	}
	overview, err := e.StatsOverviewWithOptions(ctx, OverviewOptions{
		WindowDays: windowDays,
		TopK:       topK,
	})
	if err != nil {
		return nil, err
	}
	integrity, err := e.StatsIntegrity(ctx, sampleSize)
	if err != nil {
		return nil, err
	}
	images, err := e.statsDetailImages(ctx, windowDays, topK)
	if err != nil {
		return nil, err
	}
	entities, err := e.StatsEntities(ctx, topK)
	if err != nil {
		return nil, err
	}
	return &DetailStats{
		WindowDays: windowDays,
		Overview:   overview,
		Integrity:  integrity,
		Images:     images,
		Entities:   entities,
	}, nil
}

func (e *Engine) statsDetailImages(ctx context.Context, windowDays, topK int) (*DetailImageStats, error) {
	_ = ctx
	if e.images == nil {
		return &DetailImageStats{
			BySourceKind:      map[string]int{},
			TrendCreatedDaily: []TrendPoint{},
			TopRecentImages:   []DetailImageItem{},
			TopLinkedImages:   []DetailImageItem{},
		}, nil
	}
	assets, err := e.images.ListAll()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	start := now.AddDate(0, 0, -windowDays+1)
	bySourceKind := map[string]int{}
	trendMap := map[string]int{}
	topRecent := make([]DetailImageItem, 0, len(assets))
	topLinked := make([]DetailImageItem, 0, len(assets))
	totalImages := len(assets)
	captioned := 0
	ocrCount := 0
	linkedCount := 0
	orphanCount := 0
	brokenCount := 0
	for _, asset := range assets {
		if asset == nil {
			continue
		}
		links, err := e.images.ListLinks(asset.ID)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(asset.Caption) != "" {
			captioned++
		}
		if strings.TrimSpace(asset.OCRText) != "" {
			ocrCount++
		}
		if len(links) > 0 {
			linkedCount++
		} else {
			orphanCount++
		}
		if asset.FilePath != "" {
			if _, err := os.Stat(asset.FilePath); err != nil {
				brokenCount++
			}
		}
		sourceKind := strings.TrimSpace(asset.SourceKind)
		if sourceKind == "" {
			sourceKind = "unknown"
		}
		bySourceKind[sourceKind]++
		if !asset.CreatedAt.IsZero() && !asset.CreatedAt.Before(start) {
			day := asset.CreatedAt.Format("2006-01-02")
			trendMap[day]++
		}
		coverageScore := 0.0
		if strings.TrimSpace(asset.Caption) != "" {
			coverageScore += 0.4
		}
		if strings.TrimSpace(asset.OCRText) != "" {
			coverageScore += 0.3
		}
		if len(asset.Tags) > 0 {
			coverageScore += 0.1
		}
		if len(links) > 0 {
			coverageScore += 0.2
		}
		item := DetailImageItem{
			ID:            asset.ID,
			Caption:       firstNonEmpty(asset.Caption, asset.FilePath, asset.URL, asset.ID),
			FilePath:      asset.FilePath,
			URL:           asset.URL,
			SourceKind:    asset.SourceKind,
			SourceSession: asset.SourceSession,
			CreatedAt:     asset.CreatedAt.Format(time.RFC3339),
			LinkCount:     len(links),
			OCRText:       asset.OCRText,
			CoverageScore: coverageScore,
		}
		topRecent = append(topRecent, item)
		topLinked = append(topLinked, item)
	}
	sort.Slice(topRecent, func(i, j int) bool {
		return topRecent[i].CreatedAt > topRecent[j].CreatedAt
	})
	sort.Slice(topLinked, func(i, j int) bool {
		if topLinked[i].LinkCount == topLinked[j].LinkCount {
			return topLinked[i].CoverageScore > topLinked[j].CoverageScore
		}
		return topLinked[i].LinkCount > topLinked[j].LinkCount
	})
	if topK < len(topRecent) {
		topRecent = topRecent[:topK]
	}
	if topK < len(topLinked) {
		topLinked = topLinked[:topK]
	}
	vectorTotal := -1
	if c, ok := e.imageVector.(vectorCountCap); ok {
		vectorTotal = c.VectorCount()
	}
	trend := make([]TrendPoint, 0, windowDays)
	for i := 0; i < windowDays; i++ {
		day := start.AddDate(0, 0, i).Format("2006-01-02")
		trend = append(trend, TrendPoint{
			Date:  day,
			Count: trendMap[day],
		})
	}
	captionCoverage := ratio(captioned, totalImages)
	ocrCoverage := ratio(ocrCount, totalImages)
	linkedCoverage := ratio(linkedCount, totalImages)
	return &DetailImageStats{
		TotalImages:       totalImages,
		CaptionedImages:   captioned,
		OCRImages:         ocrCount,
		LinkedImages:      linkedCount,
		OrphanImages:      orphanCount,
		BrokenFilePaths:   brokenCount,
		VectorTotal:       vectorTotal,
		CaptionCoverage:   captionCoverage,
		OCRCoverage:       ocrCoverage,
		LinkedCoverage:    linkedCoverage,
		BySourceKind:      bySourceKind,
		TrendCreatedDaily: trend,
		TopRecentImages:   topRecent,
		TopLinkedImages:   topLinked,
	}, nil
}

func ratio(numerator, denominator int) float64 {
	if denominator <= 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}
