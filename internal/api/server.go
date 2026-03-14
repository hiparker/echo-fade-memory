package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/echo-fade-memory/echo-fade-memory/internal/engine"
)

// Server provides HTTP API for the memory engine.
type Server struct {
	engine *engine.Engine
}

// NewServer creates a new API server.
func NewServer(eng *engine.Engine) *Server {
	return &Server{engine: eng}
}

// StoreRequest is the request body for store.
type StoreRequest struct {
	Content    string  `json:"content"`
	Importance float64 `json:"importance,omitempty"`
}

// StoreResponse is the response for store.
type StoreResponse struct {
	ID string `json:"id"`
}

// RecallRequest is the request for recall.
type RecallRequest struct {
	Query      string  `json:"query"`
	K          int     `json:"k,omitempty"`
	MinClarity float64 `json:"min_clarity,omitempty"`
}

// RecallResponse is the response for recall.
type RecallResponse struct {
	Results []RecallItem `json:"results"`
}

// RecallItem is a single recalled memory.
type RecallItem struct {
	ID             string  `json:"id"`
	Content        string  `json:"content"`
	ResidualContent string `json:"residual_content"`
	Clarity        float64 `json:"clarity"`
	Score          float64 `json:"score"`
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.URL.Path {
	case "/memories", "/memories/":
		if r.Method == http.MethodPost {
			s.handleStore(w, r)
			return
		}
		if r.Method == http.MethodGet {
			s.handleRecall(w, r)
			return
		}
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleStore(w http.ResponseWriter, r *http.Request) {
	var req StoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Content == "" {
		http.Error(w, `{"error":"content required"}`, http.StatusBadRequest)
		return
	}
	if req.Importance == 0 {
		req.Importance = 0.5
	}

	m, err := s.engine.Store(r.Context(), req.Content, req.Importance)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(StoreResponse{ID: m.ID})
}

func (s *Server) handleRecall(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, `{"error":"q required"}`, http.StatusBadRequest)
		return
	}
	k := 5
	if v := r.URL.Query().Get("k"); v != "" {
		if n, _ := parseInt(v); n > 0 {
			k = n
		}
	}

	results, err := s.engine.Recall(r.Context(), query, k, 0)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	items := make([]RecallItem, len(results))
	for i, r := range results {
		items[i] = RecallItem{
			ID:              r.Memory.ID,
			Content:         r.Memory.Content,
			ResidualContent: r.Memory.ResidualContent,
			Clarity:         r.Memory.Clarity,
			Score:           r.Score,
		}
	}
	json.NewEncoder(w).Encode(RecallResponse{Results: items})
}

func parseInt(s string) (int, bool) {
	n, err := strconv.Atoi(s)
	return n, err == nil
}
