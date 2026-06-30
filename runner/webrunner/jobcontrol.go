package webrunner

import (
	"context"
	"fmt"
	"log"

	"github.com/gosom/google-maps-scraper/web"
)

func (w *webrunner) registerRunCancel(jobID string, cancel context.CancelFunc) {
	if cancel == nil {
		return
	}

	w.runMu.Lock()
	defer w.runMu.Unlock()

	if w.cancels == nil {
		w.cancels = make(map[string]context.CancelFunc)
	}

	w.cancels[jobID] = cancel
}

func (w *webrunner) unregisterRunCancel(jobID string) {
	w.runMu.Lock()
	defer w.runMu.Unlock()

	if w.cancels != nil {
		delete(w.cancels, jobID)
	}
}

// RequestStop implements web.JobController. Cancels the in-flight scrape; the
// run finishes the same way as a timeout (partial results, then ok / CSV).
func (w *webrunner) RequestStop(ctx context.Context, jobID string) error {
	if jobID == "" {
		return fmt.Errorf("empty job id")
	}

	j, err := w.svc.Get(ctx, jobID)
	if err != nil {
		return err
	}

	if j.Status != web.StatusWorking {
		return fmt.Errorf("only running jobs can be stopped (current status: %s)", j.Status)
	}

	w.svc.SetJobPaused(jobID, false)

	w.runMu.Lock()
	cancel, ok := w.cancels[jobID]
	w.runMu.Unlock()

	if !ok {
		// e.g. still queuing seed jobs, or the main scrape not started yet
		return fmt.Errorf("this job is not cancelable yet; try again in a moment, or use Delete to remove a queued job")
	}

	log.Printf("user requested stop for job %s", jobID)
	cancel()

	return nil
}

// RequestPause implements web.JobController.
func (w *webrunner) RequestPause(ctx context.Context, jobID string) error {
	if jobID == "" {
		return fmt.Errorf("empty job id")
	}

	j, err := w.svc.Get(ctx, jobID)
	if err != nil {
		return err
	}

	if j.Status != web.StatusWorking {
		return fmt.Errorf("only running jobs can be paused (current status: %s)", j.Status)
	}

	w.svc.SetJobPaused(jobID, true)
	log.Printf("user requested pause for job %s", jobID)

	return nil
}

// RequestResume implements web.JobController.
func (w *webrunner) RequestResume(ctx context.Context, jobID string) error {
	if jobID == "" {
		return fmt.Errorf("empty job id")
	}

	j, err := w.svc.Get(ctx, jobID)
	if err != nil {
		return err
	}

	if j.Status != web.StatusWorking {
		return fmt.Errorf("only running jobs can be resumed (current status: %s)", j.Status)
	}

	w.svc.SetJobPaused(jobID, false)
	log.Printf("user requested resume for job %s", jobID)

	return nil
}
