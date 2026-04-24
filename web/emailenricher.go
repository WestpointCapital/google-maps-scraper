package web

import (
	"context"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gosom/google-maps-scraper/gmaps"
)

// EmailUpdater is the narrow persistence interface the enricher depends on.
type EmailUpdater interface {
	UpdateJobResultEmails(ctx context.Context, jobID, placeKey string, emails []string, status string) error
}

// enrichTask is the unit of work flowing through the enricher's queue.
type enrichTask struct {
	jobID    string
	placeKey string
	entry    *gmaps.Entry
}

// EmailEnricher is a shared, persistent worker pool that fetches emails for
// scraped places using plain HTTP only (never Playwright). Submissions are
// non-blocking, so the main Google Maps scrape pipeline is never held up
// waiting for slow contact pages.
//
// Lifecycle:
//
//	enricher := web.NewEmailEnricher(svc, 16)
//	enricher.Start(ctx)
//	enricher.Submit(jobID, placeKey, entry)
//	enricher.WaitForJob(ctx, jobID, deadline)  // optional: drain a job's queue
//	enricher.Close()                           // on webrunner shutdown
type EmailEnricher struct {
	sink    EmailUpdater
	queue   chan enrichTask
	workers int
	client  *http.Client
	once    sync.Once

	pendingMu sync.Mutex
	pending   map[string]*int64 // jobID -> live in-flight + queued counter

	closeOnce sync.Once
	closed    chan struct{}

	wg sync.WaitGroup
}

// NewEmailEnricher returns a pool with the given worker count. workers <= 0
// defaults to 16, which empirically keeps a 1 Gbps host and Playwright-free
// HTTP fetches well below saturation.
func NewEmailEnricher(sink EmailUpdater, workers int) *EmailEnricher {
	if workers <= 0 {
		workers = 16
	}

	return &EmailEnricher{
		sink:    sink,
		queue:   make(chan enrichTask, 8192),
		workers: workers,
		client:  gmaps.NewFastEmailHTTPClient(12 * time.Second),
		pending: make(map[string]*int64),
		closed:  make(chan struct{}),
	}
}

// Start spawns the worker goroutines. Calling Start more than once is a no-op.
func (e *EmailEnricher) Start(ctx context.Context) {
	e.once.Do(func() {
		for i := 0; i < e.workers; i++ {
			e.wg.Add(1)

			go e.run(ctx)
		}
	})
}

// Submit queues an entry for async email enrichment. Returns false if the
// queue is saturated or the enricher has been Closed (the row keeps its
// 'pending' status, which the job drain step will surface so the user knows).
func (e *EmailEnricher) Submit(jobID, placeKey string, entry *gmaps.Entry) bool {
	if entry == nil || jobID == "" || placeKey == "" {
		return false
	}

	select {
	case <-e.closed:
		return false
	default:
	}

	counter := e.counterFor(jobID)
	atomic.AddInt64(counter, 1)

	select {
	case e.queue <- enrichTask{jobID: jobID, placeKey: placeKey, entry: entry}:
		return true
	case <-e.closed:
		atomic.AddInt64(counter, -1)

		return false
	default:
		// Queue is full; revert the counter and let the row stay 'pending'.
		atomic.AddInt64(counter, -1)

		return false
	}
}

// PendingForJob returns the number of in-flight + queued tasks for a job.
func (e *EmailEnricher) PendingForJob(jobID string) int64 {
	if jobID == "" {
		return 0
	}

	e.pendingMu.Lock()
	c, ok := e.pending[jobID]
	e.pendingMu.Unlock()

	if !ok {
		return 0
	}

	return atomic.LoadInt64(c)
}

// WaitForJob blocks until either every queued+in-flight task for jobID has
// completed, the deadline elapses, or ctx is cancelled. It is safe to call
// from the webrunner right after the main scrape ends so the CSV can be
// regenerated with the freshly enriched emails.
func (e *EmailEnricher) WaitForJob(ctx context.Context, jobID string, deadline time.Duration) {
	if deadline <= 0 {
		deadline = 5 * time.Minute
	}

	end := time.Now().Add(deadline)
	tick := time.NewTicker(500 * time.Millisecond)

	defer tick.Stop()

	for {
		if e.PendingForJob(jobID) == 0 {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if time.Now().After(end) {
				log.Printf("email enricher: WaitForJob(%s) deadline reached with %d pending", jobID, e.PendingForJob(jobID))

				return
			}
		}
	}
}

// Close drains in-flight work and stops all workers. Safe to call any number
// of times; only the first call closes the underlying channel.
func (e *EmailEnricher) Close() {
	e.closeOnce.Do(func() {
		close(e.closed)
		close(e.queue)
	})

	e.wg.Wait()
}

func (e *EmailEnricher) counterFor(jobID string) *int64 {
	e.pendingMu.Lock()
	defer e.pendingMu.Unlock()

	c, ok := e.pending[jobID]
	if !ok {
		var zero int64
		c = &zero
		e.pending[jobID] = c
	}

	return c
}

func (e *EmailEnricher) run(ctx context.Context) {
	defer e.wg.Done()

	for task := range e.queue {
		e.process(ctx, task)
	}
}

// process performs one enrichment, persists the result, and decrements the
// per-job pending counter. Errors are logged but never propagated, so a
// single bad website cannot halt the worker.
func (e *EmailEnricher) process(parent context.Context, task enrichTask) {
	defer func() {
		counter := e.counterFor(task.jobID)
		atomic.AddInt64(counter, -1)
	}()

	// Hard cap per-site work so one slow domain can never tie up a worker
	// for longer than this. Includes the redirect chain + up to 2 contact
	// subpages, each with its own client-level timeout.
	ctx, cancel := context.WithTimeout(parent, 35*time.Second)
	defer cancel()

	err := gmaps.FetchEmailsForEntry(ctx, e.client, task.entry)

	status := EmailStatusEmpty
	switch {
	case err != nil && len(task.entry.Emails) == 0:
		status = EmailStatusFailed
	case len(task.entry.Emails) > 0:
		status = EmailStatusDone
	}

	// Persist using the parent context — even if our per-task timeout
	// fired, we still want to record what we found before bailing out.
	if uerr := e.sink.UpdateJobResultEmails(parent, task.jobID, task.placeKey, task.entry.Emails, status); uerr != nil {
		log.Printf("email enricher: persist %s/%s: %v", task.jobID, task.placeKey, uerr)
	}
}
