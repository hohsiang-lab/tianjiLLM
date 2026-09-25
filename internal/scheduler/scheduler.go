package scheduler

import (
	"context"
	"log"
	"sync"
	"time"
)

// Job is the interface for a background job.
type Job interface {
	Name() string
	Run(ctx context.Context) error
}

// entry holds a registered job and its interval.
type entry struct {
	job        Job
	interval   time.Duration
	runOnStart bool
}

// Scheduler runs background jobs at fixed intervals.
type Scheduler struct {
	entries []entry
	wg      sync.WaitGroup
	ctx     context.Context
	cancel  context.CancelFunc
}

// New creates a new scheduler.
func New() *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		ctx:    ctx,
		cancel: cancel,
	}
}

// Add registers a job to run at the given interval.
func (s *Scheduler) Add(job Job, interval time.Duration) {
	s.entries = append(s.entries, entry{
		job:      job,
		interval: interval,
	})
}

// AddWithStartupRun registers a job that runs immediately at startup,
// then at the given interval. Used for catch-up scenarios (e.g., budget reset).
func (s *Scheduler) AddWithStartupRun(job Job, interval time.Duration) {
	s.entries = append(s.entries, entry{
		job:        job,
		interval:   interval,
		runOnStart: true,
	})
}

// Start begins running all registered jobs in background goroutines.
func (s *Scheduler) Start() {
	for _, e := range s.entries {
		s.wg.Add(1)
		go s.runJob(e)
	}
	log.Printf("scheduler started with %d jobs", len(s.entries))
}

// Stop cancels all running jobs and waits for them to finish.
func (s *Scheduler) Stop() {
	s.cancel()
	s.wg.Wait()
	log.Println("scheduler stopped")
}

func (s *Scheduler) runJob(e entry) {
	defer s.wg.Done()

	if e.runOnStart {
		s.executeJob(e.job)
	}

	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.executeJob(e.job)
		}
	}
}

func (s *Scheduler) executeJob(job Job) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("scheduler: job %q panicked: %v", job.Name(), r)
		}
	}()

	if err := job.Run(s.ctx); err != nil {
		log.Printf("scheduler: job %q error: %v", job.Name(), err)
	}
}
