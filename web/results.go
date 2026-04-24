package web

import (
	"context"
	"crypto/md5" //nolint:gosec // non-cryptographic dedupe key only
	"encoding/hex"
	"strings"

	"github.com/google/uuid"
	"github.com/gosom/google-maps-scraper/gmaps"
)

// EmailStatus values stored on each result row. The async email enricher
// transitions a row from EmailStatusPending to one of the terminal states.
const (
	EmailStatusSkipped = "skipped" // no usable website (or email feature off)
	EmailStatusPending = "pending" // queued / in-flight in the enricher
	EmailStatusDone    = "done"    // at least one email found
	EmailStatusEmpty   = "empty"   // fetched OK but no emails on the site
	EmailStatusFailed  = "failed"  // network / parse error after retries
)

// JobResultSink persists place rows (implemented by *Service).
//
// The initialEmailStatus argument is honoured ONLY when a row is first
// inserted; on conflict (re-scrape of the same place) the existing
// email_status / emails_json are preserved so the async enricher's work
// is never overwritten by the main pipeline.
type JobResultSink interface {
	UpsertJobResult(ctx context.Context, jobID string, e *gmaps.Entry, initialEmailStatus string) error
}

// JobResult is one scraped place row persisted for live UI and export.
type JobResult struct {
	ID             string `json:"id"`
	JobID          string `json:"job_id"`
	PlaceKey       string `json:"place_key"`
	Title          string `json:"title"`
	Address        string `json:"address"`
	Phone          string `json:"phone"`
	Website        string `json:"website"`
	Rating         float64 `json:"rating"`
	ReviewCount    int     `json:"review_count"`
	CategoriesJSON string  `json:"categories_json"`
	EmailsJSON     string  `json:"emails_json"`
	EmailStatus    string  `json:"email_status"`
	Lat            float64 `json:"lat"`
	Lon            float64 `json:"lon"`
	Link           string  `json:"link"`
	RawJSON        string  `json:"raw_json,omitempty"`
	CreatedAt      int64   `json:"created_at"`
	UpdatedAt      int64   `json:"updated_at"`
}

// EmailStats summarises per-job email enrichment progress for the live UI.
type EmailStats struct {
	Total   int64 `json:"total"`   // total rows for this job
	Done    int64 `json:"done"`    // emails found
	Empty   int64 `json:"empty"`   // fetched but no emails
	Pending int64 `json:"pending"` // still queued / in-flight
	Failed  int64 `json:"failed"`  // hard errors
	Skipped int64 `json:"skipped"` // no usable website
}

// Eligible returns the count of rows that we tried (or are trying) to
// enrich — i.e. everything except 'skipped' rows.
func (e EmailStats) Eligible() int64 {
	return e.Total - e.Skipped
}

// Finished returns the count of eligible rows that have reached a terminal
// state (done / empty / failed).
func (e EmailStats) Finished() int64 {
	return e.Done + e.Empty + e.Failed
}

// ResultRepository stores per-place scrape rows for a job.
type ResultRepository interface {
	UpsertJobResult(ctx context.Context, jobID string, e *gmaps.Entry, initialEmailStatus string) error
	UpdateJobResultEmails(ctx context.Context, jobID, placeKey string, emails []string, status string) error
	ListJobResultsSince(ctx context.Context, jobID string, sinceUnix int64, limit int) ([]JobResult, error)
	ListJobResults(ctx context.Context, jobID string, limit int) ([]JobResult, error)
	CountJobResults(ctx context.Context, jobID string) (int64, error)
	CountJobResultsEmailStats(ctx context.Context, jobID string) (EmailStats, error)
	DeleteJobResults(ctx context.Context, jobID string) error
}

// PlaceDedupeKey returns a stable key for upserts (PlaceID preferred).
func PlaceDedupeKey(e *gmaps.Entry) string {
	if e == nil {
		return ""
	}

	if strings.TrimSpace(e.PlaceID) != "" {
		return strings.TrimSpace(e.PlaceID)
	}

	if strings.TrimSpace(e.DataID) != "" {
		return strings.TrimSpace(e.DataID)
	}

	link := strings.TrimSpace(e.Link)
	if link == "" {
		return uuid.New().String()
	}

	h := md5.Sum([]byte(link)) //nolint:gosec

	return "link:" + hex.EncodeToString(h[:])
}
