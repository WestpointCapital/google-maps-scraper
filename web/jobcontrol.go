package web

import "context"

// JobController is implemented by the web job runner; optional on Server (nil in tests or non-web modes).
type JobController interface {
	// RequestStop cancels the active scrape and lets the run finish with partial
	// results (same as hitting the wall-clock max time).
	RequestStop(ctx context.Context, jobID string) error
	// RequestPause requests the result pipeline to block until RequestResume. The
	// browser may still finish fetches; backpressure can slow the crawl.
	RequestPause(ctx context.Context, jobID string) error
	// RequestResume releases a pause.
	RequestResume(ctx context.Context, jobID string) error
}
