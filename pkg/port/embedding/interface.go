package embedding

import "context"

// Provider is the interface for text embedding.
type Provider interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}
