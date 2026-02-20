package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type countJob struct {
	name  string
	count atomic.Int32
}

func (j *countJob) Name() string { return j.name }
func (j *countJob) Run(_ context.Context) error {
	j.count.Add(1)
	return nil
}

func TestScheduler_StartStop(t *testing.T) {
	s := New()
	job := &countJob{name: "test"}
	s.Add(job, 50*time.Millisecond)

	s.Start()
	time.Sleep(180 * time.Millisecond)
	s.Stop()

	count := job.count.Load()
	assert.GreaterOrEqual(t, count, int32(2), "job should have run at least 2 times")
}

func TestScheduler_StartupRun(t *testing.T) {
	s := New()
	job := &countJob{name: "startup"}
	s.AddWithStartupRun(job, 1*time.Hour) // long interval, only startup run should fire

	s.Start()
	time.Sleep(50 * time.Millisecond) // give startup run time
	s.Stop()

	assert.Equal(t, int32(1), job.count.Load(), "startup run should execute exactly once")
}

func TestScheduler_PanicRecovery(t *testing.T) {
	s := New()
	panicJob := &panicingJob{name: "panicker"}
	s.Add(panicJob, 50*time.Millisecond)

	s.Start()
	time.Sleep(100 * time.Millisecond)
	s.Stop() // should not deadlock or panic
}

type panicingJob struct{ name string }

func (j *panicingJob) Name() string                { return j.name }
func (j *panicingJob) Run(_ context.Context) error { panic("test panic") }
