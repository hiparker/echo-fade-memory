package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hiparker/echo-fade-memory/pkg/config"
	"github.com/hiparker/echo-fade-memory/pkg/port/storefactory"
)

func TestLoadMissingConfigFallsBackToDefaults(t *testing.T) {
	home := t.TempDir()
	workspace := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspace, 0755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(workspace); err != nil {
		t.Fatalf("chdir workspace: %v", err)
	}
	defer func() { _ = os.Chdir(prev) }()

	t.Setenv("ECHO_FADE_MEMORY_HOME", home)
	t.Setenv("ECHO_FADE_MEMORY_WORKSPACE", "")

	cfg, err := config.Load(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("Load returned error for missing file: %v", err)
	}
	if cfg == nil {
		t.Fatal("Load returned nil config")
	}
	if cfg.Embedding.Type != "ollama" {
		t.Fatalf("embedding type = %q, want ollama", cfg.Embedding.Type)
	}
	if cfg.VectorStore.Type != "local" {
		t.Fatalf("vector_store.type = %q, want local", cfg.VectorStore.Type)
	}
	wantDataPath := filepath.Join(home, "workspaces", config.WorkspaceID(), "data")
	if cfg.DataPath != wantDataPath {
		t.Fatalf("DataPath = %q, want %q", cfg.DataPath, wantDataPath)
	}
	if got, want := cfg.VectorPath(), filepath.Join(cfg.DataPath, "vectors.json"); got != want {
		t.Fatalf("VectorPath = %q, want %q", got, want)
	}
	if got, want := cfg.SQLitePath(), filepath.Join(cfg.DataPath, "memories.db"); got != want {
		t.Fatalf("SQLitePath = %q, want %q", got, want)
	}
}

func TestLoadInvalidConfigFailsFast(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte("{invalid json"), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := config.Load(path); err == nil {
		t.Fatal("Load succeeded for invalid config, want error")
	}
}

func TestLoadRejectsInvalidVectorStoreType(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	data := []byte(`{"vector_store":{"type":"weaviate"}}`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := config.Load(path); err == nil {
		t.Fatal("Load succeeded for invalid vector store type, want error")
	}
}

func TestLoadUsesLanceDBDefaultPath(t *testing.T) {
	home := t.TempDir()
	workspace := filepath.Join(t.TempDir(), "runtime-workspace")
	if err := os.MkdirAll(workspace, 0755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(workspace); err != nil {
		t.Fatalf("chdir workspace: %v", err)
	}
	defer func() { _ = os.Chdir(prev) }()

	t.Setenv("ECHO_FADE_MEMORY_HOME", home)
	t.Setenv("ECHO_FADE_MEMORY_WORKSPACE", "")

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	data := []byte(`{"vector_store":{"type":"lancedb"}}`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if got, want := cfg.VectorPath(), filepath.Join(cfg.DataPath, "lancedb"); got != want {
		t.Fatalf("VectorPath = %q, want %q", got, want)
	}
}

func TestLoadRespectsWorkspaceOverride(t *testing.T) {
	home := t.TempDir()
	t.Setenv("ECHO_FADE_MEMORY_HOME", home)
	t.Setenv("ECHO_FADE_MEMORY_WORKSPACE", "demo-project")

	cfg, err := config.Load(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !strings.Contains(cfg.DataPath, filepath.Join("workspaces", "demo-project", "data")) {
		t.Fatalf("DataPath = %q, want workspace override path", cfg.DataPath)
	}
}

func TestFactoryRejectsUnsupportedStorageType(t *testing.T) {
	cfg := config.Default()
	cfg.Storage.Type = "mongodb"

	if _, err := storefactory.NewMemoryStore(cfg); err == nil {
		t.Fatal("NewMemoryStore succeeded for unsupported storage type, want error")
	}
}

func TestFactoryRejectsMissingRemoteStoreConfig(t *testing.T) {
	cfg := config.Default()
	cfg.Storage.Type = "mysql"
	cfg.Storage.MySQLDSN = ""
	if _, err := storefactory.NewMemoryStore(cfg); err == nil {
		t.Fatal("NewMemoryStore succeeded for mysql without dsn, want error")
	}

	cfg = config.Default()
	cfg.VectorStore.Type = "milvus"
	cfg.VectorStore.MilvusHost = ""
	if _, err := storefactory.NewVectorStore(cfg); err == nil {
		t.Fatal("NewVectorStore succeeded for milvus without host, want error")
	}
}
