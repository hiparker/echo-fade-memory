package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	coreengine "github.com/hiparker/echo-fade-memory/pkg/core/engine"
	"github.com/hiparker/echo-fade-memory/pkg/portal/api"
	"github.com/hiparker/echo-fade-memory/pkg/test/testutil"
)

func TestV1HealthzReturnsOK(t *testing.T) {
	srv := api.NewServer(testutil.NewTestEngine(t))

	req := httptest.NewRequest(http.MethodGet, "/v1/healthz", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestLegacyRoutesAreGone(t *testing.T) {
	srv := api.NewServer(testutil.NewTestEngine(t))

	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/remember"},
		{http.MethodGet, "/recall?q=test"},
		{http.MethodPost, "/reinforce"},
		{http.MethodPost, "/forget"},
		{http.MethodPost, "/explain"},
		{http.MethodPost, "/decay"},
		{http.MethodGet, "/memories"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s %s status = %d, want 404", tc.method, tc.path, rec.Code)
		}
	}
}

func TestMemoriesCollectionHappyPath(t *testing.T) {
	srv := api.NewServer(testutil.NewTestEngine(t))

	body := strings.NewReader(`{
		"content":"Project memory gateway contract is now resource-oriented",
		"summary":"memory gateway contract",
		"memory_type":"project",
		"source_refs":[{"kind":"chat","ref":"chat:1"}]
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/memories", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /v1/memories status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.ID == "" {
		t.Fatal("created id is empty")
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/memories?q=resource%20oriented", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/memories status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	var recall struct {
		Results []struct {
			ID         string `json:"id"`
			SourceRefs []struct {
				Ref string `json:"ref"`
			} `json:"source_refs"`
		} `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &recall); err != nil {
		t.Fatalf("decode recall response: %v", err)
	}
	if len(recall.Results) == 0 {
		t.Fatal("expected recall results")
	}
	if recall.Results[0].ID != created.ID {
		t.Fatalf("recalled id = %q, want %q", recall.Results[0].ID, created.ID)
	}
	if len(recall.Results[0].SourceRefs) == 0 || recall.Results[0].SourceRefs[0].Ref != "chat:1" {
		t.Fatalf("unexpected source refs: %+v", recall.Results[0].SourceRefs)
	}
}

func TestMemorySubresourcesHappyPath(t *testing.T) {
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

	for _, path := range []string{
		"/v1/memories/" + first.ID,
		"/v1/memories/" + first.ID + "/ground",
		"/v1/memories/" + first.ID + "/reconstruct",
		"/v1/memories/" + first.ID + "/versions",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d, want 200 body=%s", path, rec.Code, rec.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/memories/"+first.ID+"/reinforce", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST reinforce status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/v1/memories/"+first.ID, nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("DELETE memory status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
}

func TestExplainEndpointReturnsAcceptedAndFiltered(t *testing.T) {
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
	req := httptest.NewRequest(http.MethodPost, "/v1/memories/explain", bytes.NewReader(body))
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

func TestDecayEndpointReturnsOK(t *testing.T) {
	srv := api.NewServer(testutil.NewTestEngine(t))

	req := httptest.NewRequest(http.MethodPost, "/v1/memories/decay", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
}

func TestCollectionRejectsInvalidRequests(t *testing.T) {
	srv := api.NewServer(testutil.NewTestEngine(t))

	req := httptest.NewRequest(http.MethodPost, "/v1/memories", strings.NewReader("{invalid"))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid json status = %d, want 400", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/memories", strings.NewReader(`{"content":"   "}`))
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("blank content status = %d, want 400", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/memories", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing q status = %d, want 400", rec.Code)
	}
}

func TestRoutesReturn405And404(t *testing.T) {
	srv := api.NewServer(testutil.NewTestEngine(t))

	req := httptest.NewRequest(http.MethodDelete, "/v1/memories", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("DELETE collection status = %d, want 405", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/memories/missing/ground", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST ground status = %d, want 405", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/memories/missing", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET missing memory status = %d, want 404", rec.Code)
	}
}

func TestCreateRejectsOversizedBody(t *testing.T) {
	srv := api.NewServer(testutil.NewTestEngine(t))

	oversized := `{"content":"` + strings.Repeat("a", (1<<20)+32) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/memories", strings.NewReader(oversized))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413 body=%s", rec.Code, rec.Body.String())
	}
}
