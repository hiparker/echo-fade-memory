package engine_test

import (
	"context"
	"testing"

	coreengine "github.com/echo-fade-memory/echo-fade-memory/pkg/core/engine"
	"github.com/echo-fade-memory/echo-fade-memory/pkg/test/testutil"
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
