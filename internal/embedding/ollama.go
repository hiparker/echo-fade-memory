package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Client calls Ollama for embeddings.
type Client struct {
	BaseURL string
	Model   string
	Dim     int
	client  *http.Client
}

// NewOllamaClient creates an Ollama embedding client.
func NewOllamaClient(baseURL, model string, dim int) *Client {
	return &Client{
		BaseURL: baseURL,
		Model:   model,
		Dim:     dim,
		client:  &http.Client{},
	}
}

// EmbedRequest is the Ollama API request format.
type EmbedRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// EmbedResponse is the Ollama API response format.
type EmbedResponse struct {
	Embedding []float64 `json:"embedding"`
}

// Embed returns the embedding vector for the given text.
func (c *Client) Embed(ctx context.Context, text string) ([]float32, error) {
	body, _ := json.Marshal(EmbedRequest{Model: c.Model, Prompt: text})
	url := c.BaseURL
	if len(url) > 0 && url[len(url)-1] == '/' {
		url = url[:len(url)-1]
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url+"/api/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama embed: %s: %s", resp.Status, string(b))
	}

	var out EmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	// Convert float64 to float32
	vec := make([]float32, len(out.Embedding))
	for i, v := range out.Embedding {
		vec[i] = float32(v)
	}
	return vec, nil
}
