package portal_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hiparker/echo-fade-memory/pkg/portal/api"
	"github.com/hiparker/echo-fade-memory/pkg/portal/web"
	"github.com/hiparker/echo-fade-memory/pkg/test/testutil"
)

func TestDashboardRouteReturnsHTML(t *testing.T) {
	mux := newPortalMux(t)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Fatalf("content-type = %q, want text/html", contentType)
	}
	if !strings.Contains(rec.Body.String(), "Echo Fade Memory Dashboard") {
		t.Fatalf("dashboard body missing expected title")
	}
}

func TestRootRedirectsToDashboard(t *testing.T) {
	mux := newPortalMux(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rec.Code)
	}
	if got := rec.Header().Get("Location"); got != "/dashboard" {
		t.Fatalf("location = %q, want /dashboard", got)
	}
}

func TestV1RouteStillWorksUnderMux(t *testing.T) {
	mux := newPortalMux(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/healthz", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Fatalf("unexpected health payload: %s", rec.Body.String())
	}
}

func TestStatsRoutesWorkUnderMux(t *testing.T) {
	mux := newPortalMux(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/stats/overview?window_days=30", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("overview status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/stats/integrity?sample_size=20", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("integrity status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
}

func newPortalMux(t *testing.T) *http.ServeMux {
	t.Helper()

	apiServer := api.NewServer(testutil.NewTestEngine(t))
	webServer := web.NewHandler()

	mux := http.NewServeMux()
	mux.Handle("/v1/", apiServer)
	mux.Handle("/dashboard", webServer)
	mux.Handle("/dashboard/", webServer)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/dashboard", http.StatusFound)
	})

	return mux
}
