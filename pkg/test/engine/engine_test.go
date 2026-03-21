package engine_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hiparker/echo-fade-memory/pkg/core/decay"
	coreengine "github.com/hiparker/echo-fade-memory/pkg/core/engine"
	"github.com/hiparker/echo-fade-memory/pkg/core/model"
	"github.com/hiparker/echo-fade-memory/pkg/core/transform"
	"github.com/hiparker/echo-fade-memory/pkg/test/testutil"
)

func TestExplainSuppressesOlderConflictVersions(t *testing.T) {
	eng := testutil.NewTestEngine(t)
	ctx := context.Background()

	older, err := eng.Remember(ctx, coreengine.RememberRequest{
		Content:       "project decision version one",
		Summary:       "project decision",
		MemoryType:    "project",
		ConflictGroup: "project:decision",
		Importance:    0.8,
	})
	if err != nil {
		t.Fatalf("Remember older: %v", err)
	}

	newer, err := eng.Remember(ctx, coreengine.RememberRequest{
		Content:       "project decision version two",
		Summary:       "project decision",
		MemoryType:    "project",
		ConflictGroup: "project:decision",
		Importance:    0.9,
	})
	if err != nil {
		t.Fatalf("Remember newer: %v", err)
	}

	result, err := eng.Explain(ctx, "project decision", 5, 0)
	if err != nil {
		t.Fatalf("Explain: %v", err)
	}
	if len(result.Accepted) != 1 {
		t.Fatalf("accepted len = %d, want 1", len(result.Accepted))
	}
	if result.Accepted[0].MemoryID != newer.ID {
		t.Fatalf("accepted memory = %s, want %s", result.Accepted[0].MemoryID, newer.ID)
	}
	if !result.Accepted[0].ConflictWarn {
		t.Fatal("expected conflict warning on accepted latest version")
	}

	foundOlderSuppressed := false
	for _, candidate := range result.Filtered {
		if candidate.MemoryID == older.ID {
			foundOlderSuppressed = true
			if len(candidate.FilteredReasons) == 0 || candidate.FilteredReasons[0] != "superseded_by_newer_version" {
				t.Fatalf("older filtered reasons = %v, want superseded_by_newer_version", candidate.FilteredReasons)
			}
		}
	}
	if !foundOlderSuppressed {
		t.Fatal("older version was not present in filtered candidates")
	}
}

func TestGroundReturnsStoredSourceRefs(t *testing.T) {
	eng := testutil.NewTestEngine(t)
	ctx := context.Background()

	remembered, err := eng.Remember(ctx, coreengine.RememberRequest{
		Content:    "user prefers concise answers",
		Summary:    "user preference: concise answers",
		MemoryType: "preference",
		SourceRefs: []model.SourceRef{
			{Kind: "chat", Ref: "chat:123", Title: "Preference stated"},
			{Kind: "file", Ref: "file:/notes/preferences.md#L1-L2", Snippet: "concise answers"},
		},
	})
	if err != nil {
		t.Fatalf("Remember: %v", err)
	}

	grounded, err := eng.Ground(ctx, remembered.ID)
	if err != nil {
		t.Fatalf("Ground: %v", err)
	}
	if grounded == nil {
		t.Fatal("expected grounded result")
	}
	if len(grounded.SourceRefs) != 2 {
		t.Fatalf("source refs len = %d, want 2", len(grounded.SourceRefs))
	}
	if grounded.SourceRefs[0].Ref != "chat:123" {
		t.Fatalf("first source ref = %q, want chat:123", grounded.SourceRefs[0].Ref)
	}
	if grounded.SourceRefs[1].Kind != "file" {
		t.Fatalf("second source kind = %q, want file", grounded.SourceRefs[1].Kind)
	}
}

func TestDecayStageResidualOutputs(t *testing.T) {
	content := "Alpha beta gamma delta epsilon zeta eta theta iota kappa lambda mu"
	cases := []struct {
		name     string
		clarity  float64
		wantForm string
	}{
		{name: "full", clarity: 0.95, wantForm: "full"},
		{name: "summary", clarity: 0.6, wantForm: "summary"},
		{name: "keywords", clarity: 0.3, wantForm: "keywords"},
		{name: "fragment", clarity: 0.1, wantForm: "fragment"},
		{name: "outline", clarity: 0.01, wantForm: "outline"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stage := decay.ResidualFormFromClarity(tc.clarity, decay.DefaultParams())
			if got := decay.ResidualFormName(stage); got != tc.wantForm {
				t.Fatalf("ResidualFormName = %q, want %q", got, tc.wantForm)
			}

			residual := transform.ToResidual(content, stage)
			if strings.TrimSpace(residual) == "" {
				t.Fatal("residual content should not be empty")
			}
			if tc.wantForm == "full" && residual != content {
				t.Fatalf("full residual = %q, want original content", residual)
			}
		})
	}
}

