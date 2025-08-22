package worker

import (
	"context"
	"sync"

	"github.com/rs/zerolog/log"
)

// Job represents a job to be executed by a worker
type Job struct {
	ID       string
	Execute  func(ctx context.Context) (interface{}, error)
	Callback func(result interface{}, err error)
}

// Pool represents a worker pool
type Pool struct {
	numWorkers int
	jobs       chan Job
	wg         sync.WaitGroup
}

// NewPool creates a new worker pool
func NewPool(numWorkers int) *Pool {
	return &Pool{
		numWorkers: numWorkers,
		jobs:       make(chan Job),
	}
}

// Start starts the worker pool
func (p *Pool) Start(ctx context.Context) {
	for i := 0; i < p.numWorkers; i++ {
		p.wg.Add(1)
		go p.worker(ctx, i+1)
	}
	log.Info().Msgf("Worker pool started with %d workers", p.numWorkers)
}

// Stop stops the worker pool and waits for all jobs to complete
func (p *Pool) Stop() {
	close(p.jobs)
	p.wg.Wait()
	log.Info().Msg("Worker pool stopped")
}

// Submit submits a job to the worker pool
func (p *Pool) Submit(job Job) {
	p.jobs <- job
}

// worker is the main worker function
func (p *Pool) worker(ctx context.Context, id int) {
	defer p.wg.Done()
	log.Debug().Msgf("Worker %d started", id)

	for {
		select {
		case job, ok := <-p.jobs:
			if !ok {
				log.Debug().Msgf("Worker %d stopping", id)
				return
			}

			log.Debug().Msgf("Worker %d processing job %s", id, job.ID)
			result, err := job.Execute(ctx)
			if job.Callback != nil {
				job.Callback(result, err)
			}
		case <-ctx.Done():
			log.Debug().Msgf("Worker %d stopping due to context cancellation", id)
			return
		}
	}
}
