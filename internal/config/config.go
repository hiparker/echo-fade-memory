package config

import (
	"os"
	"path/filepath"
)

// Config holds application configuration.
type Config struct {
	DataPath   string
	OllamaURL  string
	EmbedModel string
	EmbedDim   int
	Port       int
}

// Default returns default configuration.
func Default() *Config {
	dataPath := os.Getenv("DATA_PATH")
	if dataPath == "" {
		dataPath = "./data"
	}
	ollamaURL := os.Getenv("OLLAMA_URL")
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434"
	}
	return &Config{
		DataPath:   dataPath,
		OllamaURL:  ollamaURL,
		EmbedModel: "nomic-embed-text",
		EmbedDim:   768,
		Port:       8080,
	}
}

// VectorPath returns path to vector store file (simple JSON for Phase 1).
func (c *Config) VectorPath() string {
	return filepath.Join(c.DataPath, "vectors.json")
}

// BlevePath returns path to Bleve index.
func (c *Config) BlevePath() string {
	return filepath.Join(c.DataPath, "bleve")
}

// SQLitePath returns path to SQLite DB.
func (c *Config) SQLitePath() string {
	return filepath.Join(c.DataPath, "memories.db")
}
