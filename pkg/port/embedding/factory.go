package embedding

import (
	"fmt"
	"os"
	"strings"

	"github.com/hiparker/echo-fade-memory/pkg/config"
	"github.com/hiparker/echo-fade-memory/pkg/port/embedding/gemini"
	"github.com/hiparker/echo-fade-memory/pkg/port/embedding/ollama"
	"github.com/hiparker/echo-fade-memory/pkg/port/embedding/openai"
)

// NewProvider creates an embedding provider from config.
// OpenAI/Gemini are only activated when type is explicitly set and API key is configured.
func NewProvider(cfg *config.Config) (Provider, error) {
	e := &cfg.Embedding
	model := e.Model
	dim := e.Dimensions
	url := e.URL

	switch strings.ToLower(e.Type) {
	case "ollama":
		return ollama.New(url, model, dim), nil
	case "openai":
		apiKey := e.APIKey
		if apiKey == "" {
			apiKey = os.Getenv("OPENAI_API_KEY")
		}
		if apiKey == "" {
			return nil, fmt.Errorf("embedding type=openai requires api_key or OPENAI_API_KEY")
		}
		return openai.New(apiKey, e.BaseURL, model, dim), nil
	case "gemini":
		apiKey := e.APIKey
		if apiKey == "" {
			apiKey = os.Getenv("GOOGLE_API_KEY")
		}
		if apiKey == "" {
			return nil, fmt.Errorf("embedding type=gemini requires api_key or GOOGLE_API_KEY")
		}
		return gemini.New(apiKey, e.BaseURL, model, dim)
	default:
		return ollama.New(url, model, dim), nil
	}
}