func TestRememberSyncsExtractedEntitiesIntoKG(t *testing.T) {
	eng, graph := testutil.NewTestEngineWithKG(t)
	ctx := context.Background()

	remembered, err := eng.Remember(ctx, coreengine.RememberRequest{
		Content:    "Project docs are published at https://docs.example.com under pkg/core/engine/engine.go",
		Summary:    "project docs publishing",
		MemoryType: "project",
		SourceRefs: []model.SourceRef{{Kind: "file", Ref: "pkg/core/engine/engine.go"}},
	})
	if err != nil {
		t.Fatalf("Remember: %v", err)
	}

	links, err := graph.ListMemoryEntityLinks(remembered.ID)
	if err != nil {
		t.Fatalf("ListMemoryEntityLinks: %v", err)
	}
	if len(links) < 2 {
		t.Fatalf("links len = %d, want at least 2", len(links))
	}

	results, err := graph.FindEntities("docs.example.com", 10)
	if err != nil {
		t.Fatalf("FindEntities url: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected url entity in KG")
	}

	foundPath := false
	for _, link := range links {
		if strings.Contains(link.Mention, "pkg/core/engine/engine.go") {
			foundPath = true
			break
		}
	}
	if !foundPath {
		t.Fatalf("expected path mention in links, got %+v", links)
	}
}

func TestDecayAllRebuildsMemoryEntityLinks(t *testing.T) {
	eng, graph := testutil.NewTestEngineWithKG(t)
	ctx := context.Background()

	remembered, err := eng.Remember(ctx, coreengine.RememberRequest{
		Content:    "API contract lives in skill/echo-fade-memory/SKILL.md and should remain traceable",
		Summary:    "api contract traceability",
		MemoryType: "project",
		SourceRefs: []model.SourceRef{{Kind: "file", Ref: "skill/echo-fade-memory/SKILL.md"}},
	})
	if err != nil {
		t.Fatalf("Remember: %v", err)
	}

	if err := graph.ReplaceMemoryEntityLinks(remembered.ID, nil); err != nil {
		t.Fatalf("ReplaceMemoryEntityLinks clear: %v", err)
	}

	cleared, err := graph.ListMemoryEntityLinks(remembered.ID)
	if err != nil {
		t.Fatalf("ListMemoryEntityLinks cleared: %v", err)
	}
	if len(cleared) != 0 {
		t.Fatalf("cleared links len = %d, want 0", len(cleared))
	}

	if err := eng.DecayAll(ctx); err != nil {
		t.Fatalf("DecayAll: %v", err)
	}

	restored, err := graph.ListMemoryEntityLinks(remembered.ID)
	if err != nil {
		t.Fatalf("ListMemoryEntityLinks restored: %v", err)
	}
	if len(restored) == 0 {
		t.Fatal("expected links to be rebuilt during decay")
	}
}

func TestDecayAllUpdatesStoredMemoryResidualState(t *testing.T) {
	eng, memStore, _, _ := testutil.NewTestEngineWithStores(t)
	ctx := context.Background()

	remembered, err := eng.Remember(ctx, coreengine.RememberRequest{
		Content:    "Alpha beta gamma delta epsilon zeta eta theta iota kappa lambda mu",
		Summary:    "greek letters note",
		MemoryType: "project",
		Importance: 0.0,
	})
	if err != nil {
		t.Fatalf("Remember: %v", err)
	}

	stored, err := memStore.Get(remembered.ID)
	if err != nil {
		t.Fatalf("Get stored: %v", err)
	}
	if stored == nil {
		t.Fatal("stored memory is nil")
	}
	stored.CreatedAt = stored.CreatedAt.AddDate(-1, 0, 0)
	stored.LastAccessedAt = stored.LastAccessedAt.AddDate(-1, 0, 0)
	stored.AccessCount = 0
	stored.Importance = 0
	if err := memStore.Save(stored); err != nil {
		t.Fatalf("Save aged memory: %v", err)
	}

	if err := eng.DecayAll(ctx); err != nil {
		t.Fatalf("DecayAll: %v", err)
	}

	decayed, err := memStore.Get(remembered.ID)
	if err != nil {
		t.Fatalf("Get decayed: %v", err)
	}
	if decayed == nil {
		t.Fatal("decayed memory is nil")
	}
	if decayed.Clarity >= 0.2 {
		t.Fatalf("clarity = %f, want below 0.2 for aged memory", decayed.Clarity)
	}
	if decayed.ResidualForm == "" || decayed.ResidualForm == "full" {
		t.Fatalf("residual form = %q, want degraded stage", decayed.ResidualForm)
	}
	if strings.TrimSpace(decayed.ResidualContent) == "" {
		t.Fatal("residual content should not be empty")
	}
	if decayed.ResidualContent == decayed.Content {
		t.Fatalf("residual content = %q, want degraded content", decayed.ResidualContent)
	}
	if decayed.LifecycleState != model.LifecycleArchived && decayed.LifecycleState != model.LifecycleForgotten {
		t.Fatalf("lifecycle state = %q, want archived or forgotten", decayed.LifecycleState)
	}
}

