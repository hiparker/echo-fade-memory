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

type explainRequest struct {
	Query      string  `json:"query"`
	K          int     `json:"k,omitempty"`
	MinClarity float64 `json:"min_clarity,omitempty"`
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
	case "/v1/memories/decay":
		if r.Method != http.MethodPost {
			writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleDecay(w, r)
	default:
		if strings.HasPrefix(r.URL.Path, "/v1/memories/") {
			s.handleMemorySubresource(w, r)
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
		items[i] = RecallItem{
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

	_ = json.NewEncoder(w).Encode(RecallResponse{Results: items})
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
