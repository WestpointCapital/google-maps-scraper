package sqlite

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gosom/google-maps-scraper/web"
)

func (repo *repo) CreatePreset(ctx context.Context, p *web.JobPreset) error {
	data, err := json.Marshal(p.Data)
	if err != nil {
		return err
	}

	const q = `INSERT INTO job_presets (id, name, data, created_at) VALUES (?, ?, ?, ?)`

	_, err = repo.db.ExecContext(ctx, q, p.ID, p.Name, string(data), p.CreatedAt.Unix())

	return err
}

func (repo *repo) DeletePreset(ctx context.Context, id string) error {
	const q = `DELETE FROM job_presets WHERE id = ?`

	_, err := repo.db.ExecContext(ctx, q, id)

	return err
}

func (repo *repo) ListPresets(ctx context.Context) ([]web.JobPreset, error) {
	const q = `SELECT id, name, data, created_at FROM job_presets ORDER BY created_at DESC`

	rows, err := repo.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var out []web.JobPreset

	for rows.Next() {
		var (
			id, name, data string
			createdAt       int64
		)

		if err := rows.Scan(&id, &name, &data, &createdAt); err != nil {
			return nil, err
		}

		var jd web.JobData

		if err := json.Unmarshal([]byte(data), &jd); err != nil {
			return nil, err
		}

		out = append(out, web.JobPreset{
			ID:        id,
			Name:      name,
			Data:      jd,
			CreatedAt: time.Unix(createdAt, 0).UTC(),
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func (repo *repo) GetPreset(ctx context.Context, id string) (web.JobPreset, error) {
	const q = `SELECT id, name, data, created_at FROM job_presets WHERE id = ?`

	var (
		pid, name, data string
		createdAt        int64
	)

	err := repo.db.QueryRowContext(ctx, q, id).Scan(&pid, &name, &data, &createdAt)
	if err != nil {
		return web.JobPreset{}, err
	}

	var jd web.JobData

	if err := json.Unmarshal([]byte(data), &jd); err != nil {
		return web.JobPreset{}, err
	}

	return web.JobPreset{
		ID:        pid,
		Name:      name,
		Data:      jd,
		CreatedAt: time.Unix(createdAt, 0).UTC(),
	}, nil
}