func TestRecallIncludesKGEvidenceForEntityOnlyMatch(t *testing.T) {
	eng, _ := testutil.NewTestEngineWithKG(t)
	ctx := context.Background()

	target, err := eng.Remember(ctx, coreengine.RememberRequest{
		Content:    "deployment checklist for service rollout",
		Summary:    "deployment checklist",
		MemoryType: "project",
		SourceRefs: []model.SourceRef{{Kind: "file", Ref: "docs/runbook.md"}},
	})
	if err != nil {
		t.Fatalf("Remember target: %v", err)
	}
	_, err = eng.Remember(ctx, coreengine.RememberRequest{
		Content:    "team lunch preference record",
		Summary:    "team lunch preference",
		MemoryType: "preference",
		SourceRefs: []model.SourceRef{{Kind: "chat", Ref: "chat:321"}},
	})
	if err != nil {
		t.Fatalf("Remember distractor: %v", err)
	}

	results, err := eng.Recall(ctx, "docs/runbook.md", 5, 0)
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected recall results")
	}

	found := false
	for _, result := range results {
		if result.MemoryID != target.ID {
			continue
		}
		found = true
		hasKGEvidence := false
		for _, evidence := range result.Evidence {
			if evidence.Backend == "kg" {
				hasKGEvidence = true
				break
			}
		}
		if !hasKGEvidence {
			t.Fatalf("expected kg evidence for target result: %+v", result.Evidence)
		}
		hasEntityMatch := false
		for _, reason := range result.WhyRecalled {
			if reason == "entity_match" {
				hasEntityMatch = true
				break
			}
		}
		if !hasEntityMatch {
			t.Fatalf("expected entity_match in why_recalled: %v", result.WhyRecalled)
		}
	}
	if !found {
		t.Fatalf("target memory %s not recalled", target.ID)
	}

	explain, err := eng.Explain(ctx, "docs/runbook.md", 5, 0)
	if err != nil {
		t.Fatalf("Explain: %v", err)
	}
	foundExplain := false
	for _, acc := range explain.Accepted {
		if acc.MemoryID != target.ID {
			continue
		}
		foundExplain = true
		hasKG := false
		for _, ev := range acc.Evidence {
			if ev.Backend == "kg" {
				hasKG = true
				break
			}
		}
		if !hasKG {
			t.Fatalf("Explain: expected kg evidence on accepted target: %+v", acc.Evidence)
		}
	}
	if !foundExplain {
		t.Fatalf("Explain: target %s not in accepted", target.ID)
	}
}

func TestStoreImageDeduplicatesAndRecallsByOCRAndLink(t *testing.T) {
	eng, _, _ := testutil.NewTestEngineWithImage(t)
	ctx := context.Background()

	remembered, err := eng.Remember(ctx, coreengine.RememberRequest{
		Content:    "project decision uses blue rollout checklist",
		Summary:    "blue rollout decision",
		MemoryType: "project",
	})
	if err != nil {
		t.Fatalf("Remember: %v", err)
	}

	imagePath := filepath.Join(t.TempDir(), "cat-checklist.png")
	if err := os.WriteFile(imagePath, []byte("fake-image-bytes"), 0644); err != nil {
		t.Fatalf("write image: %v", err)
	}
	if err := os.WriteFile(imagePath+".ocr.txt", []byte("Deployment Checklist"), 0644); err != nil {
		t.Fatalf("write ocr sidecar: %v", err)
	}

	first, duplicate, err := eng.StoreImage(ctx, coreengine.StoreImageRequest{
		FilePath:        imagePath,
		SourceSession:   "session:image",
		LinkedMemoryIDs: []string{remembered.ID},
	})
	if err != nil {
		t.Fatalf("StoreImage first: %v", err)
	}
	if duplicate {
		t.Fatal("first image should not be duplicate")
	}
	second, duplicate, err := eng.StoreImage(ctx, coreengine.StoreImageRequest{
		FilePath: imagePath,
	})
	if err != nil {
		t.Fatalf("StoreImage second: %v", err)
	}
	if !duplicate {
		t.Fatal("second image should be duplicate")
	}
	if first.ID != second.ID {
		t.Fatalf("duplicate image id = %s, want %s", second.ID, first.ID)
	}

	results, err := eng.RecallImages(ctx, "deployment checklist", 5)
	if err != nil {
		t.Fatalf("RecallImages OCR: %v", err)
	}
	if len(results) == 0 || results[0].ImageID != first.ID {
		t.Fatalf("expected OCR recall to return stored image, got %+v", results)
	}

	linked, err := eng.RecallImages(ctx, "blue rollout decision", 5)
	if err != nil {
		t.Fatalf("RecallImages linked: %v", err)
	}
	foundLinked := false
	for _, item := range linked {
		if item.ImageID == first.ID && len(item.LinkedMemoryIDs) > 0 {
			foundLinked = true
			break
		}
	}
	if !foundLinked {
		t.Fatalf("expected linked recall to surface image, got %+v", linked)
	}
}
