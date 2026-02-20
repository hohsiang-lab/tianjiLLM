package strategy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPriorityQueue_PriorityOrdering(t *testing.T) {
	pq := NewPriorityQueue(NewShuffle())

	pq.Enqueue("low-priority", 10)
	pq.Enqueue("high-priority", 1)
	pq.Enqueue("medium-priority", 5)

	item := pq.Dequeue()
	require.NotNil(t, item)
	assert.Equal(t, "high-priority", item.DeploymentID)
	assert.Equal(t, 1, item.Priority)

	item = pq.Dequeue()
	require.NotNil(t, item)
	assert.Equal(t, "medium-priority", item.DeploymentID)
	assert.Equal(t, 5, item.Priority)

	item = pq.Dequeue()
	require.NotNil(t, item)
	assert.Equal(t, "low-priority", item.DeploymentID)
	assert.Equal(t, 10, item.Priority)
}

func TestPriorityQueue_FIFOWithinSamePriority(t *testing.T) {
	pq := NewPriorityQueue(NewShuffle())

	pq.Enqueue("first", 5)
	pq.Enqueue("second", 5)
	pq.Enqueue("third", 5)

	item := pq.Dequeue()
	assert.Equal(t, "first", item.DeploymentID)

	item = pq.Dequeue()
	assert.Equal(t, "second", item.DeploymentID)

	item = pq.Dequeue()
	assert.Equal(t, "third", item.DeploymentID)
}

func TestPriorityQueue_EmptyDequeue(t *testing.T) {
	pq := NewPriorityQueue(NewShuffle())
	assert.Nil(t, pq.Dequeue())
	assert.Equal(t, 0, pq.Len())
}

func TestPriorityQueue_DefaultPriority(t *testing.T) {
	pq := NewPriorityQueue(NewShuffle())

	// Priority 0 is the default (highest priority)
	pq.Enqueue("default-priority", 0)
	pq.Enqueue("low-priority", 10)

	item := pq.Dequeue()
	assert.Equal(t, "default-priority", item.DeploymentID)
}

func TestPriorityQueue_Pick(t *testing.T) {
	pq := NewPriorityQueue(NewShuffle())
	deps := makeDeployments(3)

	// Pick delegates to inner strategy
	picked := pq.Pick(deps)
	require.NotNil(t, picked)
}
