package testutil

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hiparker/echo-fade-memory/pkg/config"
	"github.com/hiparker/echo-fade-memory/pkg/core/engine"
	"github.com/hiparker/echo-fade-memory/pkg/port/embedding"
	"github.com/hiparker/echo-fade-memory/pkg/port/store"
	"github.com/hiparker/echo-fade-memory/pkg/port/store/sqlite"
	"github.com/hiparker/echo-fade-memory/pkg/port/vector/local"
)

type fakeEmbedder struct{}

func (fakeEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	text = strings.ToLower(text)
	vec := []float32{0.1, 0.1, 0.1}
	switch {
	case strings.Contains(text, "project"):
		vec[0] = 1
	case strings.Contains(text, "config"):
		vec[1] = 1
	default:
		vec[2] = 1
	}
	return vec, nil
}

func NewFakeEmbedder() embedding.Provider {
	return fakeEmbedder{}
}

func NewTestEngine(t *testing.T) *engine.Engine {
	t.Helper()

	tmp := t.TempDir()
	cfg := config.Default()
	cfg.DataPath = tmp
	cfg.Decay.CacheTTLMin = 0
	cfg.VectorStore.Type = "local"
	cfg.VectorStore.Path = filepath.Join(tmp, "vectors.json")

	mem, err := sqlite.New(filepath.Join(tmp, "memories.db"))
	if err != nil {
		t.Fatalf("sqlite.New: %v", err)
	}

	vectorStore, err := local.New(cfg.VectorStore.Path)
	if err != nil {
		t.Fatalf("local.New: %v", err)
	}

	bleveStore, err := store.OpenOrCreateBleve(filepath.Join(tmp, "bleve"))
	if err != nil {
		t.Fatalf("OpenOrCreateBleve: %v", err)
	}

	eng := engine.NewWithDeps(cfg, mem, vectorStore, bleveStore, NewFakeEmbedder())
	t.Cleanup(func() {
		_ = eng.Close()
	})
	return eng
}
