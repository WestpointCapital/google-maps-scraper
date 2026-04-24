package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	_ "modernc.org/sqlite" // sqlite driver

	"github.com/gosom/google-maps-scraper/web"
)

type repo struct {
	db *sql.DB
}

func New(path string) (web.Datastore, error) {
	db, err := initDatabase(path)
	if err != nil {
		return nil, err
	}

	return &repo{db: db}, nil
}

func (repo *repo) Get(ctx context.Context, id string) (web.Job, error) {
	const q = `SELECT * from jobs WHERE id = ?`

	row := repo.db.QueryRowContext(ctx, q, id)

	return rowToJob(row)
}

func (repo *repo) Create(ctx context.Context, job *web.Job) error {
	item, err := jobToRow(job)
	if err != nil {
		return err
	}

	const q = `INSERT INTO jobs (id, name, status, data, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`

	_, err = repo.db.ExecContext(ctx, q, item.ID, item.Name, item.Status, item.Data, item.CreatedAt, item.UpdatedAt)
	if err != nil {
		return err
	}

	return nil
}

func (repo *repo) Delete(ctx context.Context, id string) error {
	tx, err := repo.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM job_results WHERE job_id = ?`, id); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM jobs WHERE id = ?`, id); err != nil {
		return err
	}

	return tx.Commit()
}

func (repo *repo) Select(ctx context.Context, params web.SelectParams) ([]web.Job, error) {
	q := `SELECT * from jobs`

	var args []any

	if params.Status != "" {
		q += ` WHERE status = ?`

		args = append(args, params.Status)
	}

	q += " ORDER BY created_at DESC"

	if params.Limit > 0 {
		q += " LIMIT ?"

		args = append(args, params.Limit)
	}

	rows, err := repo.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var ans []web.Job

	for rows.Next() {
		job, err := rowToJob(rows)
		if err != nil {
			return nil, err
		}

		ans = append(ans, job)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return ans, nil
}

func (repo *repo) Update(ctx context.Context, job *web.Job) error {
	item, err := jobToRow(job)
	if err != nil {
		return err
	}

	const q = `UPDATE jobs SET name = ?, status = ?, data = ?, updated_at = ? WHERE id = ?`

	_, err = repo.db.ExecContext(ctx, q, item.Name, item.Status, item.Data, item.UpdatedAt, item.ID)

	return err
}

type scannable interface {
	Scan(dest ...any) error
}

func rowToJob(row scannable) (web.Job, error) {
	var j job

	err := row.Scan(&j.ID, &j.Name, &j.Status, &j.Data, &j.CreatedAt, &j.UpdatedAt)
	if err != nil {
		return web.Job{}, err
	}

	ans := web.Job{
		ID:     j.ID,
		Name:   j.Name,
		Status: j.Status,
		Date:   time.Unix(j.CreatedAt, 0).UTC(),
	}

	err = json.Unmarshal([]byte(j.Data), &ans.Data)
	if err != nil {
		return web.Job{}, err
	}

	return ans, nil
}

func jobToRow(item *web.Job) (job, error) {
	data, err := json.Marshal(item.Data)
	if err != nil {
		return job{}, err
	}

	return job{
		ID:        item.ID,
		Name:      item.Name,
		Status:    item.Status,
		Data:      string(data),
		CreatedAt: item.Date.Unix(),
		UpdatedAt: time.Now().UTC().Unix(),
	}, nil
}

type job struct {
	ID        string
	Name      string
	Status    string
	Data      string
	CreatedAt int64
	UpdatedAt int64
}

func initDatabase(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(30 * time.Minute)

	_, err = db.Exec("PRAGMA busy_timeout = 5000")
	if err != nil {
		return nil, err
	}

	_, err = db.Exec("PRAGMA journal_mode=WAL")
	if err != nil {
		return nil, err
	}

	_, err = db.Exec("PRAGMA synchronous=NORMAL")
	if err != nil {
		return nil, err
	}

	_, err = db.Exec("PRAGMA cache_size=1000")
	if err != nil {
		return nil, err
	}

	err = db.Ping()
	if err != nil {
		return nil, err
	}

	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		return nil, err
	}

	return db, createSchema(db)
}

func createSchema(db *sql.DB) error {
	// Base tables only first. Legacy databases may already have job_results
	// without email_status / updated_at — we must ALTER those in before any
	// CREATE INDEX that references the new columns, or startup and queries
	// fail with "no such column: updated_at".
	baseStmts := []string{
		`CREATE TABLE IF NOT EXISTS jobs (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			status TEXT NOT NULL,
			data TEXT NOT NULL,
			created_at INT NOT NULL,
			updated_at INT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS job_presets (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			data TEXT NOT NULL,
			created_at INT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS job_results (
			id TEXT PRIMARY KEY,
			job_id TEXT NOT NULL,
			place_key TEXT NOT NULL,
			title TEXT,
			address TEXT,
			phone TEXT,
			website TEXT,
			rating REAL,
			review_count INT,
			categories_json TEXT,
			emails_json TEXT,
			email_status TEXT NOT NULL DEFAULT '',
			lat REAL,
			lon REAL,
			link TEXT,
			raw_json TEXT,
			created_at INT NOT NULL,
			updated_at INT NOT NULL DEFAULT 0,
			UNIQUE(job_id, place_key)
		)`,
	}

	for _, s := range baseStmts {
		if _, err := db.Exec(s); err != nil {
			return err
		}
	}

	if err := addColumnIfMissing(db, "job_results", "email_status", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}

	if err := addColumnIfMissing(db, "job_results", "updated_at", "INT NOT NULL DEFAULT 0"); err != nil {
		return err
	}

	// Rows inserted before updated_at existed may have 0; align with created_at
	// so live polling (?since=0) still returns historical rows once.
	if _, err := db.Exec(`UPDATE job_results SET updated_at = created_at WHERE updated_at IS NULL OR updated_at = 0`); err != nil {
		return err
	}

	indexStmts := []string{
		`CREATE INDEX IF NOT EXISTS idx_job_results_job_created ON job_results(job_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_job_results_job_updated ON job_results(job_id, updated_at)`,
		`CREATE INDEX IF NOT EXISTS idx_job_results_email_status ON job_results(job_id, email_status)`,
	}

	for _, s := range indexStmts {
		if _, err := db.Exec(s); err != nil {
			return err
		}
	}

	return nil
}

// addColumnIfMissing applies a no-op ALTER when the column already exists,
// so re-opening an older database file does not error out at startup.
func addColumnIfMissing(db *sql.DB, table, col, ddl string) error {
	rows, err := db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return err
	}

	defer rows.Close()

	for rows.Next() {
		var (
			cid     int
			name    string
			ctype   string
			notnull int
			dflt    sql.NullString
			pk      int
		)

		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}

		if name == col {
			return nil
		}
	}

	if err := rows.Err(); err != nil {
		return err
	}

	_, err = db.Exec("ALTER TABLE " + table + " ADD COLUMN " + col + " " + ddl)

	return err
}
