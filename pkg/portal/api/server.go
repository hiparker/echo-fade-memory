package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hiparker/echo-fade-memory/pkg/core/engine"
	"github.com/hiparker/echo-fade-memory/pkg/core/model"
)

const (
	maxBodyBytes = 1 << 20 // 1MB
	maxRecallK   = 100
)

// Server provides HTTP API for the memory engine.
type Server struct {
	engine *engine.Engine
}

// NewServer creates a new API server.
func NewServer(eng *engine.Engine) *Server {
	return &Server{engine: eng}
}

// StoreRequest is the request body for memory creation.
type StoreRequest struct {
	Content       string            `json:"content"`
	Summary       string            `json:"summary,omitempty"`
	Importance    float64           `json:"importance,omitempty"`
	MemoryType    string            `json:"memory_type,omitempty"`
	ConflictGroup string            `json:"conflict_group,omitempty"`
	SourceRefs    []model.SourceRef `json:"source_refs,omitempty"`
}

// StoreResponse is the response for memory creation or reinforcement.
type StoreResponse struct {
	ID             string `json:"id"`
	Summary        string `json:"summary,omitempty"`
	LifecycleState string `json:"lifecycle_state,omitempty"`
	ConflictGroup  string `json:"conflict_group,omitempty"`
	Version        int    `json:"version,omitempty"`
}

// RecallItem is a single recalled memory.
type RecallItem struct {
	ID              string                  `json:"id"`
	Content         string                  `json:"content"`
	Summary         string                  `json:"summary"`
	ResidualContent string                  `json:"residual_content"`
	Clarity         float64                 `json:"clarity"`
	Score           float64                 `json:"score"`
	Strength        float64                 `json:"strength"`
	Freshness       float64                 `json:"freshness"`
	Fuzziness       float64                 `json:"fuzziness"`
	DecayStage      string                  `json:"decay_stage"`
	LastAccessedAt  string                  `json:"last_accessed_at"`
	NeedsGrounding  bool                    `json:"needs_grounding"`
	SourceRefs      []model.SourceRef       `json:"source_refs,omitempty"`
	WhyRecalled     []string                `json:"why_recalled,omitempty"`
	Evidence        []engine.RecallEvidence `json:"evidence,omitempty"`
	ConflictGroup   string                  `json:"conflict_group,omitempty"`
	Version         int                     `json:"version,omitempty"`
	LifecycleState  string                  `json:"lifecycle_state,omitempty"`
	ConflictWarning bool                    `json:"conflict_warning,omitempty"`
	Suppressed      []int                   `json:"suppressed_versions,omitempty"`
}

// RecallResponse is the response for recall.
type RecallResponse struct {
	Results []RecallItem `json:"results"`
}

type EntitySearchItem struct {
	ID            string `json:"id"`
	CanonicalName string `json:"canonical_name"`
	DisplayName   string `json:"display_name"`
	EntityType    string `json:"entity_type"`
	MemoryCount   int    `json:"memory_count,omitempty"`
}

type ImageItem struct {
	ID              string   `json:"id"`
	FilePath        string   `json:"file_path,omitempty"`
	URL             string   `json:"url,omitempty"`
	SHA256          string   `json:"sha256"`
	SourceSession   string   `json:"source_session,omitempty"`
	SourceKind      string   `json:"source_kind,omitempty"`
	SourceActor     string   `json:"source_actor,omitempty"`
	CreatedAt       string   `json:"created_at,omitempty"`
	UpdatedAt       string   `json:"updated_at,omitempty"`
	Caption         string   `json:"caption,omitempty"`
	Tags            []string `json:"tags,omitempty"`
	OCRText         string   `json:"ocr_text,omitempty"`
	LinkedMemoryIDs []string `json:"linked_memory_ids,omitempty"`
}

type ImageRecallItem struct {
	ImageItem
	Score        float64 `json:"score"`
	VectorScore  float64 `json:"vector_score,omitempty"`
	VectorRank   int     `json:"vector_rank,omitempty"`
	KeywordScore float64 `json:"keyword_score,omitempty"`
	KeywordRank  int     `json:"keyword_rank,omitempty"`
	LinkedBoost  float64 `json:"linked_boost,omitempty"`
}

type ImageListResponse struct {
	Query string      `json:"query,omitempty"`
	Count int         `json:"count"`
	Items []ImageItem `json:"items"`
}

