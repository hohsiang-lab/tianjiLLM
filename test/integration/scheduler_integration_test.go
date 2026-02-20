package integration

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/scheduler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type countJob struct {
	name     string
	runCount atomic.Int32
}

func (j *countJob) Name() string { return j.name }

func (j *countJob) Run(_ context.Context) error {
	j.runCount.Add(1)
	return nil
}

func TestScheduler_StartStop(t *testing.T) {
	s := scheduler.New()

	job := &countJob{name: "test_job"}
	s.Add(job, 50*time.Millisecond)

	s.Start()

	// Wait for at least one tick
	time.Sleep(120 * time.Millisecond)
	s.Stop()

	assert.GreaterOrEqual(t, job.runCount.Load(), int32(1))
}

func TestScheduler_StartupRun(t *testing.T) {
	s := scheduler.New()

	job := &countJob{name: "startup_job"}
	s.AddWithStartupRun(job, 10*time.Second) // long interval, should only run at startup

	s.Start()

	// Wait briefly — startup run should fire immediately
	time.Sleep(100 * time.Millisecond)
	s.Stop()

	assert.GreaterOrEqual(t, job.runCount.Load(), int32(1))
}

func TestScheduler_MultipleJobs(t *testing.T) {
	s := scheduler.New()

	job1 := &countJob{name: "job1"}
	job2 := &countJob{name: "job2"}
	s.Add(job1, 50*time.Millisecond)
	s.Add(job2, 50*time.Millisecond)

	s.Start()

	time.Sleep(120 * time.Millisecond)
	s.Stop()

	assert.GreaterOrEqual(t, job1.runCount.Load(), int32(1))
	assert.GreaterOrEqual(t, job2.runCount.Load(), int32(1))
}

func TestScheduler_GracefulShutdown(t *testing.T) {
	s := scheduler.New()

	job := &countJob{name: "graceful_job"}
	s.Add(job, 1*time.Second)

	s.Start()

	// Stop should return without hanging
	done := make(chan struct{})
	go func() {
		s.Stop()
		close(done)
	}()

	select {
	case <-done:
		// OK
	case <-time.After(2 * time.Second):
		require.Fail(t, "scheduler.Stop() hung during graceful shutdown")
	}
}
