package milvus

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hiparker/echo-fade-memory/pkg/config"
	"github.com/hiparker/echo-fade-memory/pkg/port/vector"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

const (
	defaultCollection = "echo_fade_memory"
	idField           = "id"
	vectorField       = "vector"
)

// Store implements vector.Store with Milvus.
type Store struct {
	c          client.Client
	coll       string
	dim        int
	initOnce   sync.Once
	initErr    error
}

// New creates a Milvus vector store.
func New(cfg *config.Config) (vector.Store, error) {
	host := cfg.VectorStore.MilvusHost
	if host == "" {
		host = "localhost"
	}
	port := cfg.VectorStore.MilvusPort
	if port == 0 {
		port = 19530
	}
	addr := host + ":" + strconv.Itoa(port)
	dim := cfg.Embedding.Dimensions
	if dim <= 0 {
		dim = 768
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c, err := client.NewClient(ctx, client.Config{Address: addr})
	if err != nil {
		return nil, fmt.Errorf("milvus connect: %w", err)
	}

	if cfg.VectorStore.MilvusDB != "" {
		if err := c.UsingDatabase(ctx, cfg.VectorStore.MilvusDB); err != nil {
			c.Close()
			return nil, fmt.Errorf("milvus use db: %w", err)
		}
	}

	s := &Store{c: c, coll: defaultCollection, dim: dim}
	return s, nil
}

func (s *Store) ensureCollection(ctx context.Context) error {
	s.initOnce.Do(func() {
		has, err := s.c.HasCollection(ctx, s.coll)
		if err != nil {
			s.initErr = fmt.Errorf("milvus has collection: %w", err)
			return
		}
		if has {
			return
		}
		schema := entity.NewSchema().WithName(s.coll).WithDescription("echo-fade-memory vectors").
			WithField(entity.NewField().WithName(idField).WithDataType(entity.FieldTypeVarChar).WithMaxLength(64).WithIsPrimaryKey(true)).
			WithField(entity.NewField().WithName(vectorField).WithDataType(entity.FieldTypeFloatVector).WithDim(int64(s.dim)))
		if err := s.c.CreateCollection(ctx, schema, entity.DefaultShardNumber); err != nil {
			s.initErr = fmt.Errorf("milvus create collection: %w", err)
			return
		}
		idx, err := entity.NewIndexHNSW(entity.IP, 16, 200)
		if err != nil {
			s.initErr = fmt.Errorf("milvus new index: %w", err)
			return
		}
		if err := s.c.CreateIndex(ctx, s.coll, vectorField, idx, false); err != nil {
			s.initErr = fmt.Errorf("milvus create index: %w", err)
			return
		}
		if err := s.c.LoadCollection(ctx, s.coll, false); err != nil {
			s.initErr = fmt.Errorf("milvus load collection: %w", err)
			return
		}
	})
	return s.initErr
}

func normalize(vec []float32) []float32 {
	var sum float64
	for _, v := range vec {
		sum += float64(v) * float64(v)
	}
	if sum < 1e-12 {
		return vec
	}
	norm := float32(math.Sqrt(sum))
	out := make([]float32, len(vec))
	for i, v := range vec {
		out[i] = v / norm
	}
	return out
}

func (s *Store) Add(id string, vec []float32) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.ensureCollection(ctx); err != nil {
		return err
	}
	vec = normalize(vec)
	idCol := entity.NewColumnVarChar(idField, []string{id})
	vecCol := entity.NewColumnFloatVector(vectorField, s.dim, [][]float32{vec})
	_, err := s.c.Insert(ctx, s.coll, "", idCol, vecCol)
	if err != nil {
		return fmt.Errorf("milvus insert: %w", err)
	}
	flushCtx, flushCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer flushCancel()
	return s.c.Flush(flushCtx, s.coll, false)
}

func (s *Store) Search(ctx context.Context, query []float32, k int) ([]string, []float32, error) {
	if err := s.ensureCollection(ctx); err != nil {
		return nil, nil, err
	}
	query = normalize(query)
	sp, _ := entity.NewIndexHNSWSearchParam(200)
	results, err := s.c.Search(ctx, s.coll, []string{}, "", []string{idField}, []entity.Vector{entity.FloatVector(query)}, vectorField, entity.IP, k, sp)
	if err != nil {
		return nil, nil, fmt.Errorf("milvus search: %w", err)
	}
	if len(results) == 0 {
		return nil, nil, nil
	}
	r := results[0]
	ids := make([]string, 0, r.ResultCount)
	scores := make([]float32, 0, r.ResultCount)
	for _, f := range r.Fields {
		if col, ok := f.(*entity.ColumnVarChar); ok {
			for i := 0; i < r.ResultCount; i++ {
				v, err := col.ValueByIdx(i)
				if err != nil {
					continue
				}
				ids = append(ids, v)
			}
			break
		}
	}
	for i := 0; i < r.ResultCount; i++ {
		scores = append(scores, r.Scores[i])
	}
	return ids, scores, nil
}

func (s *Store) Close() error {
	return s.c.Close()
}

func (s *Store) Remove(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.ensureCollection(ctx); err != nil {
		return err
	}
	// Escape " and \ for Milvus expr string literal
	escaped := strings.ReplaceAll(id, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	expr := fmt.Sprintf("%s == \"%s\"", idField, escaped)
	return s.c.Delete(ctx, s.coll, "", expr)
}
