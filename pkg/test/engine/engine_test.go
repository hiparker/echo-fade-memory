package engine_test

import (
	"context"
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
