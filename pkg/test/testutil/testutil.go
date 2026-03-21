package testutil

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hiparker/echo-fade-memory/pkg/config"
	"github.com/hiparker/echo-fade-memory/pkg/core/engine"
	"github.com/hiparker/echo-fade-memory/pkg/port/embedding"
	"github.com/hiparker/echo-fade-memory/pkg/port/imageproc/basic"
	"github.com/hiparker/echo-fade-memory/pkg/port/imagestore"
	imgsqlite "github.com/hiparker/echo-fade-memory/pkg/port/imagestore/sqlite"
	"github.com/hiparker/echo-fade-memory/pkg/port/kgstore"
	kgsqlite "github.com/hiparker/echo-fade-memory/pkg/port/kgstore/sqlite"
	"github.com/hiparker/echo-fade-memory/pkg/port/memstore"
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
	eng, _, _ := NewTestEngineWithImage(t)
	return eng
}

func NewTestEngineWithKG(t *testing.T) (*engine.Engine, kgstore.Store) {
	t.Helper()
	eng, graph, _ := NewTestEngineWithImage(t)
	return eng, graph
}

func NewTestEngineWithImage(t *testing.T) (*engine.Engine, kgstore.Store, imagestore.Store) {
	t.Helper()
	eng, _, graphStore, imageStore := NewTestEngineWithStores(t)
	return eng, graphStore, imageStore
}

func NewTestEngineWithStores(t *testing.T) (*engine.Engine, memstore.MemoryStore, kgstore.Store, imagestore.Store) {
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

	graphStore, err := kgsqlite.New(filepath.Join(tmp, "kg.db"))
	if err != nil {
		t.Fatalf("kgsqlite.New: %v", err)
	}

	imageStore, err := imgsqlite.New(filepath.Join(tmp, "images.db"))
	if err != nil {
		t.Fatalf("imgsqlite.New: %v", err)
	}

	imageVectorStore, err := local.New(filepath.Join(tmp, "image-vectors.json"))
	if err != nil {
		t.Fatalf("local.New image: %v", err)
	}

	imageBleveStore, err := store.OpenOrCreateBleve(filepath.Join(tmp, "image-bleve"))
	if err != nil {
		t.Fatalf("OpenOrCreateBleve image: %v", err)
	}

	eng := engine.NewWithDepsFull(cfg, mem, vectorStore, bleveStore, NewFakeEmbedder(), graphStore, imageStore, imageVectorStore, imageBleveStore, basic.New())
	t.Cleanup(func() {
		_ = eng.Close()
	})
	return eng, mem, graphStore, imageStore
}
