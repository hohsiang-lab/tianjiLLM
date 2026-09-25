package strategy

import (
	"container/heap"
	"sync"

	"github.com/praxisllmlab/tianjiLLM/internal/router"
)

// PriorityQueue implements a weighted priority queue for request scheduling.
// Lower priority number = higher priority (served first).
// Same-priority uses FIFO ordering.
type PriorityQueue struct {
	mu    sync.Mutex
	items priorityHeap
	seq   int64 // monotonic counter for FIFO ordering within same priority
	inner router.Strategy
}

// NewPriorityQueue creates a priority-based scheduler that wraps an inner strategy
// for actual deployment selection.
func NewPriorityQueue(inner router.Strategy) *PriorityQueue {
	return &PriorityQueue{
		inner: inner,
	}
}

// QueueItem represents a queued request with its priority.
type QueueItem struct {
	Priority     int
	DeploymentID string
	seq          int64 // insertion order for FIFO within same priority
	index        int   // heap index
}

// Enqueue adds an item to the priority queue.
func (pq *PriorityQueue) Enqueue(deploymentID string, priority int) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	pq.seq++
	item := &QueueItem{
		Priority:     priority,
		DeploymentID: deploymentID,
		seq:          pq.seq,
	}
	heap.Push(&pq.items, item)
}

// Dequeue removes and returns the highest-priority item (lowest number).
// Returns nil if queue is empty.
func (pq *PriorityQueue) Dequeue() *QueueItem {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if pq.items.Len() == 0 {
		return nil
	}
	item, _ := heap.Pop(&pq.items).(*QueueItem)
	return item
}

// Len returns the number of queued items.
func (pq *PriorityQueue) Len() int {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	return pq.items.Len()
}

// Pick delegates to the inner strategy for deployment selection.
func (pq *PriorityQueue) Pick(deployments []*router.Deployment) *router.Deployment {
	return pq.inner.Pick(deployments)
}

// priorityHeap implements heap.Interface for priority queue items.
type priorityHeap []*QueueItem

func (h priorityHeap) Len() int { return len(h) }

func (h priorityHeap) Less(i, j int) bool {
	if h[i].Priority != h[j].Priority {
		return h[i].Priority < h[j].Priority
	}
	return h[i].seq < h[j].seq // FIFO within same priority
}

func (h priorityHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *priorityHeap) Push(x any) {
	item, _ := x.(*QueueItem)
	item.index = len(*h)
	*h = append(*h, item)
}

func (h *priorityHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*h = old[:n-1]
	return item
}
