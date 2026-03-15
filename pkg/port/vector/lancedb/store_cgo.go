//go:build cgo && lancedb

package lancedb

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"
	"github.com/hiparker/echo-fade-memory/pkg/config"
	"github.com/lancedb/lancedb-go/pkg/contracts"
	sdklancedb "github.com/lancedb/lancedb-go/pkg/lancedb"
)

const (
	defaultTableName = "echo_fade_memory_vectors"
	idField          = "id"
	vectorField      = "vector"
)

// Store implements the vector store interface with LanceDB.
type Store struct {
	path      string
	tableName string
	dim       int

	initOnce sync.Once
	initErr  error

	mu    sync.Mutex
	conn  contracts.IConnection
	table contracts.ITable
}

// New creates a LanceDB-backed vector store.
func New(cfg *config.Config) (*Store, error) {
	dim := cfg.Embedding.Dimensions
	if dim <= 0 {
		dim = 768
	}
	path := cfg.VectorStore.Path
	if path == "" {
		path = filepath.Join(cfg.DataPath, "lancedb")
	}
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, fmt.Errorf("create lancedb path: %w", err)
	}
	return &Store{
		path:      path,
		tableName: defaultTableName,
		dim:       dim,
	}, nil
}

func (s *Store) ensureTable(ctx context.Context) error {
	s.initOnce.Do(func() {
		conn, err := sdklancedb.Connect(ctx, s.path, nil)
		if err != nil {
			s.initErr = fmt.Errorf("connect lancedb: %w", err)
			return
		}

		table, err := conn.OpenTable(ctx, s.tableName)
		if err != nil {
			schema, schemaErr := sdklancedb.NewSchemaBuilder().
				AddStringField(idField, false).
				AddVectorField(vectorField, s.dim, contracts.VectorDataTypeFloat32, false).
				Build()
			if schemaErr != nil {
				_ = conn.Close()
				s.initErr = fmt.Errorf("build lancedb schema: %w", schemaErr)
				return
			}
			table, err = conn.CreateTable(ctx, s.tableName, schema)
			if err != nil {
				_ = conn.Close()
				s.initErr = fmt.Errorf("create lancedb table: %w", err)
				return
			}
		}

		s.conn = conn
		s.table = table
	})
	return s.initErr
}

func (s *Store) Add(id string, vec []float32) error {
	if len(vec) != s.dim {
		return fmt.Errorf("lancedb vector dimension mismatch: got %d want %d", len(vec), s.dim)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.ensureTable(ctx); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.table.Delete(ctx, idFilter(id)); err != nil {
		return fmt.Errorf("delete existing lancedb row: %w", err)
	}

	record, err := newRecord(id, vec)
	if err != nil {
		return err
	}
	defer record.Release()

	if err := s.table.Add(ctx, record, nil); err != nil {
		return fmt.Errorf("add lancedb row: %w", err)
	}
	return nil
}

func (s *Store) Search(ctx context.Context, query []float32, k int) ([]string, []float32, error) {
	if k <= 0 {
		return nil, nil, nil
	}
	if len(query) != s.dim {
		return nil, nil, fmt.Errorf("lancedb query dimension mismatch: got %d want %d", len(query), s.dim)
	}
	if err := s.ensureTable(ctx); err != nil {
		return nil, nil, err
	}

	rows, err := s.table.VectorSearch(ctx, vectorField, query, k)
	if err != nil {
		return nil, nil, fmt.Errorf("lancedb search: %w", err)
	}
	ids := make([]string, 0, len(rows))
	scores := make([]float32, 0, len(rows))
	for i, row := range rows {
		id, ok := row[idField].(string)
		if !ok || id == "" {
			continue
		}
		ids = append(ids, id)
		scores = append(scores, scoreFromRow(row, i))
	}
	return ids, scores, nil
}

func (s *Store) Remove(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.ensureTable(ctx); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.table.Delete(ctx, idFilter(id)); err != nil {
		return fmt.Errorf("delete lancedb row: %w", err)
	}
	return nil
}

// Close releases the LanceDB connection if it was opened.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var err error
	if s.table != nil {
		err = s.table.Close()
		s.table = nil
	}
	if s.conn != nil {
		closeErr := s.conn.Close()
		s.conn = nil
		if err == nil {
			err = closeErr
		}
	}
	return err
}

func newRecord(id string, vec []float32) (arrow.Record, error) {
	pool := memory.NewGoAllocator()

	idBuilder := array.NewStringBuilder(pool)
	defer idBuilder.Release()
	idBuilder.Append(id)
	idArray := idBuilder.NewArray()

	vecBuilder := array.NewFloat32Builder(pool)
	defer vecBuilder.Release()
	vecBuilder.AppendValues(vec, nil)
	vecValues := vecBuilder.NewArray()

	listType := arrow.FixedSizeListOf(int32(len(vec)), arrow.PrimitiveTypes.Float32)
	listData := array.NewData(
		listType,
		1,
		[]*memory.Buffer{nil},
		[]arrow.ArrayData{vecValues.Data()},
		0,
		0,
	)
	vecArray := array.NewFixedSizeListData(listData)

	schema := arrow.NewSchema([]arrow.Field{
		{Name: idField, Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: vectorField, Type: listType, Nullable: false},
	}, nil)
	record := array.NewRecord(schema, []arrow.Array{idArray, vecArray}, 1)

	vecArray.Release()
	vecValues.Release()

	return record, nil
}

func idFilter(id string) string {
	return fmt.Sprintf("%s = '%s'", idField, strings.ReplaceAll(id, "'", "''"))
}

func scoreFromRow(row map[string]interface{}, rank int) float32 {
	for _, key := range []string{"_distance", "distance", "_score", "score"} {
		if v, ok := numericValue(row[key]); ok {
			return distanceToScore(v)
		}
	}
	return 1 / float32(rank+1)
}

func numericValue(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case string:
		f, err := strconv.ParseFloat(n, 64)
		if err == nil {
			return f, true
		}
	}
	return 0, false
}

func distanceToScore(v float64) float32 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	if v < 0 {
		return float32(v)
	}
	return float32(1 / (1 + v))
}
