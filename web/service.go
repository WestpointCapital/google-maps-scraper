package web

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gosom/google-maps-scraper/gmaps"
)

type Service struct {
	store      Datastore
	dataFolder string
}

func NewService(store Datastore, dataFolder string) *Service {
	return &Service{
		store:      store,
		dataFolder: dataFolder,
	}
}

func (s *Service) Create(ctx context.Context, job *Job) error {
	return s.store.Create(ctx, job)
}

func (s *Service) All(ctx context.Context) ([]Job, error) {
	return s.store.Select(ctx, SelectParams{})
}

func (s *Service) Get(ctx context.Context, id string) (Job, error) {
	return s.store.Get(ctx, id)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	if strings.Contains(id, "/") || strings.Contains(id, "\\") || strings.Contains(id, "..") {
		return fmt.Errorf("invalid file name")
	}

	datapath := filepath.Join(s.dataFolder, id+".csv")

	if _, err := os.Stat(datapath); err == nil {
		if err := os.Remove(datapath); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	return s.store.Delete(ctx, id)
}

func (s *Service) Update(ctx context.Context, job *Job) error {
	return s.store.Update(ctx, job)
}

func (s *Service) SelectPending(ctx context.Context) ([]Job, error) {
	return s.store.Select(ctx, SelectParams{Status: StatusPending, Limit: 1})
}

func (s *Service) ListPresets(ctx context.Context) ([]JobPreset, error) {
	return s.store.ListPresets(ctx)
}

func (s *Service) GetPreset(ctx context.Context, id string) (JobPreset, error) {
	return s.store.GetPreset(ctx, id)
}

func (s *Service) CreatePreset(ctx context.Context, p *JobPreset) error {
	return s.store.CreatePreset(ctx, p)
}

func (s *Service) DeletePreset(ctx context.Context, id string) error {
	return s.store.DeletePreset(ctx, id)
}

// UpsertJobResult persists one place row for live results / export.
// initialEmailStatus is honoured only on insert; on conflict (re-scrape) the
// existing email_status / emails_json are preserved.
func (s *Service) UpsertJobResult(ctx context.Context, jobID string, e *gmaps.Entry, initialEmailStatus string) error {
	if e == nil {
		return nil
	}

	return s.store.UpsertJobResult(ctx, jobID, e, initialEmailStatus)
}

// UpdateJobResultEmails records the async email enricher's output for one row.
func (s *Service) UpdateJobResultEmails(ctx context.Context, jobID, placeKey string, emails []string, status string) error {
	return s.store.UpdateJobResultEmails(ctx, jobID, placeKey, emails, status)
}

// ListJobResultsSince returns rows with updated_at strictly after sinceUnix.
// Polling on updated_at means email enrichment edits stream into the live UI
// the moment they land, not just initial inserts.
func (s *Service) ListJobResultsSince(ctx context.Context, jobID string, sinceUnix int64, limit int) ([]JobResult, error) {
	return s.store.ListJobResultsSince(ctx, jobID, sinceUnix, limit)
}

// ListJobResults returns every row for a job ordered by first-seen time.
func (s *Service) ListJobResults(ctx context.Context, jobID string, limit int) ([]JobResult, error) {
	return s.store.ListJobResults(ctx, jobID, limit)
}

// CountJobResults returns how many rows are stored for a job.
func (s *Service) CountJobResults(ctx context.Context, jobID string) (int64, error) {
	return s.store.CountJobResults(ctx, jobID)
}

// CountJobResultsEmailStats returns counts grouped by email_status.
func (s *Service) CountJobResultsEmailStats(ctx context.Context, jobID string) (EmailStats, error) {
	return s.store.CountJobResultsEmailStats(ctx, jobID)
}

func (s *Service) GetCSV(_ context.Context, id string) (string, error) {
	if strings.Contains(id, "/") || strings.Contains(id, "\\") || strings.Contains(id, "..") {
		return "", fmt.Errorf("invalid file name")
	}

	datapath := filepath.Join(s.dataFolder, id+".csv")

	if _, err := os.Stat(datapath); os.IsNotExist(err) {
		return "", fmt.Errorf("csv file not found for job %s", id)
	}

	return datapath, nil
}