type ToolRecallMixedItem struct {
	ObjectType string  `json:"object_type"`
	ID         string  `json:"id"`
	Score      float64 `json:"score"`
	Title      string  `json:"title,omitempty"`
	Summary    string  `json:"summary,omitempty"`
}

type ToolRecallMemoryItem struct {
	ID             string  `json:"id"`
	Summary        string  `json:"summary"`
	Score          float64 `json:"score"`
	NeedsGrounding bool    `json:"needs_grounding"`
	LifecycleState string  `json:"lifecycle_state,omitempty"`
}

type ToolRecallImageItem struct {
	ID      string   `json:"id"`
	Caption string   `json:"caption,omitempty"`
	Tags    []string `json:"tags,omitempty"`
	Score   float64  `json:"score"`
}

type ToolRecallEntityItem struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	EntityType  string `json:"entity_type"`
	MemoryCount int    `json:"memory_count,omitempty"`
}

type ToolForgetMatchItem struct {
	ObjectType string `json:"object_type"`
	ID         string `json:"id"`
	Title      string `json:"title,omitempty"`
	Summary    string `json:"summary,omitempty"`
}

type ToolStoreResponse struct {
	Status         string `json:"status"`
	ObjectType     string `json:"object_type"`
	ID             string `json:"id"`
	Duplicate      bool   `json:"duplicate,omitempty"`
	Title          string `json:"title,omitempty"`
	Summary        string `json:"summary,omitempty"`
	LifecycleState string `json:"lifecycle_state,omitempty"`
}

type ToolRecallResponse struct {
	Query    string                 `json:"query"`
	Count    int                    `json:"count"`
	Mixed    []ToolRecallMixedItem  `json:"mixed"`
	Memories []ToolRecallMemoryItem `json:"memories"`
	Images   []ToolRecallImageItem  `json:"images"`
	Entities []ToolRecallEntityItem `json:"entities"`
}

type WorkbenchQueryResponse struct {
	Query    string                `json:"query"`
	Count    int                   `json:"count"`
	Mixed    []ToolRecallMixedItem `json:"mixed"`
	Results  []RecallItem          `json:"results,omitempty"`
	Memories []RecallItem          `json:"memories"`
	Images   []ImageRecallItem     `json:"images"`
	Entities []EntitySearchItem    `json:"entities"`
	Explain  *engine.ExplainResult `json:"explain,omitempty"`
}

type EntityListResponse struct {
	Query string          `json:"query,omitempty"`
	Count int             `json:"count"`
	Items []*model.Entity `json:"items"`
}

type EntityMemoryResponse struct {
	Entity   *model.Entity               `json:"entity"`
	Count    int                         `json:"count"`
	Memories []engine.EntityMemoryResult `json:"memories"`
}

type EntityRelationsResponse struct {
	Entity    *model.Entity     `json:"entity"`
	Count     int               `json:"count"`
	Relations []*model.Relation `json:"relations"`
}

type explainRequest struct {
	Query      string  `json:"query"`
	K          int     `json:"k,omitempty"`
	MinClarity float64 `json:"min_clarity,omitempty"`
}

type toolRecallRequest struct {
	Query string `json:"query"`
	K     int    `json:"k,omitempty"`
}

type toolStoreRequest struct {
	ObjectType      string            `json:"object_type,omitempty"`
	Content         string            `json:"content"`
	Summary         string            `json:"summary,omitempty"`
	Importance      float64           `json:"importance,omitempty"`
	MemoryType      string            `json:"memory_type,omitempty"`
	ConflictGroup   string            `json:"conflict_group,omitempty"`
	SourceKind      string            `json:"source_kind,omitempty"`
	SourceRef       string            `json:"source_ref,omitempty"`
	SourceSession   string            `json:"source_session,omitempty"`
	SourceActor     string            `json:"source_actor,omitempty"`
	FilePath        string            `json:"file_path,omitempty"`
	URL             string            `json:"url,omitempty"`
	Caption         string            `json:"caption,omitempty"`
	Tags            []string          `json:"tags,omitempty"`
	OCRText         string            `json:"ocr_text,omitempty"`
	LinkedMemoryIDs []string          `json:"linked_memory_ids,omitempty"`
	Links           []model.ImageLink `json:"links,omitempty"`
}

