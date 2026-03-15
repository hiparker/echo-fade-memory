package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	coreengine "github.com/echo-fade-memory/echo-fade-memory/pkg/core/engine"
	"github.com/echo-fade-memory/echo-fade-memory/pkg/portal/api"
	"github.com/echo-fade-memory/echo-fade-memory/pkg/test/testutil"
)

func TestHealthzReturnsOK(t *testing.T) {
	eng := testutil.NewTestEngine(t)
	srv := api.NewServer(eng)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestV1HealthzReturnsOK(t *testing.T) {
	eng := testutil.NewTestEngine(t)
	srv := api.NewServer(eng)

	req := httptest.NewRequest(http.MethodGet, "/v1/healthz", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestExplainReturnsAcceptedAndFiltered(t *testing.T) {
	eng := testutil.NewTestEngine(t)
	ctx := context.Background()

	_, err := eng.Remember(ctx, coreengine.RememberRequest{
		Content:       "project decision version one",
		Summary:       "project decision",
		MemoryType:    "project",
		ConflictGroup: "project:decision",
	})
	if err != nil {
		t.Fatalf("Remember older: %v", err)
	}
	_, err = eng.Remember(ctx, coreengine.RememberRequest{
		Content:       "project decision version two",
		Summary:       "project decision",
		MemoryType:    "project",
		ConflictGroup: "project:decision",
	})
	if err != nil {
		t.Fatalf("Remember newer: %v", err)
	}

	srv := api.NewServer(eng)
	body, _ := json.Marshal(map[string]interface{}{
		"query": "project decision",
		"k":     5,
	})
	req := httptest.NewRequest(http.MethodPost, "/explain", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Accepted []map[string]interface{} `json:"accepted"`
		Filtered []map[string]interface{} `json:"filtered"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Accepted) != 1 {
		t.Fatalf("accepted len = %d, want 1", len(payload.Accepted))
	}
	if len(payload.Filtered) == 0 {
		t.Fatal("filtered candidates should not be empty")
	}
}

func TestVersionsEndpointReturnsNewestFirst(t *testing.T) {
	eng := testutil.NewTestEngine(t)
	ctx := context.Background()

	first, err := eng.Remember(ctx, coreengine.RememberRequest{
		Content:       "project config old",
		Summary:       "project config",
		MemoryType:    "project",
		ConflictGroup: "project:config",
	})
	if err != nil {
		t.Fatalf("Remember first: %v", err)
	}
	_, err = eng.Remember(ctx, coreengine.RememberRequest{
		Content:       "project config new",
		Summary:       "project config",
		MemoryType:    "project",
		ConflictGroup: "project:config",
	})
	if err != nil {
		t.Fatalf("Remember second: %v", err)
	}

	srv := api.NewServer(eng)
	req := httptest.NewRequest(http.MethodGet, "/memories/"+first.ID+"/versions", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Results []struct {
			Version int `json:"version"`
		} `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode versions: %v", err)
	}
	if len(payload.Results) != 2 {
		t.Fatalf("versions len = %d, want 2", len(payload.Results))
	}
	if payload.Results[0].Version <= payload.Results[1].Version {
		t.Fatalf("versions not ordered newest first: %+v", payload.Results)
	}
}

func TestV1VersionsEndpointReturnsNewestFirst(t *testing.T) {
	eng := testutil.NewTestEngine(t)
	ctx := context.Background()

	first, err := eng.Remember(ctx, coreengine.RememberRequest{
		Content:       "project api old",
		Summary:       "project api",
		MemoryType:    "project",
		ConflictGroup: "project:api",
	})
	if err != nil {
		t.Fatalf("Remember first: %v", err)
	}
	_, err = eng.Remember(ctx, coreengine.RememberRequest{
		Content:       "project api new",
		Summary:       "project api",
		MemoryType:    "project",
		ConflictGroup: "project:api",
	})
	if err != nil {
		t.Fatalf("Remember second: %v", err)
	}

	srv := api.NewServer(eng)
	req := httptest.NewRequest(http.MethodGet, "/v1/memories/"+first.ID+"/versions", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
}

func TestDecayEndpointReturnsOK(t *testing.T) {
	eng := testutil.NewTestEngine(t)
	srv := api.NewServer(eng)

	req := httptest.NewRequest(http.MethodPost, "/decay", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
}
