package cache

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/redis/go-redis/v9"
)

// SemanticCache uses Redis Stack FT.SEARCH with vector similarity.
// Requires Redis Stack (not vanilla Redis).
type SemanticCache struct {
	client         redis.UniversalClient
	indexName      string
	prefix         string
	threshold      float64 // cosine distance threshold (default 0.1)
	embeddingModel string
	embedFn        func(ctx context.Context, text string) ([]float32, error)
}

// SemanticCacheConfig holds configuration for semantic cache.
type SemanticCacheConfig struct {
	Client         redis.UniversalClient
	IndexName      string
	Prefix         string
	Threshold      float64
	EmbeddingModel string
	EmbedFn        func(ctx context.Context, text string) ([]float32, error)
}

// NewSemanticCache creates a semantic cache.
func NewSemanticCache(cfg SemanticCacheConfig) *SemanticCache {
	if cfg.IndexName == "" {
		cfg.IndexName = "idx:semantic_cache"
	}
	if cfg.Prefix == "" {
		cfg.Prefix = "cache:semantic:"
	}
	if cfg.Threshold == 0 {
		cfg.Threshold = 0.1
	}
	return &SemanticCache{
		client:         cfg.Client,
		indexName:      cfg.IndexName,
		prefix:         cfg.Prefix,
		threshold:      cfg.Threshold,
		embeddingModel: cfg.EmbeddingModel,
		embedFn:        cfg.EmbedFn,
	}
}

func (s *SemanticCache) Get(ctx context.Context, key string) ([]byte, error) {
	if s.embedFn == nil {
		return nil, fmt.Errorf("semantic cache: embed function not configured")
	}

	vec, err := s.embedFn(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("semantic cache embed: %w", err)
	}

	vecBytes := float32ToBytes(vec)

	// FT.SEARCH idx:semantic_cache "*=>[KNN 1 @embedding $vec AS score]" PARAMS 2 vec <bytes> DIALECT 2
	result, err := s.client.Do(ctx, "FT.SEARCH", s.indexName,
		"*=>[KNN 1 @embedding $vec AS score]",
		"PARAMS", 2, "vec", vecBytes,
		"SORTBY", "score",
		"LIMIT", 0, 1,
		"DIALECT", 2,
	).Result()
	if err != nil {
		return nil, fmt.Errorf("semantic cache search: %w", err)
	}

	// Parse FT.SEARCH result
	results, ok := result.([]any)
	if !ok || len(results) < 2 {
		return nil, nil // no results
	}

	// results[0] = total count, results[1] = key, results[2] = fields
	if len(results) < 3 {
		return nil, nil
	}

	fields, ok := results[2].([]any)
	if !ok {
		return nil, nil
	}

	var score float64
	var response []byte
	for i := 0; i < len(fields)-1; i += 2 {
		fieldName, _ := fields[i].(string)
		switch fieldName {
		case "score":
			if s, ok := fields[i+1].(string); ok {
				_, _ = fmt.Sscanf(s, "%f", &score)
			}
		case "response":
			if s, ok := fields[i+1].(string); ok {
				response = []byte(s)
			}
		}
	}

	if score > s.threshold {
		return nil, nil // not similar enough
	}

	return response, nil
}

func (s *SemanticCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if s.embedFn == nil {
		return fmt.Errorf("semantic cache: embed function not configured")
	}

	vec, err := s.embedFn(ctx, key)
	if err != nil {
		return fmt.Errorf("semantic cache embed: %w", err)
	}

	vecBytes := float32ToBytes(vec)
	hashKey := s.prefix + key

	pipe := s.client.Pipeline()
	pipe.HSet(ctx, hashKey, map[string]any{
		"embedding": vecBytes,
		"response":  string(value),
		"query":     key,
	})
	if ttl > 0 {
		pipe.Expire(ctx, hashKey, ttl)
	}
	_, err = pipe.Exec(ctx)
	return err
}

func (s *SemanticCache) Delete(ctx context.Context, key string) error {
	return s.client.Del(ctx, s.prefix+key).Err()
}

func (s *SemanticCache) MGet(ctx context.Context, keys ...string) ([][]byte, error) {
	result := make([][]byte, len(keys))
	for i, key := range keys {
		val, err := s.Get(ctx, key)
		if err != nil {
			return nil, err
		}
		result[i] = val
	}
	return result, nil
}

// float32ToBytes converts a float32 slice to bytes (little-endian).
func float32ToBytes(vec []float32) []byte {
	buf := make([]byte, len(vec)*4)
	for i, v := range vec {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	return buf
}

// Ensure json is used (for potential future use in response serialization)
var _ = json.Marshal
