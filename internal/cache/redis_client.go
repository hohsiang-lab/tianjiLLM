package cache

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/redis/go-redis/v9"
)

// RedisMode indicates the Redis deployment topology.
type RedisMode int

const (
	RedisModeStandalone RedisMode = iota
	RedisModeCluster
	RedisModeSentinel
)

// NewRedisClient creates a Redis client from environment variables.
// Supports Standalone, Cluster, and Sentinel modes.
// Env vars match Python LiteLLM's _redis.py configuration.
func NewRedisClient(ctx context.Context) (redis.UniversalClient, error) {
	mode := detectMode()

	switch mode {
	case RedisModeCluster:
		return newClusterClient(ctx)
	case RedisModeSentinel:
		return newSentinelClient(ctx)
	default:
		return newStandaloneClient(ctx)
	}
}

func detectMode() RedisMode {
	if os.Getenv("REDIS_CLUSTER_NODES") != "" {
		return RedisModeCluster
	}
	if os.Getenv("REDIS_SENTINEL_NODES") != "" {
		return RedisModeSentinel
	}
	return RedisModeStandalone
}

func newStandaloneClient(ctx context.Context) (redis.UniversalClient, error) {
	if redisURL := os.Getenv("REDIS_URL"); redisURL != "" {
		opts, err := redis.ParseURL(redisURL)
		if err != nil {
			return nil, fmt.Errorf("parse REDIS_URL: %w", err)
		}
		opts.PoolSize = poolSize()
		client := redis.NewClient(opts)
		if err := client.Ping(ctx).Err(); err != nil {
			return nil, fmt.Errorf("redis ping: %w", err)
		}
		return client, nil
	}

	host := envOr("REDIS_HOST", "localhost")
	port := envOr("REDIS_PORT", "6379")

	opts := &redis.Options{
		Addr:     host + ":" + port,
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       envInt("REDIS_DB", 0),
		PoolSize: poolSize(),
	}

	if username := os.Getenv("REDIS_USERNAME"); username != "" {
		opts.Username = username
	}

	if useSSL() {
		opts.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}

	client := redis.NewClient(opts)
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return client, nil
}

func newClusterClient(ctx context.Context) (redis.UniversalClient, error) {
	nodesJSON := os.Getenv("REDIS_CLUSTER_NODES")
	var nodes []string
	if err := json.Unmarshal([]byte(nodesJSON), &nodes); err != nil {
		return nil, fmt.Errorf("parse REDIS_CLUSTER_NODES: %w", err)
	}

	opts := &redis.ClusterOptions{
		Addrs:    nodes,
		Password: os.Getenv("REDIS_PASSWORD"),
		PoolSize: poolSize(),
	}

	if useSSL() {
		opts.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}

	client := redis.NewClusterClient(opts)
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis cluster ping: %w", err)
	}
	return client, nil
}

func newSentinelClient(ctx context.Context) (redis.UniversalClient, error) {
	nodesJSON := os.Getenv("REDIS_SENTINEL_NODES")
	var nodes []string
	if err := json.Unmarshal([]byte(nodesJSON), &nodes); err != nil {
		return nil, fmt.Errorf("parse REDIS_SENTINEL_NODES: %w", err)
	}

	serviceName := envOr("REDIS_SERVICE_NAME", "mymaster")

	opts := &redis.FailoverOptions{
		MasterName:    serviceName,
		SentinelAddrs: nodes,
		Password:      os.Getenv("REDIS_PASSWORD"),
		DB:            envInt("REDIS_DB", 0),
		PoolSize:      poolSize(),
	}

	if useSSL() {
		opts.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}

	client := redis.NewFailoverClient(opts)
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis sentinel ping: %w", err)
	}
	return client, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func poolSize() int {
	return envInt("REDIS_MAX_CONNECTIONS", 10)
}

func useSSL() bool {
	return strings.EqualFold(os.Getenv("REDIS_SSL"), "true")
}
