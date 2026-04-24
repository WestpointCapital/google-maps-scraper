package web

import (
	"context"
	"sync"
	"testing"
	"time"
)

type fakeUpdater struct {
	mu      sync.Mutex
	updates []updateCall
}

type updateCall struct {
	jobID    string
	placeKey string
	emails   []string
	status   string
}

func (f *fakeUpdater) UpdateJobResultEmails(_ context.Context, jobID, placeKey string, emails []string, status string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.updates = append(f.updates, updateCall{
		jobID:    jobID,
		placeKey: placeKey,
		emails:   append([]string(nil), emails...),
		status:   status,
	})

	return nil
}

func (f *fakeUpdater) snapshot() []updateCall {
	f.mu.Lock()
	defer f.mu.Unlock()

	out := make([]updateCall, len(f.updates))
	copy(out, f.updates)

	return out
}

func TestEmailEnricher_SkipsInvalidEntries(t *testing.T) {
	t.Parallel()

	upd := &fakeUpdater{}
	en := NewEmailEnricher(upd, 4)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	en.Start(ctx)

	if got := en.Submit("", "pk", nil); got {
		t.Fatalf("Submit should return false for nil entry / empty job id")
	}

	if got := en.PendingForJob("nope"); got != 0 {
		t.Fatalf("PendingForJob for unknown job: got %d want 0", got)
	}

	en.Close()

	if got := len(upd.snapshot()); got != 0 {
		t.Fatalf("expected zero updates, got %d", got)
	}
}

func TestEmailEnricher_WaitForJobReturnsImmediatelyWhenIdle(t *testing.T) {
	t.Parallel()

	upd := &fakeUpdater{}
	en := NewEmailEnricher(upd, 2)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	en.Start(ctx)

	start := time.Now()
	en.WaitForJob(ctx, "no-such-job", 200*time.Millisecond)

	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("WaitForJob blocked too long for idle job: %s", elapsed)
	}

	en.Close()
}
