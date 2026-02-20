package callback

import (
	"log"
	"os"
	"strconv"
	"sync"
	"time"
)

// BatchLogger provides a shared base for callbacks that batch log entries
// and flush them periodically or when a size threshold is reached.
// Matches Python LiteLLM's CustomBatchLogger behavior.
type BatchLogger struct {
	mu          sync.Mutex
	queue       []LogData
	batchSize   int
	flushTicker *time.Ticker
	flushFn     func(batch []LogData) error
	stopCh      chan struct{}
	stopped     bool
}

// NewBatchLogger creates a BatchLogger with the given flush function.
// batchSize and flushInterval can be overridden via DEFAULT_BATCH_SIZE
// and DEFAULT_FLUSH_INTERVAL_SECONDS environment variables.
func NewBatchLogger(flushFn func(batch []LogData) error) *BatchLogger {
	batchSize := 512
	if v := os.Getenv("DEFAULT_BATCH_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			batchSize = n
		}
	}

	interval := 5 * time.Second
	if v := os.Getenv("DEFAULT_FLUSH_INTERVAL_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			interval = time.Duration(n) * time.Second
		}
	}

	return &BatchLogger{
		batchSize:   batchSize,
		flushFn:     flushFn,
		stopCh:      make(chan struct{}),
		flushTicker: time.NewTicker(interval),
	}
}

// Start begins the periodic flush goroutine.
func (b *BatchLogger) Start() {
	go func() {
		for {
			select {
			case <-b.flushTicker.C:
				b.flush()
			case <-b.stopCh:
				return
			}
		}
	}()
}

// LogSuccess appends a success log entry and flushes if batch is full.
func (b *BatchLogger) LogSuccess(data LogData) {
	b.append(data)
}

// LogFailure appends a failure log entry and flushes if batch is full.
func (b *BatchLogger) LogFailure(data LogData) {
	b.append(data)
}

func (b *BatchLogger) append(data LogData) {
	b.mu.Lock()
	b.queue = append(b.queue, data)
	shouldFlush := len(b.queue) >= b.batchSize
	b.mu.Unlock()

	if shouldFlush {
		b.flush()
	}
}

// flush takes all queued items and calls flushFn.
// On error: discard batch and log error (matching Python behavior).
func (b *BatchLogger) flush() {
	b.mu.Lock()
	if len(b.queue) == 0 {
		b.mu.Unlock()
		return
	}
	batch := b.queue
	b.queue = nil
	b.mu.Unlock()

	if err := b.flushFn(batch); err != nil {
		log.Printf("batch flush failed (%d items discarded): %v", len(batch), err)
	}
}

// Stop flushes remaining items and stops the ticker.
func (b *BatchLogger) Stop() {
	b.mu.Lock()
	if b.stopped {
		b.mu.Unlock()
		return
	}
	b.stopped = true
	b.mu.Unlock()

	b.flushTicker.Stop()
	close(b.stopCh)
	b.flush()
}

// QueueLen returns the current queue length (for testing).
func (b *BatchLogger) QueueLen() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.queue)
}
