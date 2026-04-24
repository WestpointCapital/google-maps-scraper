package sqlite

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/gosom/google-maps-scraper/gmaps"
	"github.com/gosom/google-maps-scraper/web"
)

// UpsertJobResult inserts or refreshes the place row for (jobID, place_key).
//
// On INSERT we set email_status = initialEmailStatus and emails_json from the
// supplied entry. On CONFLICT (re-scrape of the same place) we DELIBERATELY
// preserve the existing email_status and emails_json so the async email
// enricher's results can never be overwritten by the main scraper.
func (repo *repo) UpsertJobResult(ctx context.Context, jobID string, e *gmaps.Entry, initialEmailStatus string) error {
	if e == nil || strings.TrimSpace(jobID) == "" {
		return nil
	}

	pk := web.PlaceDedupeKey(e)
	if pk == "" {
		return nil
	}

	cat, err := json.Marshal(e.Categories)
	if err != nil {
		cat = []byte("[]")
	}

	emails, err := json.Marshal(e.Emails)
	if err != nil {
		emails = []byte("[]")
	}

	raw, err := json.Marshal(e)
	if err != nil {
		raw = []byte("{}")
	}

	rowID := uuid.New().String()
	now := time.Now().UTC().Unix()

	const q = `
INSERT INTO job_results (
	id, job_id, place_key, title, address, phone, website,
	rating, review_count, categories_json, emails_json, email_status,
	lat, lon, link, raw_json, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(job_id, place_key) DO UPDATE SET
	title = excluded.title,
	address = excluded.address,
	phone = excluded.phone,
	website = excluded.website,
	rating = excluded.rating,
	review_count = excluded.review_count,
	categories_json = excluded.categories_json,
	-- emails_json and email_status preserved on conflict so the async
	-- enricher's work is never clobbered by a later main-pipeline scrape.
	lat = excluded.lat,
	lon = excluded.lon,
	link = excluded.link,
	raw_json = excluded.raw_json,
	updated_at = excluded.updated_at
`

	_, err = repo.db.ExecContext(ctx, q,
		rowID,
		jobID,
		pk,
		e.Title,
		e.Address,
		e.Phone,
		e.WebSite,
		e.ReviewRating,
		e.ReviewCount,
		string(cat),
		string(emails),
		initialEmailStatus,
		e.Latitude,
		e.Longtitude,
		e.Link,
		string(raw),
		now,
		now,
	)

	return err
}

// UpdateJobResultEmails atomically writes enricher output for one row.
// It also bumps updated_at so the live UI's polling endpoint can stream
// the new emails to the open dialog.
func (repo *repo) UpdateJobResultEmails(ctx context.Context, jobID, placeKey string, emails []string, status string) error {
	if strings.TrimSpace(jobID) == "" || strings.TrimSpace(placeKey) == "" {
		return nil
	}

	js, err := json.Marshal(emails)
	if err != nil {
		js = []byte("[]")
	}

	const q = `
UPDATE job_results
SET emails_json = ?, email_status = ?, updated_at = ?
WHERE job_id = ? AND place_key = ?
`

	_, err = repo.db.ExecContext(ctx, q, string(js), status, time.Now().UTC().Unix(), jobID, placeKey)

	return err
}

func (repo *repo) ListJobResultsSince(ctx context.Context, jobID string, sinceUnix int64, limit int) ([]web.JobResult, error) {
	if limit <= 0 || limit > 5000 {
		limit = 500
	}

	const q = `
SELECT id, job_id, place_key, title, address, phone, website,
	rating, review_count, categories_json, emails_json, email_status,
	lat, lon, link, raw_json, created_at, updated_at
FROM job_results
WHERE job_id = ? AND updated_at > ?
ORDER BY updated_at ASC
LIMIT ?
`

	rows, err := repo.db.QueryContext(ctx, q, jobID, sinceUnix, limit)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	return scanJobResultRows(rows)
}

// ListJobResults returns rows ordered by first-seen time. Used for CSV regeneration.
func (repo *repo) ListJobResults(ctx context.Context, jobID string, limit int) ([]web.JobResult, error) {
	q := `
SELECT id, job_id, place_key, title, address, phone, website,
	rating, review_count, categories_json, emails_json, email_status,
	lat, lon, link, raw_json, created_at, updated_at
FROM job_results
WHERE job_id = ?
ORDER BY created_at ASC
`

	args := []any{jobID}

	if limit > 0 {
		q += " LIMIT ?"

		args = append(args, limit)
	}

	rows, err := repo.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	return scanJobResultRows(rows)
}

func scanJobResultRows(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
}) ([]web.JobResult, error) {
	var out []web.JobResult

	for rows.Next() {
		var r web.JobResult

		err := rows.Scan(
			&r.ID, &r.JobID, &r.PlaceKey, &r.Title, &r.Address, &r.Phone, &r.Website,
			&r.Rating, &r.ReviewCount, &r.CategoriesJSON, &r.EmailsJSON, &r.EmailStatus,
			&r.Lat, &r.Lon, &r.Link, &r.RawJSON, &r.CreatedAt, &r.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		out = append(out, r)
	}

	return out, rows.Err()
}

func (repo *repo) CountJobResults(ctx context.Context, jobID string) (int64, error) {
	const q = `SELECT COUNT(1) FROM job_results WHERE job_id = ?`

	var n int64

	err := repo.db.QueryRowContext(ctx, q, jobID).Scan(&n)

	return n, err
}

// CountJobResultsEmailStats returns counts grouped by email_status, plus the total.
func (repo *repo) CountJobResultsEmailStats(ctx context.Context, jobID string) (web.EmailStats, error) {
	var stats web.EmailStats

	const q = `
SELECT
	COUNT(1) AS total,
	SUM(CASE WHEN email_status = 'done'    THEN 1 ELSE 0 END) AS done,
	SUM(CASE WHEN email_status = 'empty'   THEN 1 ELSE 0 END) AS empty,
	SUM(CASE WHEN email_status = 'pending' THEN 1 ELSE 0 END) AS pending,
	SUM(CASE WHEN email_status = 'failed'  THEN 1 ELSE 0 END) AS failed,
	SUM(CASE WHEN email_status = 'skipped' THEN 1 ELSE 0 END) AS skipped
FROM job_results
WHERE job_id = ?
`

	err := repo.db.QueryRowContext(ctx, q, jobID).Scan(
		&stats.Total,
		&stats.Done,
		&stats.Empty,
		&stats.Pending,
		&stats.Failed,
		&stats.Skipped,
	)

	return stats, err
}

func (repo *repo) DeleteJobResults(ctx context.Context, jobID string) error {
	const q = `DELETE FROM job_results WHERE job_id = ?`

	_, err := repo.db.ExecContext(ctx, q, jobID)

	return err
}
