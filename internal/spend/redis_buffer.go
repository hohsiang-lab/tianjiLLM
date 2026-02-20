package spend

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/redis/go-redis/v9"
)

const (
	spendQueueKey  = "tianji:spend_queue"
	flushInterval  = 10 * time.Second
	flushBatchSize = 100
)

// RedisBuffer buffers spend logs in Redis and batch-flushes to the DB.
type RedisBuffer struct {
	rdb      redis.UniversalClient
	database *db.Queries
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewRedisBuffer creates a buffer that queues spend logs in Redis.
func NewRedisBuffer(rdb redis.UniversalClient, database *db.Queries) *RedisBuffer {
	ctx, cancel := context.WithCancel(context.Background())
	b := &RedisBuffer{
		rdb:      rdb,
		database: database,
		ctx:      ctx,
		cancel:   cancel,
	}
	go b.flushLoop()
	return b
}

// Push adds a spend log to the Redis queue.
func (b *RedisBuffer) Push(params db.CreateSpendLogParams) {
	data, err := json.Marshal(params)
	if err != nil {
		log.Printf("warn: marshal spend log: %v", err)
		return
	}
	b.rdb.RPush(b.ctx, spendQueueKey, data)
}

// Flush manually triggers a flush of buffered spend logs.
func (b *RedisBuffer) Flush() {
	b.flush()
}

// Stop stops the flush loop.
func (b *RedisBuffer) Stop() {
	b.cancel()
}

func (b *RedisBuffer) flushLoop() {
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-b.ctx.Done():
			b.flush() // final flush
			return
		case <-ticker.C:
			b.flush()
		}
	}
}

func (b *RedisBuffer) flush() {
	for {
		results, err := b.rdb.LPopCount(b.ctx, spendQueueKey, flushBatchSize).Result()
		if err != nil || len(results) == 0 {
			return
		}

		for _, raw := range results {
			var params db.CreateSpendLogParams
			if err := json.Unmarshal([]byte(raw), &params); err != nil {
				log.Printf("warn: unmarshal spend log: %v", err)
				continue
			}

			if err := b.database.CreateSpendLog(b.ctx, params); err != nil {
				log.Printf("warn: write spend log: %v", err)
			}
		}

		if len(results) < flushBatchSize {
			return
		}
	}
}
