package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestToolEndpointsHappyPath(t *testing.T) {
	srv := api.NewServer(testutil.NewTestEngine(t))

	storeBody, _ := json.Marshal(map[string]interface{}{
		"content":        "thin tool api keeps model calls small",
		"summary":        "thin tool api",
		"memory_type":    "project",
		"importance":     0.9,
		"source_kind":    "chat",
		"source_ref":     "chat:thin",
		"conflict_group": "tool:thin-api",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/tools/store", bytes.NewReader(storeBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /v1/tools/store status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	var stored struct {
		Status         string `json:"status"`
		ID             string `json:"id"`
		ObjectType     string `json:"object_type"`
		Title          string `json:"title"`
		Summary        string `json:"summary"`
		LifecycleState string `json:"lifecycle_state"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &stored); err != nil {
		t.Fatalf("decode store response: %v", err)
	}
	if stored.Status != "stored" || stored.ID == "" || stored.ObjectType != "memory" || stored.Title == "" {
		t.Fatalf("unexpected store response: %+v", stored)
	}

	recallBody, _ := json.Marshal(map[string]interface{}{
		"query": "model calls small",
	})
	req = httptest.NewRequest(http.MethodPost, "/v1/tools/recall", bytes.NewReader(recallBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /v1/tools/recall status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	var recall struct {
		Query    string `json:"query"`
		Count    int    `json:"count"`
		Memories []struct {
			ID             string `json:"id"`
			Summary        string `json:"summary"`
			NeedsGrounding bool   `json:"needs_grounding"`
		} `json:"memories"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &recall); err != nil {
		t.Fatalf("decode recall response: %v", err)
	}
	if recall.Query != "model calls small" || recall.Count == 0 || len(recall.Memories) == 0 {
		t.Fatalf("unexpected recall response: %+v", recall)
	}
	if recall.Memories[0].ID != stored.ID {
		t.Fatalf("recalled id = %q, want %q", recall.Memories[0].ID, stored.ID)
	}

	forgetBody, _ := json.Marshal(map[string]interface{}{"query": "model calls small"})
	req = httptest.NewRequest(http.MethodPost, "/v1/tools/forget", bytes.NewReader(forgetBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /v1/tools/forget status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
}

func TestUnifiedToolEndpointsSupportImages(t *testing.T) {
	srv := api.NewServer(testutil.NewTestEngine(t))

	imagePath := filepath.Join(t.TempDir(), "ops-dashboard.png")
	if err := os.WriteFile(imagePath, []byte("fake-image"), 0644); err != nil {
		t.Fatalf("write image: %v", err)
	}
	if err := os.WriteFile(imagePath+".ocr.txt", []byte("Latency Spike"), 0644); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}

	storeBody, _ := json.Marshal(map[string]interface{}{
		"object_type":    "image",
		"file_path":      imagePath,
		"source_session": "session:image-tool",
		"source_kind":    "chat_image",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/tools/store", bytes.NewReader(storeBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /v1/tools/store image status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	var stored struct {
		Status     string `json:"status"`
		ObjectType string `json:"object_type"`
		ID         string `json:"id"`
		Title      string `json:"title"`
		Summary    string `json:"summary"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &stored); err != nil {
		t.Fatalf("decode image store response: %v", err)
	}
	if stored.Status != "stored" || stored.ObjectType != "image" || stored.ID == "" || stored.Title == "" {
		t.Fatalf("unexpected image store response: %+v", stored)
	}

	recallBody, _ := json.Marshal(map[string]interface{}{"query": "latency spike"})
	req = httptest.NewRequest(http.MethodPost, "/v1/tools/recall", bytes.NewReader(recallBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /v1/tools/recall unified status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	var recall struct {
		Query  string `json:"query"`
		Count  int    `json:"count"`
		Images []struct {
			ID string `json:"id"`
		} `json:"images"`
		Mixed []struct {
			ObjectType string `json:"object_type"`
			ID         string `json:"id"`
		} `json:"mixed"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &recall); err != nil {
		t.Fatalf("decode unified recall response: %v", err)
	}
	if recall.Query != "latency spike" || recall.Count == 0 || len(recall.Images) == 0 || recall.Images[0].ID != stored.ID {
		t.Fatalf("unexpected unified recall response: %+v", recall)
	}

	forgetBody, _ := json.Marshal(map[string]interface{}{"query": "latency spike", "object_type": "image"})
	req = httptest.NewRequest(http.MethodPost, "/v1/tools/forget", bytes.NewReader(forgetBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /v1/tools/forget image status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
}

func TestToolForgetSupportsQuerySelection(t *testing.T) {
	srv := api.NewServer(testutil.NewTestEngine(t))

	storeBody, _ := json.Marshal(map[string]interface{}{
		"content":     "forget this deployment note after migration",
		"summary":     "temporary deployment note",
		"memory_type": "project",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/tools/store", bytes.NewReader(storeBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /v1/tools/store status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	forgetBody, _ := json.Marshal(map[string]interface{}{"query": "deployment note after migration"})
	req = httptest.NewRequest(http.MethodPost, "/v1/tools/forget", bytes.NewReader(forgetBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /v1/tools/forget by query status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Status     string `json:"status"`
		ID         string `json:"id"`
		ObjectType string `json:"object_type"`
		Query      string `json:"query"`
		Match      struct {
			ID         string `json:"id"`
			ObjectType string `json:"object_type"`
			Title      string `json:"title"`
			Summary    string `json:"summary"`
		} `json:"match"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode forget query response: %v", err)
	}
	if payload.Status != "forgotten" || payload.ID == "" || payload.ObjectType != "memory" || payload.Query == "" || payload.Match.ID != payload.ID || (payload.Match.Title == "" && payload.Match.Summary == "") {
		t.Fatalf("unexpected forget query response: %+v", payload)
	}
}

func TestEntityEndpointsHappyPath(t *testing.T) {
	srv := api.NewServer(testutil.NewTestEngine(t))

	create := func(content, summary, ref string) {
		t.Helper()
		body := strings.NewReader(fmt.Sprintf(`{
			"content": %q,
			"summary": %q,
			"memory_type": "project",
			"source_refs": [{"kind":"file","ref":%q}]
		}`, content, summary, ref))
		req := httptest.NewRequest(http.MethodPost, "/v1/memories", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("POST /v1/memories status = %d, want 200 body=%s", rec.Code, rec.Body.String())
		}
	}

	create("runbook covers deploy sequence", "deploy runbook", "docs/runbook.md")
	create("runbook covers rollback sequence", "rollback runbook", "docs/runbook.md")

	req := httptest.NewRequest(http.MethodGet, "/v1/dashboard/entities?q=runbook&limit=10", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/dashboard/entities status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	var list struct {
		Count int `json:"count"`
		Items []struct {
			ID            string `json:"id"`
			CanonicalName string `json:"canonical_name"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode entities list: %v", err)
	}
	if list.Count == 0 || len(list.Items) == 0 {
		t.Fatalf("expected entity results, got %+v", list)
	}
	entityID := list.Items[0].ID
	if entityID == "" {
		t.Fatal("entity id is empty")
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/dashboard/entities/"+entityID, nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/dashboard/entities/{id} status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/dashboard/entities/"+entityID+"/relations?limit=10", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/dashboard/entities/{id}/relations status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	var relations struct {
		Count int `json:"count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &relations); err != nil {
		t.Fatalf("decode entity relations: %v", err)
	}
	if relations.Count == 0 {
		t.Fatal("expected entity relations")
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/dashboard/entities/"+entityID+"/memories?limit=10", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/dashboard/entities/{id}/memories status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	var memories struct {
		Count    int `json:"count"`
		Memories []struct {
			MemoryID string `json:"memory_id"`
		} `json:"memories"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &memories); err != nil {
		t.Fatalf("decode entity memories: %v", err)
	}
	if memories.Count == 0 || len(memories.Memories) == 0 {
		t.Fatalf("expected entity memories, got %+v", memories)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/dashboard/stats/entities?top_k=5", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/dashboard/stats/entities status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	var stats struct {
		TotalEntities  int `json:"total_entities"`
		TotalRelations int `json:"total_relations"`
		TotalLinks     int `json:"total_links"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatalf("decode entity stats: %v", err)
	}
	if stats.TotalEntities == 0 || stats.TotalRelations == 0 || stats.TotalLinks == 0 {
		t.Fatalf("unexpected entity stats: %+v", stats)
	}
}

func TestImageEndpointsHappyPath(t *testing.T) {
	srv := api.NewServer(testutil.NewTestEngine(t))

	storeMemoryReq := httptest.NewRequest(http.MethodPost, "/v1/memories", strings.NewReader(`{"content":"decision about cat checklist","summary":"cat checklist decision","memory_type":"project"}`))
	storeMemoryRec := httptest.NewRecorder()
	srv.ServeHTTP(storeMemoryRec, storeMemoryReq)
	if storeMemoryRec.Code != http.StatusOK {
		t.Fatalf("store memory status = %d, want 200 body=%s", storeMemoryRec.Code, storeMemoryRec.Body.String())
	}
	var memory struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(storeMemoryRec.Body.Bytes(), &memory); err != nil {
		t.Fatalf("decode memory response: %v", err)
	}

	imagePath := filepath.Join(t.TempDir(), "cat-screenshot.png")
	if err := os.WriteFile(imagePath, []byte("fake-image"), 0644); err != nil {
		t.Fatalf("write image: %v", err)
	}
	if err := os.WriteFile(imagePath+".ocr.txt", []byte("Important screen text"), 0644); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}

	storeBody, _ := json.Marshal(map[string]interface{}{
		"object_type":       "image",
		"file_path":         imagePath,
		"source_session":    "session:image",
		"linked_memory_ids": []string{memory.ID},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/tools/store", bytes.NewReader(storeBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /v1/tools/store image status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	var stored struct {
		Status     string `json:"status"`
		ID         string `json:"id"`
		ObjectType string `json:"object_type"`
		Title      string `json:"title"`
		Summary    string `json:"summary"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &stored); err != nil {
		t.Fatalf("decode store_image response: %v", err)
	}
	if stored.Status != "stored" || stored.ID == "" || stored.ObjectType != "image" || stored.Title == "" {
		t.Fatalf("unexpected image store response: %+v", stored)
	}

	recallBody, _ := json.Marshal(map[string]interface{}{"query": "important screen text"})
	req = httptest.NewRequest(http.MethodPost, "/v1/tools/recall", bytes.NewReader(recallBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /v1/tools/recall unified status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	var recalled struct {
		Count  int `json:"count"`
		Images []struct {
			ID string `json:"id"`
		} `json:"images"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &recalled); err != nil {
		t.Fatalf("decode unified recall response: %v", err)
	}
	if recalled.Count == 0 || len(recalled.Images) == 0 || recalled.Images[0].ID != stored.ID {
		t.Fatalf("unexpected unified image recall response: %+v", recalled)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/dashboard/images?q=cat&limit=10", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/dashboard/images status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/dashboard/images/"+stored.ID, nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/dashboard/images/{id} status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/dashboard/images/"+stored.ID+"/links", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/dashboard/images/{id}/links status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/dashboard/stats/images?top_k=5", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/dashboard/stats/images status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	var stats struct {
		TotalImages     int `json:"total_images"`
		CaptionedImages int `json:"captioned_images"`
		OCRImages       int `json:"ocr_images"`
		TotalLinks      int `json:"total_links"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatalf("decode image stats response: %v", err)
	}
	if stats.TotalImages == 0 || stats.CaptionedImages == 0 || stats.OCRImages == 0 || stats.TotalLinks == 0 {
		t.Fatalf("unexpected image stats: %+v", stats)
	}
}

func TestDashboardWorkbenchQueryAndDetailStats(t *testing.T) {
	srv := api.NewServer(testutil.NewTestEngine(t))

	storeMemoryReq := httptest.NewRequest(http.MethodPost, "/v1/memories", strings.NewReader(`{"content":"runbook decision for latency incident","summary":"latency incident runbook","memory_type":"project","source_refs":[{"kind":"file","ref":"docs/runbook.md"}]}`))
	storeMemoryRec := httptest.NewRecorder()
	srv.ServeHTTP(storeMemoryRec, storeMemoryReq)
	if storeMemoryRec.Code != http.StatusOK {
		t.Fatalf("store memory status = %d, want 200 body=%s", storeMemoryRec.Code, storeMemoryRec.Body.String())
	}

	imagePath := filepath.Join(t.TempDir(), "latency-incident.png")
	if err := os.WriteFile(imagePath, []byte("fake-image"), 0644); err != nil {
		t.Fatalf("write image: %v", err)
	}
	if err := os.WriteFile(imagePath+".ocr.txt", []byte("Latency Incident"), 0644); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}
	storeImageBody, _ := json.Marshal(map[string]interface{}{
		"object_type": "image",
		"file_path":   imagePath,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/tools/store", bytes.NewReader(storeImageBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("store image status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	workbenchBody, _ := json.Marshal(map[string]interface{}{"query": "runbook latency incident", "k": 5})
	req = httptest.NewRequest(http.MethodPost, "/v1/dashboard/workbench/query", bytes.NewReader(workbenchBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /v1/dashboard/workbench/query status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	var workbench struct {
		Query    string `json:"query"`
		Count    int    `json:"count"`
		Memories []struct {
			ID string `json:"id"`
		} `json:"memories"`
		Images []struct {
			ID string `json:"id"`
		} `json:"images"`
		Entities []struct {
			ID string `json:"id"`
		} `json:"entities"`
		Explain *struct {
			Query string `json:"query"`
		} `json:"explain"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &workbench); err != nil {
		t.Fatalf("decode workbench response: %v", err)
	}
	if workbench.Query == "" || workbench.Count == 0 || len(workbench.Images) == 0 || workbench.Explain == nil {
		t.Fatalf("unexpected workbench response: %+v", workbench)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/dashboard/stats/detail?window_days=30&top_k=5&sample_size=20", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/dashboard/stats/detail status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	var detail struct {
		Overview *struct {
			TotalMemories int `json:"total_memories"`
		} `json:"overview"`
		Images *struct {
			TotalImages     int                      `json:"total_images"`
			CaptionedImages int                      `json:"captioned_images"`
			TopRecentImages []map[string]interface{} `json:"top_recent_images"`
		} `json:"images"`
		Entities *struct {
			TotalEntities int `json:"total_entities"`
		} `json:"entities"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode detail stats response: %v", err)
	}
	if detail.Overview == nil || detail.Images == nil || detail.Entities == nil {
		t.Fatalf("missing detail sections: %+v", detail)
	}
	if detail.Overview.TotalMemories == 0 || detail.Images.TotalImages == 0 || len(detail.Images.TopRecentImages) == 0 {
		t.Fatalf("unexpected detail stats payload: %+v", detail)
	}
}

func TestRemovedLegacyToolRoutesReturnNotFound(t *testing.T) {
	srv := api.NewServer(testutil.NewTestEngine(t))
	for _, tc := range []struct {
		path string
		body string
	}{
		{path: "/v1/tools/maintain", body: `{}`},
		{path: "/v1/tools/store_image", body: `{"file_path":"x"}`},
		{path: "/v1/tools/recall_image", body: `{"query":"x"}`},
		{path: "/v1/tools/link_image", body: `{"image_id":"x"}`},
	} {
		req := httptest.NewRequest(http.MethodPost, tc.path, bytes.NewReader([]byte(tc.body)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("POST %s status = %d, want 404 body=%s", tc.path, rec.Code, rec.Body.String())
		}
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

func TestStatsOverviewEndpointReturnsAggregates(t *testing.T) {
	srv := api.NewServer(testutil.NewTestEngine(t))

	create := func(content string) string {
		t.Helper()
		createReq := httptest.NewRequest(
			http.MethodPost,
			"/v1/memories",
			strings.NewReader(`{"content":"`+content+`","memory_type":"project"}`),
		)
		createRec := httptest.NewRecorder()
		srv.ServeHTTP(createRec, createReq)
		if createRec.Code != http.StatusOK {
			t.Fatalf("create status = %d, want 200 body=%s", createRec.Code, createRec.Body.String())
		}
		var out struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(createRec.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode create response: %v", err)
		}
		if out.ID == "" {
			t.Fatal("create response missing id")
		}
		return out.ID
	}
	idA := create("dashboard metrics memory A")
	time.Sleep(5 * time.Millisecond)
	idB := create("dashboard metrics memory B")
	time.Sleep(5 * time.Millisecond)
	idC := create("dashboard metrics memory C")

	// Increase access count for one memory to make top_accessed ordering deterministic.
	for i := 0; i < 3; i++ {
		reqReinforce := httptest.NewRequest(http.MethodPost, "/v1/memories/"+idA+"/reinforce", nil)
		recReinforce := httptest.NewRecorder()
		srv.ServeHTTP(recReinforce, reqReinforce)
		if recReinforce.Code != http.StatusOK {
			t.Fatalf("reinforce status = %d, want 200 body=%s", recReinforce.Code, recReinforce.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/dashboard/stats/overview?window_days=30&top_k=10", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		TotalMemories        int `json:"total_memories"`
		NewMemoriesWindow    int `json:"new_memories_window"`
		NewMemoriesToday     int `json:"new_memories_today"`
		NewMemoriesYesterday int `json:"new_memories_yesterday"`
		HighDecayRiskCount   int `json:"high_decay_risk_count"`
		TopNewMemories       []struct {
			ID string `json:"id"`
		} `json:"top_new_memories"`
		TopDecayRiskMemories []struct {
			ID        string  `json:"id"`
			RiskScore float64 `json:"risk_score"`
		} `json:"top_decay_risk_memories"`
		TopAccessedMemories []struct {
			ID          string `json:"id"`
			AccessCount int    `json:"access_count"`
		} `json:"top_accessed_memories"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.TotalMemories == 0 {
		t.Fatal("expected total_memories > 0")
	}
	if payload.NewMemoriesWindow == 0 {
		t.Fatal("expected new_memories_window > 0")
	}
	if payload.NewMemoriesToday == 0 {
		t.Fatal("expected new_memories_today > 0")
	}
	if payload.NewMemoriesYesterday < 0 {
		t.Fatal("expected non-negative new_memories_yesterday")
	}
	if payload.HighDecayRiskCount < 0 {
		t.Fatal("expected non-negative high_decay_risk_count")
	}
	if len(payload.TopNewMemories) == 0 {
		t.Fatal("expected top_new_memories not empty")
	}
	seenTopNew := map[string]bool{}
	for _, item := range payload.TopNewMemories {
		seenTopNew[item.ID] = true
	}
	if !seenTopNew[idA] || !seenTopNew[idB] || !seenTopNew[idC] {
		t.Fatalf("top_new_memories missing expected ids: %+v", payload.TopNewMemories)
	}
	if len(payload.TopDecayRiskMemories) == 0 {
		t.Fatal("expected top_decay_risk_memories not empty")
	}
	if len(payload.TopAccessedMemories) == 0 {
		t.Fatal("expected top_accessed_memories not empty")
	}
	if payload.TopAccessedMemories[0].ID != idA {
		t.Fatalf("top accessed first id = %q, want %q", payload.TopAccessedMemories[0].ID, idA)
	}
	if payload.TopAccessedMemories[0].AccessCount < 1 {
		t.Fatal("top accessed item should have access_count >= 1")
	}
}

func TestStatsIntegrityEndpointReturnsChecks(t *testing.T) {
	srv := api.NewServer(testutil.NewTestEngine(t))

	createReq := httptest.NewRequest(
		http.MethodPost,
		"/v1/memories",
		strings.NewReader(`{"content":"integrity check memory","memory_type":"project"}`),
	)
	createRec := httptest.NewRecorder()
	srv.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusOK {
		t.Fatalf("create status = %d, want 200 body=%s", createRec.Code, createRec.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/dashboard/stats/integrity?sample_size=50", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		SQLTotal      int    `json:"sql_total"`
		SampleChecked int    `json:"sample_checked"`
		Capability    string `json:"capability"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.SQLTotal == 0 {
		t.Fatal("expected sql_total > 0")
	}
	if payload.SampleChecked == 0 {
		t.Fatal("expected sample_checked > 0")
	}
	if payload.Capability == "" {
		t.Fatal("expected capability in payload")
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

	req = httptest.NewRequest(http.MethodGet, "/v1/dashboard/stats/overview?window_days=bad", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid window_days status = %d, want 400", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/dashboard/stats/integrity?sample_size=bad", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid sample_size status = %d, want 400", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/dashboard/stats/overview?top_k=bad", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid top_k status = %d, want 400", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/dashboard/stats/overview?risk_w_clarity=-1", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid risk_w_clarity status = %d, want 400", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/dashboard/stats/overview?risk_w_idle=-1", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid risk_w_idle status = %d, want 400", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/dashboard/entities?limit=bad", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid entities limit status = %d, want 400", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/dashboard/stats/entities?top_k=bad", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid entity top_k status = %d, want 400", rec.Code)
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

	req = httptest.NewRequest(http.MethodPost, "/v1/dashboard/stats/overview", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST stats overview status = %d, want 405", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/dashboard/stats/integrity", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST stats integrity status = %d, want 405", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/dashboard/entities", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST entities status = %d, want 405", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/dashboard/stats/entities", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST stats entities status = %d, want 405", rec.Code)
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

// TestLiveServerEndToEndFlow targets a running server (default http://127.0.0.1:8080).
// It is opt-in so CI/local unit runs stay hermetic.
func TestLiveServerEndToEndFlow(t *testing.T) {
	if os.Getenv("EFM_E2E_LIVE") != "1" {
		t.Skip("set EFM_E2E_LIVE=1 to run live end-to-end test against a running server")
	}

	baseURL := strings.TrimSpace(os.Getenv("EFM_E2E_BASE_URL"))
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8080"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	client := &http.Client{Timeout: 15 * time.Second}
	waitForLiveReady(t, client, baseURL)

	token := fmt.Sprintf("live-e2e-%d", time.Now().UnixNano())

	create := func(content, summary string, importance float64, refs []map[string]string) string {
		t.Helper()
		payload := map[string]interface{}{
			"content":     content,
			"summary":     summary,
			"memory_type": "preference",
			"importance":  importance,
			"source_refs": refs,
		}
		raw, _ := json.Marshal(payload)
		req, err := http.NewRequest(http.MethodPost, baseURL+"/v1/memories", bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("build create request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("POST /v1/memories failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("POST /v1/memories status=%d want=200", resp.StatusCode)
		}
		var out struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("decode create response: %v", err)
		}
		if out.ID == "" {
			t.Fatal("created id is empty")
		}
		return out.ID
	}

	idA := create(
		"I strongly prefer jasmine green tea in the morning for calm focus. "+token,
		"morning jasmine tea preference "+token,
		0.95,
		[]map[string]string{{"kind": "chat", "ref": "live:" + token + ":a"}},
	)
	idB := create(
		"I usually drink espresso after lunch for a quick energy boost. "+token,
		"afternoon espresso habit "+token,
		0.40,
		[]map[string]string{{"kind": "chat", "ref": "live:" + token + ":b"}},
	)

	// 1) Keyword recall: should return the jasmine memory.
	{
		recallURL := baseURL + "/v1/memories?q=" + url.QueryEscape("jasmine morning "+token) + "&k=5"
		resp, err := client.Get(recallURL)
		if err != nil {
			t.Fatalf("GET recall failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET recall status=%d want=200", resp.StatusCode)
		}
		var out struct {
			Results []struct {
				ID string `json:"id"`
			} `json:"results"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("decode recall response: %v", err)
		}
		if !containsID(out.Results, idA) {
			t.Fatalf("keyword recall did not return idA=%s", idA)
		}
	}

	// 2) Semantic-style recall (paraphrased query): should still surface idA.
	{
		recallURL := baseURL + "/v1/memories?q=" + url.QueryEscape("what drink helps me focus in early day "+token) + "&k=10"
		resp, err := client.Get(recallURL)
		if err != nil {
			t.Fatalf("GET semantic recall failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET semantic recall status=%d want=200", resp.StatusCode)
		}
		var out struct {
			Results []struct {
				ID string `json:"id"`
			} `json:"results"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("decode semantic recall response: %v", err)
		}
		if !containsID(out.Results, idA) {
			t.Fatalf("semantic recall did not return idA=%s", idA)
		}
	}

	// 3) Reinforce memory A then check detail fields.
	{
		req, _ := http.NewRequest(http.MethodPost, baseURL+"/v1/memories/"+idA+"/reinforce", nil)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("POST reinforce failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("POST reinforce status=%d want=200", resp.StatusCode)
		}

		resp, err = client.Get(baseURL + "/v1/memories/" + idA)
		if err != nil {
			t.Fatalf("GET memory detail failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET memory detail status=%d want=200", resp.StatusCode)
		}
		var detail struct {
			ID              string  `json:"id"`
			AccessCount     int     `json:"access_count"`
			Importance      float64 `json:"importance"`
			EmotionalWeight float64 `json:"emotional_weight"`
			LifecycleState  string  `json:"lifecycle_state"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
			t.Fatalf("decode memory detail: %v", err)
		}
		if detail.ID != idA {
			t.Fatalf("detail id=%s want=%s", detail.ID, idA)
		}
		if detail.AccessCount < 1 {
			t.Fatalf("access_count=%d want>=1 after reinforce", detail.AccessCount)
		}
		if detail.Importance < 0.9 {
			t.Fatalf("importance=%f want>=0.9", detail.Importance)
		}
		// Current API/engine contract has no input field for emotional_weight on create.
		// Verify the field exists and remains non-negative.
		if detail.EmotionalWeight < 0 {
			t.Fatalf("emotional_weight=%f want>=0", detail.EmotionalWeight)
		}
		if detail.LifecycleState != "reinforced" {
			t.Fatalf("lifecycle_state=%q want=reinforced", detail.LifecycleState)
		}
	}

	// 4) Ground endpoint returns source refs for memory A.
	{
		resp, err := client.Get(baseURL + "/v1/memories/" + idA + "/ground")
		if err != nil {
			t.Fatalf("GET ground failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET ground status=%d want=200", resp.StatusCode)
		}
		var out struct {
			MemoryID   string `json:"memory_id"`
			SourceRefs []struct {
				Ref string `json:"ref"`
			} `json:"source_refs"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("decode ground response: %v", err)
		}
		if out.MemoryID != idA {
			t.Fatalf("ground memory_id=%s want=%s", out.MemoryID, idA)
		}
		if len(out.SourceRefs) == 0 {
			t.Fatal("ground source_refs should not be empty")
		}
	}

	// 5) Delete both memories and verify they're gone.
	for _, id := range []string{idA, idB} {
		req, _ := http.NewRequest(http.MethodDelete, baseURL+"/v1/memories/"+id, nil)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("DELETE memory %s failed: %v", id, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("DELETE memory %s status=%d want=200", id, resp.StatusCode)
		}

		resp, err = client.Get(baseURL + "/v1/memories/" + id)
		if err != nil {
			t.Fatalf("GET deleted memory %s failed: %v", id, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("GET deleted memory %s status=%d want=404", id, resp.StatusCode)
		}
	}
}

func waitForLiveReady(t *testing.T, client *http.Client, baseURL string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for {
		resp, err := client.Get(baseURL + "/v1/healthz")
		if err == nil && resp != nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("live server not ready at %s/v1/healthz", baseURL)
		}
		time.Sleep(300 * time.Millisecond)
	}
}

func containsID(results []struct {
	ID string `json:"id"`
}, target string) bool {
	for _, item := range results {
		if item.ID == target {
			return true
		}
	}
	return false
}
