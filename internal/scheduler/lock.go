package scheduler

import (
	"context"
	"log"
	"time"

	"github.com/go-redsync/redsync/v4"
	redsyncredis "github.com/go-redsync/redsync/v4/redis/goredis/v9"
	"github.com/redis/go-redis/v9"
)

// DistributedLock wraps a redsync mutex for distributed job locking.
// Multiple scheduler instances (pods) use this to ensure only one
// instance runs a given job at a time.
type DistributedLock struct {
	rs *redsync.Redsync
}

// NewDistributedLock creates a distributed lock manager backed by Redis.
func NewDistributedLock(rdb redis.UniversalClient) *DistributedLock {
	pool := redsyncredis.NewPool(rdb)
	return &DistributedLock{
		rs: redsync.New(pool),
	}
}

// WithLock wraps a Job so it acquires a distributed lock before running.
// If the lock is already held by another instance, the job is skipped.
type WithLock struct {
	inner Job
	lock  *DistributedLock
	ttl   time.Duration
}

// NewWithLock creates a locked job wrapper.
func NewWithLock(inner Job, lock *DistributedLock, ttl time.Duration) *WithLock {
	return &WithLock{
		inner: inner,
		lock:  lock,
		ttl:   ttl,
	}
}

func (w *WithLock) Name() string { return w.inner.Name() }

func (w *WithLock) Run(ctx context.Context) error {
	mutex := w.lock.rs.NewMutex(
		"tianji:scheduler:lock:"+w.inner.Name(),
		redsync.WithExpiry(w.ttl),
		redsync.WithTries(1), // don't retry — just skip if locked
	)

	if err := mutex.LockContext(ctx); err != nil {
		log.Printf("scheduler: job %q skipped (locked by another instance)", w.inner.Name())
		return nil
	}
	defer func() { _, _ = mutex.UnlockContext(ctx) }()

	return w.inner.Run(ctx)
}
