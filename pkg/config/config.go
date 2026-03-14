package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
)

// Config holds application configuration.
type Config struct {
	DataPath    string            `json:"data_path"`
	Port        int               `json:"port"`
	Ollama      OllamaConfig      `json:"ollama"`
	Decay       DecayConfig       `json:"decay"`
	VectorStore VectorStoreConfig `json:"vector_store"`
	Storage     StorageConfig     `json:"storage"`
}

// OllamaConfig holds Ollama embedding settings.
type OllamaConfig struct {
	URL        string `json:"url"`
	Model      string `json:"model"`
	Dimensions int    `json:"dimensions"`
}

// DecayConfig holds decay algorithm parameters.
type DecayConfig struct {
	Tau         float64 `json:"tau"`
	Alpha       float64 `json:"alpha"`
	Epsilon     float64 `json:"epsilon"`
	CacheTTLMin float64 `json:"cache_ttl_min"`
	Lambda, AccessBoost, EmotionalProtect float64
	HorizonDays float64
	DecayMode   string
	ClarityFull, ClaritySummary, ClarityKeywords, ClarityFragment float64
	StageSummary, StageKeywords, StageFragment, StageOutline     int
}

// VectorStoreConfig holds vector store backend settings.
type VectorStoreConfig struct {
	Type       string `json:"type"`
	Path       string `json:"path"`
	MilvusHost string `json:"milvus_host"`
	MilvusPort int    `json:"milvus_port"`
	MilvusDB   string `json:"milvus_db"`
}

// StorageConfig holds metadata storage backend settings.
type StorageConfig struct {
	Type        string `json:"type"`
	Path        string `json:"path"`
	PostgresDSN string `json:"postgres_dsn"`
}

// Default returns default configuration.
func Default() *Config {
	return &Config{
		DataPath: "./data",
		Port:     8080,
		Ollama: OllamaConfig{
			URL:        "http://localhost:11434",
			Model:      "nomic-embed-text",
			Dimensions: 768,
		},
		Decay: DecayConfig{
			Tau:         90,
			Alpha:       1.5,
			Epsilon:     0.1,
			CacheTTLMin: 5,
		},
		VectorStore: VectorStoreConfig{Type: "local", Path: ""},
		Storage:     StorageConfig{Type: "sqlite", Path: ""},
	}
}

// Load reads config from file and env. Env overrides file.
func Load(configPath string) (*Config, error) {
	cfg := Default()
	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err == nil {
			if err := json.Unmarshal(data, cfg); err != nil {
				return nil, err
			}
		}
	}
	applyEnvOverrides(cfg)
	resolvePaths(cfg)
	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("DATA_PATH"); v != "" {
		cfg.DataPath = v
	}
	if v := os.Getenv("PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Port = n
		}
	}
	if v := os.Getenv("OLLAMA_URL"); v != "" {
		cfg.Ollama.URL = v
	}
	if v := os.Getenv("OLLAMA_MODEL"); v != "" {
		cfg.Ollama.Model = v
	}
	if v := os.Getenv("OLLAMA_DIMENSIONS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Ollama.Dimensions = n
		}
	}
	if v := os.Getenv("DECAY_TAU"); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.Decay.Tau = n
		}
	}
	if v := os.Getenv("DECAY_ALPHA"); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.Decay.Alpha = n
		}
	}
	if v := os.Getenv("DECAY_EPSILON"); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.Decay.Epsilon = n
		}
	}
	if v := os.Getenv("DECAY_CACHE_TTL_MIN"); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil && n >= 0 {
			cfg.Decay.CacheTTLMin = n
		}
	}
	if v := os.Getenv("DECAY_LAMBDA"); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.Decay.Lambda = n
		}
	}
	if v := os.Getenv("DECAY_ACCESS_BOOST"); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.Decay.AccessBoost = n
		}
	}
	if v := os.Getenv("DECAY_HORIZON_DAYS"); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.Decay.HorizonDays = n
		}
	}
	if v := os.Getenv("VECTOR_STORE_TYPE"); v != "" {
		cfg.VectorStore.Type = v
	}
	if v := os.Getenv("VECTOR_STORE_PATH"); v != "" {
		cfg.VectorStore.Path = v
	}
	if v := os.Getenv("STORAGE_TYPE"); v != "" {
		cfg.Storage.Type = v
	}
	if v := os.Getenv("STORAGE_PATH"); v != "" {
		cfg.Storage.Path = v
	}
	if v := os.Getenv("POSTGRES_DSN"); v != "" {
		cfg.Storage.PostgresDSN = v
	}
}

func resolvePaths(cfg *Config) {
	if cfg.VectorStore.Path == "" {
		if cfg.VectorStore.Type == "lancedb" {
			cfg.VectorStore.Path = filepath.Join(cfg.DataPath, "lancedb")
		} else {
			cfg.VectorStore.Path = filepath.Join(cfg.DataPath, "vectors.json")
		}
	}
	if cfg.Storage.Path == "" {
		cfg.Storage.Path = filepath.Join(cfg.DataPath, "memories.db")
	}
}

// VectorPath returns path for local vector store.
func (c *Config) VectorPath() string {
	if c.VectorStore.Path != "" {
		return c.VectorStore.Path
	}
	return filepath.Join(c.DataPath, "vectors.json")
}

// BlevePath returns path to Bleve index.
func (c *Config) BlevePath() string {
	return filepath.Join(c.DataPath, "bleve")
}

// SQLitePath returns path to SQLite DB.
func (c *Config) SQLitePath() string {
	if c.Storage.Path != "" {
		return c.Storage.Path
	}
	return filepath.Join(c.DataPath, "memories.db")
}