type toolForgetRequest struct {
	ID         string `json:"id"`
	Query      string `json:"query,omitempty"`
	K          int    `json:"k,omitempty"`
	ObjectType string `json:"object_type,omitempty"`
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.URL.Path {
	case "/v1/healthz", "/v1/readyz":
		if r.Method != http.MethodGet {
			writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	case "/v1/memories", "/v1/memories/":
		switch r.Method {
		case http.MethodPost:
			s.handleStore(w, r)
		case http.MethodGet:
			s.handleRecall(w, r)
		default:
			writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	case "/v1/memories/explain":
		if r.Method != http.MethodPost {
			writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleExplain(w, r)
	case "/v1/tools/recall":
		if r.Method != http.MethodPost {
			writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleToolRecall(w, r)
	case "/v1/tools/store":
		if r.Method != http.MethodPost {
			writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleToolStore(w, r)
	case "/v1/tools/forget":
		if r.Method != http.MethodPost {
			writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleToolForget(w, r)
	case "/v1/memories/decay":
		if r.Method != http.MethodPost {
			writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleDecay(w, r)
	case "/v1/dashboard/stats/overview":
		if r.Method != http.MethodGet {
			writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleStatsOverview(w, r)
	case "/v1/dashboard/stats/integrity":
		if r.Method != http.MethodGet {
			writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleStatsIntegrity(w, r)
	case "/v1/dashboard/stats/detail":
		if r.Method != http.MethodGet {
			writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleStatsDetail(w, r)
	case "/v1/dashboard/images", "/v1/dashboard/images/":
		if r.Method != http.MethodGet {
			writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleImages(w, r)
	case "/v1/dashboard/entities", "/v1/dashboard/entities/":
		if r.Method != http.MethodGet {
			writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleEntities(w, r)
	case "/v1/dashboard/stats/images":
		if r.Method != http.MethodGet {
			writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleStatsImages(w, r)
	case "/v1/dashboard/stats/entities":
		if r.Method != http.MethodGet {
			writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleStatsEntities(w, r)
	case "/v1/dashboard/workbench/query":
		if r.Method != http.MethodPost {
			writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleDashboardWorkbenchQuery(w, r)
	default:
		if strings.HasPrefix(r.URL.Path, "/v1/memories/") {
			s.handleMemorySubresource(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/v1/dashboard/entities/") {
			s.handleEntitySubresource(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/v1/dashboard/images/") {
			s.handleImageSubresource(w, r)
			return
		}
		http.NotFound(w, r)
	}
}

func writeJSONError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func (s *Server) handleStore(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req StoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if strings.Contains(err.Error(), "too large") {
			writeJSONError(w, "request body too large", http.StatusRequestEntityTooLarge)
		} else {
			writeJSONError(w, "invalid json", http.StatusBadRequest)
		}
		return
	}

	content := strings.TrimSpace(req.Content)
	if content == "" {
		writeJSONError(w, "content required", http.StatusBadRequest)
		return
	}
	if req.Importance == 0 {
		req.Importance = 0.5
	}

	m, err := s.engine.Remember(r.Context(), engine.RememberRequest{
		Content:       content,
		Summary:       strings.TrimSpace(req.Summary),
		Importance:    req.Importance,
		MemoryType:    strings.TrimSpace(req.MemoryType),
		ConflictGroup: strings.TrimSpace(req.ConflictGroup),
		SourceRefs:    req.SourceRefs,
	})
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(StoreResponse{
		ID:             m.ID,
		Summary:        m.Summary,
		LifecycleState: m.LifecycleState,
		ConflictGroup:  m.ConflictGroup,
		Version:        m.Version,
	})
}

func (s *Server) handleToolStore(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req toolStoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid json", http.StatusBadRequest)
		return
	}
	result, err := s.engine.StoreTool(r.Context(), engine.ToolStoreRequest{
		ObjectType:      strings.TrimSpace(req.ObjectType),
		Content:         strings.TrimSpace(req.Content),
		Summary:         strings.TrimSpace(req.Summary),
		Importance:      req.Importance,
		MemoryType:      strings.TrimSpace(req.MemoryType),
		ConflictGroup:   strings.TrimSpace(req.ConflictGroup),
		SourceKind:      strings.TrimSpace(req.SourceKind),
		SourceRef:       strings.TrimSpace(req.SourceRef),
		SourceSession:   strings.TrimSpace(req.SourceSession),
		SourceActor:     strings.TrimSpace(req.SourceActor),
		FilePath:        strings.TrimSpace(req.FilePath),
		URL:             strings.TrimSpace(req.URL),
		Caption:         strings.TrimSpace(req.Caption),
		Tags:            req.Tags,
		OCRText:         strings.TrimSpace(req.OCRText),
		LinkedMemoryIDs: req.LinkedMemoryIDs,
		Links:           req.Links,
	})
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	resp := ToolStoreResponse{
		Status:     result.Status,
		ObjectType: result.ObjectType,
		ID:         result.ID,
		Duplicate:  result.Duplicate,
	}
	if result.Memory != nil {
		resp.Title = strings.TrimSpace(result.Memory.Summary)
		if resp.Title == "" {
			resp.Title = strings.TrimSpace(result.Memory.Content)
		}
		resp.Summary = strings.TrimSpace(result.Memory.ResidualContent)
		if resp.Summary == "" {
			resp.Summary = strings.TrimSpace(result.Memory.Summary)
		}
		resp.LifecycleState = strings.TrimSpace(result.Memory.LifecycleState)
	}
	if result.Image != nil {
		resp.Title = strings.TrimSpace(result.Image.Caption)
		if resp.Title == "" {
			resp.Title = strings.TrimSpace(result.Image.FilePath)
		}
		if resp.Title == "" {
			resp.Title = strings.TrimSpace(result.Image.URL)
		}
		resp.Summary = strings.TrimSpace(result.Image.OCRText)
		if resp.Summary == "" {
			resp.Summary = strings.TrimSpace(strings.Join(result.Image.Tags, ", "))
		}
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleRecall(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		writeJSONError(w, "q required", http.StatusBadRequest)
		return
	}

	k := 5
	if v := r.URL.Query().Get("k"); v != "" {
		if n, _ := strconv.Atoi(v); n > 0 {
			k = n
			if k > maxRecallK {
				k = maxRecallK
			}
		}
	}

	results, err := s.engine.Recall(r.Context(), query, k, 0)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	items := make([]RecallItem, len(results))
	for i, result := range results {
		items[i] = recallItemFromResult(result)
	}

	_ = json.NewEncoder(w).Encode(RecallResponse{Results: items})
}

func (s *Server) handleToolRecall(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req toolRecallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid json", http.StatusBadRequest)
		return
	}
	query := strings.TrimSpace(req.Query)
	if query == "" {
		writeJSONError(w, "query required", http.StatusBadRequest)
		return
	}
	k := req.K
	if k <= 0 {
		k = 5
	}
	if k > maxRecallK {
		k = maxRecallK
	}
	results, err := s.engine.RecallTool(r.Context(), query, k, false)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(toolRecallResponseFromEngine(results))
}

func recallItemFromResult(result engine.RecallResult) RecallItem {
	return RecallItem{
		ID:              result.Memory.ID,
		Content:         result.Memory.Content,
		Summary:         result.Summary,
		ResidualContent: result.Memory.ResidualContent,
		Clarity:         result.Memory.Clarity,
		Score:           result.Score,
		Strength:        result.Strength,
		Freshness:       result.Freshness,
		Fuzziness:       result.Fuzziness,
		DecayStage:      result.DecayStage,
		LastAccessedAt:  result.LastAccessedAt.Format(time.RFC3339),
		NeedsGrounding:  result.NeedsGrounding,
		SourceRefs:      result.SourceRefs,
		WhyRecalled:     result.WhyRecalled,
		Evidence:        result.Evidence,
		ConflictGroup:   result.ConflictGroup,
		Version:         result.Version,
		LifecycleState:  result.LifecycleState,
		ConflictWarning: result.ConflictWarn,
		Suppressed:      result.Suppressed,
	}
}

func (s *Server) handleReinforce(w http.ResponseWriter, r *http.Request, id string) {
	m, err := s.engine.Reinforce(r.Context(), id)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if m == nil {
		writeJSONError(w, "memory not found", http.StatusNotFound)
		return
	}

	_ = json.NewEncoder(w).Encode(StoreResponse{
		ID:             m.ID,
		Summary:        m.Summary,
		LifecycleState: m.LifecycleState,
		ConflictGroup:  m.ConflictGroup,
		Version:        m.Version,
	})
}

func (s *Server) handleForget(w http.ResponseWriter, r *http.Request, id string) {
	if err := s.engine.Forget(r.Context(), id); err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "forgotten", "id": id})
}

func (s *Server) handleToolForget(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req toolForgetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid json", http.StatusBadRequest)
		return
	}
	id := strings.TrimSpace(req.ID)
	query := strings.TrimSpace(req.Query)
	if id == "" && query == "" {
		writeJSONError(w, "id or query required", http.StatusBadRequest)
		return
	}
	result, err := s.engine.ForgetTool(r.Context(), req.ObjectType, id, query, req.K)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	resp := map[string]interface{}{
		"status":      result.Status,
		"id":          result.ID,
		"object_type": result.ObjectType,
	}
	if result.Query != "" {
		resp["query"] = result.Query
	}
	if result.Match != nil {
		resp["match"] = ToolForgetMatchItem{
			ObjectType: result.Match.ObjectType,
			ID:         result.Match.ID,
			Title:      result.Match.Title,
			Summary:    result.Match.Summary,
		}
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleDecay(w http.ResponseWriter, r *http.Request) {
	if err := s.engine.DecayAll(r.Context()); err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "decayed"})
}

func (s *Server) handleExplain(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req explainRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid json", http.StatusBadRequest)
		return
	}

	req.Query = strings.TrimSpace(req.Query)
	if req.Query == "" {
		writeJSONError(w, "query required", http.StatusBadRequest)
		return
	}

	k := req.K
	if k <= 0 {
		k = 5
	}

	result, err := s.engine.Explain(r.Context(), req.Query, k, req.MinClarity)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	items := make([]RecallItem, len(result.Accepted))
	for i, accepted := range result.Accepted {
		items[i] = RecallItem{
			ID:              accepted.MemoryID,
			Content:         accepted.Memory.Content,
			Summary:         accepted.Summary,
			ResidualContent: accepted.Memory.ResidualContent,
			Clarity:         accepted.Memory.Clarity,
			Score:           accepted.Score,
			Strength:        accepted.Strength,
			Freshness:       accepted.Freshness,
			Fuzziness:       accepted.Fuzziness,
			DecayStage:      accepted.DecayStage,
			LastAccessedAt:  accepted.LastAccessedAt.Format(time.RFC3339),
			NeedsGrounding:  accepted.NeedsGrounding,
			SourceRefs:      accepted.SourceRefs,
			WhyRecalled:     accepted.WhyRecalled,
			Evidence:        accepted.Evidence,
			ConflictGroup:   accepted.ConflictGroup,
			Version:         accepted.Version,
			LifecycleState:  accepted.LifecycleState,
			ConflictWarning: accepted.ConflictWarn,
			Suppressed:      accepted.Suppressed,
		}
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"query":    result.Query,
		"accepted": items,
		"filtered": result.Filtered,
	})
}

func (s *Server) handleStatsOverview(w http.ResponseWriter, r *http.Request) {
	windowDays := 30
	topK := 10
	riskWClarity := 0.7
	riskWIdle := 0.3
	if v := strings.TrimSpace(r.URL.Query().Get("window_days")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			writeJSONError(w, "invalid window_days", http.StatusBadRequest)
			return
		}
		windowDays = n
	}
	if v := strings.TrimSpace(r.URL.Query().Get("top_k")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			writeJSONError(w, "invalid top_k", http.StatusBadRequest)
			return
		}
		topK = n
	}
	if v := strings.TrimSpace(r.URL.Query().Get("risk_w_clarity")); v != "" {
		n, err := strconv.ParseFloat(v, 64)
		if err != nil || n < 0 {
			writeJSONError(w, "invalid risk_w_clarity", http.StatusBadRequest)
			return
		}
		riskWClarity = n
	}
	if v := strings.TrimSpace(r.URL.Query().Get("risk_w_idle")); v != "" {
		n, err := strconv.ParseFloat(v, 64)
		if err != nil || n < 0 {
			writeJSONError(w, "invalid risk_w_idle", http.StatusBadRequest)
			return
		}
		riskWIdle = n
	}
	stats, err := s.engine.StatsOverviewWithOptions(r.Context(), engine.OverviewOptions{
		WindowDays:    windowDays,
		TopK:          topK,
		RiskWClarity:  riskWClarity,
		RiskWIdleDays: riskWIdle,
	})
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(stats)
}

func (s *Server) handleStatsIntegrity(w http.ResponseWriter, r *http.Request) {
	sampleSize := 200
	if v := strings.TrimSpace(r.URL.Query().Get("sample_size")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			writeJSONError(w, "invalid sample_size", http.StatusBadRequest)
			return
		}
		sampleSize = n
	}
	stats, err := s.engine.StatsIntegrity(r.Context(), sampleSize)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(stats)
}

func (s *Server) handleStatsDetail(w http.ResponseWriter, r *http.Request) {
	windowDays, ok := parsePositiveIntQuery(r, "window_days", 30)
	if !ok {
		writeJSONError(w, "invalid window_days", http.StatusBadRequest)
		return
	}
	topK, ok := parsePositiveIntQuery(r, "top_k", 10)
	if !ok {
		writeJSONError(w, "invalid top_k", http.StatusBadRequest)
		return
	}
	sampleSize, ok := parsePositiveIntQuery(r, "sample_size", 200)
	if !ok {
		writeJSONError(w, "invalid sample_size", http.StatusBadRequest)
		return
	}
	stats, err := s.engine.StatsDetail(r.Context(), windowDays, topK, sampleSize)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(stats)
}

func (s *Server) handleDashboardWorkbenchQuery(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req toolRecallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid json", http.StatusBadRequest)
		return
	}
	query := strings.TrimSpace(req.Query)
	if query == "" {
		writeJSONError(w, "query required", http.StatusBadRequest)
		return
	}
	k := req.K
	if k <= 0 {
		k = 5
	}
	if k > maxRecallK {
		k = maxRecallK
	}
	result, err := s.engine.RecallTool(r.Context(), query, k, true)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	resp := workbenchRecallResponseFromEngine(result)
	resp.Explain = result.Explain
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleImages(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	limit, ok := parsePositiveIntQuery(r, "limit", 20)
	if !ok {
		writeJSONError(w, "invalid limit", http.StatusBadRequest)
		return
	}
	assets, err := s.engine.ListImages(r.Context(), query, limit)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	items := make([]ImageItem, 0, len(assets))
	for _, asset := range assets {
		links, err := s.engine.ImageLinks(r.Context(), asset.ID)
		if err != nil {
			writeJSONError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		items = append(items, imageItemFromAsset(asset, links))
	}
	_ = json.NewEncoder(w).Encode(ImageListResponse{
		Query: query,
		Count: len(items),
		Items: items,
	})
}

func (s *Server) handleStatsImages(w http.ResponseWriter, r *http.Request) {
	topK, ok := parsePositiveIntQuery(r, "top_k", 10)
	if !ok {
		writeJSONError(w, "invalid top_k", http.StatusBadRequest)
		return
	}
	stats, err := s.engine.StatsImages(r.Context(), topK)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(stats)
}

func (s *Server) handleEntities(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	limit, ok := parsePositiveIntQuery(r, "limit", 20)
	if !ok {
		writeJSONError(w, "invalid limit", http.StatusBadRequest)
		return
	}
	items, err := s.engine.ListEntities(r.Context(), query, limit)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(EntityListResponse{
		Query: query,
		Count: len(items),
		Items: items,
	})
}

func (s *Server) handleStatsEntities(w http.ResponseWriter, r *http.Request) {
	topK, ok := parsePositiveIntQuery(r, "top_k", 10)
	if !ok {
		writeJSONError(w, "invalid top_k", http.StatusBadRequest)
		return
	}
	stats, err := s.engine.StatsEntities(r.Context(), topK)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(stats)
}

func (s *Server) handleEntitySubresource(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/dashboard/entities/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := parts[0]
	entityValue, err := s.engine.GetEntity(r.Context(), id)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if entityValue == nil {
		writeJSONError(w, "entity not found", http.StatusNotFound)
		return
	}

	if len(parts) == 1 {
		_ = json.NewEncoder(w).Encode(entityValue)
		return
	}
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}

	limit, ok := parsePositiveIntQuery(r, "limit", 20)
	if !ok {
		writeJSONError(w, "invalid limit", http.StatusBadRequest)
		return
	}
	switch parts[1] {
	case "relations":
		relations, err := s.engine.EntityRelations(r.Context(), id, limit)
		if err != nil {
			writeJSONError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(EntityRelationsResponse{
			Entity:    entityValue,
			Count:     len(relations),
			Relations: relations,
		})
	case "memories":
		memories, err := s.engine.EntityMemories(r.Context(), id, limit)
		if err != nil {
			writeJSONError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(EntityMemoryResponse{
			Entity:   entityValue,
			Count:    len(memories),
			Memories: memories,
		})
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleImageSubresource(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/dashboard/images/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	imageID := parts[0]
	asset, err := s.engine.GetImage(r.Context(), imageID)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if asset == nil {
		writeJSONError(w, "image not found", http.StatusNotFound)
		return
	}
	links, err := s.engine.ImageLinks(r.Context(), imageID)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(parts) == 1 {
		_ = json.NewEncoder(w).Encode(imageItemFromAsset(asset, links))
		return
	}
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	switch parts[1] {
	case "links":
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":    imageID,
			"count": len(links),
			"links": links,
		})
	default:
		http.NotFound(w, r)
	}
}

func parsePositiveIntQuery(r *http.Request, key string, fallback int) (int, bool) {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return fallback, true
	}
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

func toolRecallResponseFromEngine(result *engine.FederatedRecallResult) ToolRecallResponse {
	memories := make([]ToolRecallMemoryItem, 0, len(result.Memories))
	for _, item := range result.Memories {
		memories = append(memories, ToolRecallMemoryItem{
			ID:             item.MemoryID,
			Summary:        item.Summary,
			Score:          item.Score,
			NeedsGrounding: item.NeedsGrounding,
			LifecycleState: item.LifecycleState,
		})
	}
	images := make([]ToolRecallImageItem, 0, len(result.Images))
	for _, item := range result.Images {
		caption := ""
		tags := []string(nil)
		if item.Asset != nil {
			caption = item.Asset.Caption
			tags = item.Asset.Tags
		}
		images = append(images, ToolRecallImageItem{
			ID:      item.ImageID,
			Caption: caption,
			Tags:    tags,
			Score:   item.Score,
		})
	}
	entities := make([]ToolRecallEntityItem, 0, len(result.Entities))
	for _, item := range result.Entities {
		if item == nil {
			continue
		}
		displayName := item.DisplayName
		if strings.TrimSpace(displayName) == "" {
			displayName = item.CanonicalName
		}
		if strings.TrimSpace(displayName) == "" {
			displayName = item.ID
		}
		entities = append(entities, ToolRecallEntityItem{
			ID:          item.ID,
			DisplayName: displayName,
			EntityType:  item.EntityType,
			MemoryCount: item.MemoryCount,
		})
	}
	mixed := make([]ToolRecallMixedItem, 0, len(result.Mixed))
	for _, item := range result.Mixed {
		mixed = append(mixed, ToolRecallMixedItem{
			ObjectType: item.ObjectType,
			ID:         item.ID,
			Score:      item.Score,
			Title:      item.Title,
			Summary:    item.Summary,
		})
	}
	return ToolRecallResponse{
		Query:    result.Query,
		Count:    len(mixed),
		Mixed:    mixed,
		Memories: memories,
		Images:   images,
		Entities: entities,
	}
}

func workbenchRecallResponseFromEngine(result *engine.FederatedRecallResult) WorkbenchQueryResponse {
	memories := make([]RecallItem, 0, len(result.Memories))
	for _, item := range result.Memories {
		memories = append(memories, recallItemFromResult(item))
	}
	images := make([]ImageRecallItem, 0, len(result.Images))
	for _, item := range result.Images {
		images = append(images, imageRecallItemFromResult(item))
	}
	entities := make([]EntitySearchItem, 0, len(result.Entities))
	for _, item := range result.Entities {
		entities = append(entities, entitySearchItemFromModel(item))
	}
	mixed := make([]ToolRecallMixedItem, 0, len(result.Mixed))
	for _, item := range result.Mixed {
		mixed = append(mixed, ToolRecallMixedItem{
			ObjectType: item.ObjectType,
			ID:         item.ID,
			Score:      item.Score,
			Title:      item.Title,
			Summary:    item.Summary,
		})
	}
	return WorkbenchQueryResponse{
		Query:    result.Query,
		Count:    len(mixed),
		Mixed:    mixed,
		Results:  memories,
		Memories: memories,
		Images:   images,
		Entities: entities,
	}
}

func recallItemFromMemory(memory *model.Memory, score float64) RecallItem {
	if memory == nil {
		return RecallItem{}
	}
	summary := strings.TrimSpace(memory.Summary)
	if summary == "" {
		summary = strings.TrimSpace(memory.ResidualContent)
	}
	if summary == "" {
		summary = strings.TrimSpace(memory.Content)
	}
	return RecallItem{
		ID:              memory.ID,
		Content:         memory.Content,
		Summary:         summary,
		ResidualContent: memory.ResidualContent,
		Clarity:         memory.Clarity,
		Score:           score,
		Strength:        memory.Clarity,
		Freshness:       memory.Clarity,
		Fuzziness:       memory.Fuzziness(),
		DecayStage:      memory.ResidualForm,
		LastAccessedAt:  memory.LastAccessedAt.Format(time.RFC3339),
		NeedsGrounding:  len(memory.SourceRefs) == 0,
		SourceRefs:      memory.SourceRefs,
		ConflictGroup:   memory.ConflictGroup,
		Version:         memory.Version,
		LifecycleState:  memory.LifecycleState,
	}
}

func entitySearchItemFromModel(entityValue *model.Entity) EntitySearchItem {
	if entityValue == nil {
		return EntitySearchItem{}
	}
	return EntitySearchItem{
		ID:            entityValue.ID,
		CanonicalName: entityValue.CanonicalName,
		DisplayName:   entityValue.DisplayName,
		EntityType:    entityValue.EntityType,
		MemoryCount:   entityValue.MemoryCount,
	}
}

func imageItemFromAsset(asset *model.ImageAsset, links []model.ImageLink) ImageItem {
	if asset == nil {
		return ImageItem{}
	}
	return ImageItem{
		ID:              asset.ID,
		FilePath:        asset.FilePath,
		URL:             asset.URL,
		SHA256:          asset.SHA256,
		SourceSession:   asset.SourceSession,
		SourceKind:      asset.SourceKind,
		SourceActor:     asset.SourceActor,
		CreatedAt:       asset.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       asset.UpdatedAt.Format(time.RFC3339),
		Caption:         asset.Caption,
		Tags:            asset.Tags,
		OCRText:         asset.OCRText,
		LinkedMemoryIDs: imageLinkedMemoryIDsFromLinks(links),
	}
}

func imageRecallItemFromResult(result model.ImageRecallResult) ImageRecallItem {
	item := imageItemFromAsset(result.Asset, nil)
	item.LinkedMemoryIDs = result.LinkedMemoryIDs
	return ImageRecallItem{
		ImageItem:    item,
		Score:        result.Score,
		VectorScore:  result.VectorScore,
		VectorRank:   result.VectorRank,
		KeywordScore: result.KeywordScore,
		KeywordRank:  result.KeywordRank,
		LinkedBoost:  result.LinkedBoost,
	}
}

func imageLinkedMemoryIDsFromLinks(links []model.ImageLink) []string {
	out := make([]string, 0, len(links))
	for _, link := range links {
		if link.LinkType == "memory" && strings.TrimSpace(link.TargetID) != "" {
			out = append(out, link.TargetID)
		}
	}
	return out
}

func (s *Server) handleMemorySubresource(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/memories/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}

	if len(parts) == 1 {
		id := parts[0]
		switch r.Method {
		case http.MethodGet:
			res, err := s.engine.Get(r.Context(), id)
			if err != nil {
				writeJSONError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if res == nil {
				writeJSONError(w, "memory not found", http.StatusNotFound)
				return
			}
			_ = json.NewEncoder(w).Encode(res)
		case http.MethodDelete:
			s.handleForget(w, r, id)
		default:
			writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}

	id, action := parts[0], parts[1]
	switch action {
	case "reinforce":
		if r.Method != http.MethodPost {
			writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleReinforce(w, r, id)
	case "ground", "reconstruct":
		if r.Method != http.MethodGet {
			writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		res, err := s.engine.Ground(r.Context(), id)
		if err != nil {
			writeJSONError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if res == nil {
			writeJSONError(w, "memory not found", http.StatusNotFound)
			return
		}
		if action == "reconstruct" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"memory_id":        res.MemoryID,
				"summary":          res.Summary,
				"residual_content": res.ResidualContent,
				"strength":         res.Strength,
				"decay_stage":      res.DecayStage,
			})
			return
		}
		_ = json.NewEncoder(w).Encode(res)
	case "versions":
		if r.Method != http.MethodGet {
			writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		results, err := s.engine.Versions(r.Context(), id)
		if err != nil {
			writeJSONError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if results == nil {
			writeJSONError(w, "memory not found", http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"results": results})
	default:
		http.NotFound(w, r)
	}
}
