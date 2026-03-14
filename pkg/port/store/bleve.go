package store

import (
	"context"
	"fmt"

	"github.com/blevesearch/bleve/v2"
)

// BleveStore provides BM25 full-text search.
type BleveStore struct {
	index bleve.Index
}

// OpenOrCreateBleve opens existing or creates new Bleve index.
func OpenOrCreateBleve(path string) (*BleveStore, error) {
	idx, err := bleve.Open(path)
	if err == nil {
		return &BleveStore{index: idx}, nil
	}
	m := bleve.NewIndexMapping()
	idx, err = bleve.New(path, m)
	if err != nil {
		return nil, fmt.Errorf("bleve open/create: %w", err)
	}
	return &BleveStore{index: idx}, nil
}

func (b *BleveStore) Index(id, content string) error {
	return b.index.Index(id, map[string]interface{}{"content": content})
}

func (b *BleveStore) Delete(id string) error {
	return b.index.Delete(id)
}

func (b *BleveStore) Search(ctx context.Context, query string, k int) ([]string, []float64, error) {
	q := bleve.NewMatchQuery(query)
	req := bleve.NewSearchRequest(q)
	req.Size = k
	req.Fields = []string{"*"}

	res, err := b.index.Search(req)
	if err != nil {
		return nil, nil, err
	}

	ids := make([]string, 0, len(res.Hits))
	scores := make([]float64, 0, len(res.Hits))
	for _, h := range res.Hits {
		ids = append(ids, h.ID)
		scores = append(scores, h.Score)
	}
	return ids, scores, nil
}

func (b *BleveStore) Close() error {
	return b.index.Close()
}
