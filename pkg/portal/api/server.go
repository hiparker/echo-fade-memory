package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/echo-fade-memory/echo-fade-memory/pkg/core/engine"
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

// StoreRequest is the request body for store.
type StoreRequest struct {
	Content    string  `json:"content"`
	Importance float64 `json:"importance,omitempty"`
}

// StoreResponse is the response for store.
type StoreResponse struct {
	ID string `json:"id"`
}

// RecallItem is a single recalled memory.
type RecallItem struct {
	ID              string  `json:"id"`
	Content         string  `json:"content"`
	ResidualContent string  `json:"residual_content"`
	Clarity         float64 `json:"clarity"`
	Score           float64 `json:"score"`
}

// RecallResponse is the response for recall.
type RecallResponse struct {
	Results []RecallItem `json:"results"`
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.URL.Path {
	case "/memories", "/memories/":
		switch r.Method {
		case http.MethodPost:
			s.handleStore(w, r)
		case http.MethodGet:
			s.handleRecall(w, r)
		default:
			writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	default:
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

	m, err := s.engine.Store(r.Context(), content, req.Importance)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(StoreResponse{ID: m.ID})
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
	for i, r := range results {
		items[i] = RecallItem{
			ID:              r.Memory.ID,
			Content:         r.Memory.Content,
			ResidualContent: r.Memory.ResidualContent,
			Clarity:         r.Memory.Clarity,
			Score:           r.Score,
		}
	}
	_ = json.NewEncoder(w).Encode(RecallResponse{Results: items})
}
