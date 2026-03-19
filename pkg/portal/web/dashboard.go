package web

import (
	"net/http"

	portalstatic "github.com/hiparker/echo-fade-memory/pkg/static"
)

// Handler serves the embedded dashboard page.
type Handler struct{}

// NewHandler creates a dashboard handler.
func NewHandler() *Handler {
	return &Handler{}
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.URL.Path != "/dashboard" && r.URL.Path != "/dashboard/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(portalstatic.DashboardHTML))
}
