package openai

import (
	"context"
	"fmt"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// Provider implements embedding.Provider with OpenAI official SDK.
type Provider struct {
	client openai.Client
	model  openai.EmbeddingModel
	dim    int
}

// New creates an OpenAI embedding provider using the official SDK.
func New(apiKey, baseURL, model string, dim int) *Provider {
	if model == "" {
		model = "text-embedding-3-small"
	}
	opts := []option.RequestOption{option.WithAPIKey(apiKey)}
	if baseURL != "" && baseURL != "https://api.openai.com/v1" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	client := openai.NewClient(opts...)
	return &Provider{
		client: client,
		model:  openai.EmbeddingModel(model),
		dim:    dim,
	}
}

func (p *Provider) Embed(ctx context.Context, text string) ([]float32, error) {
	params := openai.EmbeddingNewParams{
		Input: openai.EmbeddingNewParamsInputUnion{
			OfString: openai.String(text),
		},
		Model: p.model,
	}
	if p.dim > 0 {
		params.Dimensions = openai.Int(int64(p.dim))
	}
	resp, err := p.client.Embeddings.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("openai embed: %w", err)
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("openai embed: no data in response")
	}
	emb := resp.Data[0].Embedding
	vec := make([]float32, len(emb))
	for i, v := range emb {
		vec[i] = float32(v)
	}
	return vec, nil
}
