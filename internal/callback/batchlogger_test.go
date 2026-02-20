package callback

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestLogData(model string) LogData {
	return LogData{
		Model:    model,
		Provider: "test",
	}
}

func TestBatchLogger_FlushOnBatchSize(t *testing.T) {
	var flushed [][]LogData
	var mu sync.Mutex

	bl := &BatchLogger{
		batchSize: 3,
		flushFn: func(batch []LogData) error {
			mu.Lock()
			flushed = append(flushed, batch)
			mu.Unlock()
			return nil
		},
		stopCh:      make(chan struct{}),
		flushTicker: time.NewTicker(time.Hour), // won't fire
	}

	bl.LogSuccess(newTestLogData("m1"))
	bl.LogSuccess(newTestLogData("m2"))
	assert.Equal(t, 2, bl.QueueLen())

	bl.LogSuccess(newTestLogData("m3")) // triggers flush

	// flush is synchronous in append path
	mu.Lock()
	require.Len(t, flushed, 1)
	assert.Len(t, flushed[0], 3)
	mu.Unlock()
	assert.Equal(t, 0, bl.QueueLen())
}

func TestBatchLogger_FlushOnTicker(t *testing.T) {
	var flushed [][]LogData
	var mu sync.Mutex

	bl := &BatchLogger{
		batchSize: 100, // won't trigger size flush
		flushFn: func(batch []LogData) error {
			mu.Lock()
			flushed = append(flushed, batch)
			mu.Unlock()
			return nil
		},
		stopCh:      make(chan struct{}),
		flushTicker: time.NewTicker(50 * time.Millisecond),
	}
	bl.Start()
	defer bl.Stop()

	bl.LogSuccess(newTestLogData("m1"))
	bl.LogFailure(newTestLogData("m2"))

	// wait for ticker flush
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	require.GreaterOrEqual(t, len(flushed), 1)
	total := 0
	for _, batch := range flushed {
		total += len(batch)
	}
	mu.Unlock()
	assert.Equal(t, 2, total)
}

func TestBatchLogger_DiscardOnError(t *testing.T) {
	flushCount := 0
	bl := &BatchLogger{
		batchSize: 2,
		flushFn: func(batch []LogData) error {
			flushCount++
			return errors.New("storage unavailable")
		},
		stopCh:      make(chan struct{}),
		flushTicker: time.NewTicker(time.Hour),
	}

	bl.LogSuccess(newTestLogData("m1"))
	bl.LogSuccess(newTestLogData("m2")) // triggers flush, which fails

	assert.Equal(t, 1, flushCount)
	assert.Equal(t, 0, bl.QueueLen()) // batch discarded, not re-queued
}

func TestBatchLogger_StopDrainsQueue(t *testing.T) {
	var flushed [][]LogData
	var mu sync.Mutex

	bl := &BatchLogger{
		batchSize: 100,
		flushFn: func(batch []LogData) error {
			mu.Lock()
			flushed = append(flushed, batch)
			mu.Unlock()
			return nil
		},
		stopCh:      make(chan struct{}),
		flushTicker: time.NewTicker(time.Hour),
	}
	bl.Start()

	bl.LogSuccess(newTestLogData("m1"))
	bl.LogSuccess(newTestLogData("m2"))
	bl.LogSuccess(newTestLogData("m3"))
	assert.Equal(t, 3, bl.QueueLen())

	bl.Stop()

	mu.Lock()
	require.Len(t, flushed, 1)
	assert.Len(t, flushed[0], 3)
	mu.Unlock()
	assert.Equal(t, 0, bl.QueueLen())
}

func TestBatchLogger_StopIdempotent(t *testing.T) {
	bl := &BatchLogger{
		batchSize:   100,
		flushFn:     func(batch []LogData) error { return nil },
		stopCh:      make(chan struct{}),
		flushTicker: time.NewTicker(time.Hour),
	}
	bl.Start()

	bl.Stop()
	bl.Stop() // second call should not panic
}

func TestBatchLogger_ConcurrentLogSuccessFailure(t *testing.T) {
	var totalItems atomic.Int64

	bl := &BatchLogger{
		batchSize: 1000, // large enough to avoid mid-test flushes
		flushFn: func(batch []LogData) error {
			totalItems.Add(int64(len(batch)))
			return nil
		},
		stopCh:      make(chan struct{}),
		flushTicker: time.NewTicker(time.Hour),
	}
	bl.Start()

	var wg sync.WaitGroup
	for range 100 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			bl.LogSuccess(newTestLogData("concurrent"))
		}()
		go func() {
			defer wg.Done()
			bl.LogFailure(newTestLogData("concurrent"))
		}()
	}
	wg.Wait()

	bl.Stop()

	assert.Equal(t, int64(200), totalItems.Load()+int64(bl.QueueLen()))
}

func TestBatchLogger_EmptyFlushNoop(t *testing.T) {
	flushCalled := false
	bl := &BatchLogger{
		batchSize: 100,
		flushFn: func(batch []LogData) error {
			flushCalled = true
			return nil
		},
		stopCh:      make(chan struct{}),
		flushTicker: time.NewTicker(time.Hour),
	}

	bl.flush() // empty queue
	assert.False(t, flushCalled)
}
