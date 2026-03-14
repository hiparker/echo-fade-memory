package gemini

import (
	"context"
	"fmt"

	"google.golang.org/genai"
)

// Provider implements embedding.Provider with Google Gemini official SDK.
type Provider struct {
	client *genai.Client
	model  string
	dim    int
}

// New creates a Gemini embedding provider using the official genai SDK.
func New(apiKey, baseURL, model string, dim int) (*Provider, error) {
	if model == "" {
		model = "text-embedding-004"
	}
	cfg := &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	}
	client, err := genai.NewClient(context.Background(), cfg)
	if err != nil {
		return nil, err
	}
	return &Provider{
		client: client,
		model:  model,
		dim:    dim,
	}, nil
}

func (p *Provider) Embed(ctx context.Context, text string) ([]float32, error) {
	contents := []*genai.Content{
		genai.NewContentFromText(text, genai.RoleUser),
	}
	var opts *genai.EmbedContentConfig
	if p.dim > 0 {
		d := int32(p.dim)
		opts = &genai.EmbedContentConfig{
			OutputDimensionality: &d,
		}
	}
	result, err := p.client.Models.EmbedContent(ctx, p.model, contents, opts)
	if err != nil {
		return nil, fmt.Errorf("gemini embed: %w", err)
	}
	if len(result.Embeddings) == 0 {
		return nil, fmt.Errorf("gemini embed: no embeddings in response")
	}
	return result.Embeddings[0].Values, nil
}
